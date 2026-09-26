package typesafeapi_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cristianoliveira/jeq/internal/domain/contract"
	"github.com/cristianoliveira/jeq/internal/domain/jeq"
	"github.com/cristianoliveira/jeq/internal/fixtures"
	"github.com/cristianoliveira/jeq/internal/infra/typesafeapi"
)

const testKey = "sekret-test-key-123"

func newTestClient(t *testing.T, baseURL string, mutate func(*typesafeapi.Client)) *typesafeapi.Client {
	t.Helper()
	c := typesafeapi.New(baseURL, &http.Client{Timeout: 5 * time.Second}, testKey)
	if mutate != nil {
		mutate(c)
	}
	return c
}

func sampleRequest(t *testing.T) contract.Request {
	t.Helper()
	req, err := contract.DecodeRequest(fixtures.MustContract(t, "request_full.json"))
	require.Nil(t, err)
	return req
}

func TestEvaluateHitsSystemOneWithBearerAndContentType(t *testing.T) {
	type requestObservation struct {
		auth, contentType, path, method string
	}
	observations := make(chan requestObservation, 1)
	responseBody := fixtures.MustContract(t, "response_200_full.json")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		observations <- requestObservation{
			auth:        r.Header.Get("Authorization"),
			contentType: r.Header.Get("Content-Type"),
			path:        r.URL.Path,
			method:      r.Method,
		}
		_, _ = w.Write(responseBody)
	}))
	defer srv.Close()

	resp, err := newTestClient(t, srv.URL, nil).Evaluate(context.Background(), sampleRequest(t))
	require.Nil(t, err)
	got := <-observations
	assert.Equal(t, "Bearer "+testKey, got.auth)
	assert.Equal(t, "application/json", got.contentType)
	assert.Equal(t, "/v1/systemone", got.path)
	assert.Equal(t, http.MethodPost, got.method)
	assert.Equal(t, "jev-1.13.0", resp.Model)
}

func TestModelsHitsModelsWithBearer(t *testing.T) {
	type requestObservation struct {
		auth, path, method string
	}
	observations := make(chan requestObservation, 1)
	modelsBody := fixtures.MustContract(t, "models.json")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		observations <- requestObservation{
			auth:   r.Header.Get("Authorization"),
			path:   r.URL.Path,
			method: r.Method,
		}
		_, _ = w.Write(modelsBody)
	}))
	defer srv.Close()

	models, err := newTestClient(t, srv.URL, nil).Models(context.Background())
	require.Nil(t, err)
	got := <-observations
	assert.Equal(t, "Bearer "+testKey, got.auth)
	assert.Equal(t, "/v1/models", got.path)
	assert.Equal(t, http.MethodGet, got.method)
	require.Len(t, models.Models, 2)
	assert.Equal(t, "jev-latest", models.Models[0].Name)
}

func TestMissingKeyFailsPreNetwork(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, nil)
	c.APIKey = ""

	_, err := c.Evaluate(context.Background(), sampleRequest(t))
	require.NotNil(t, err)
	assert.Equal(t, jeq.CodeAuthMissing, err.Code)
	assert.Zero(t, hits.Load(), "missing key must fail before making a request")
}

func TestStatusClassification(t *testing.T) {
	tests := []struct {
		name     string
		status   int
		body     string
		wantCode jeq.Code
	}{
		{"401 is auth rejected", http.StatusUnauthorized, `{"message":"bad key"}`, jeq.CodeAuthRejected},
		{"422 is request rejected", http.StatusUnprocessableEntity, `{"message":"criteria unknown"}`, jeq.CodeRequestRejected},
		{"429 is rate limited", http.StatusTooManyRequests, `{"message":"slow down"}`, jeq.CodeRateLimited},
		{"529 is rate limited", 529, `{"message":"overloaded"}`, jeq.CodeRateLimited},
		{"500 is server error", http.StatusInternalServerError, `oops`, jeq.CodeServerError},
		{"503 is server error", http.StatusServiceUnavailable, ``, jeq.CodeServerError},
		{"400 is a protocol breach", http.StatusBadRequest, `{"message":"nope"}`, jeq.CodeResponseInvalid},
		{"200 with malformed body is response invalid", http.StatusOK, `{not json`, jeq.CodeResponseInvalid},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer srv.Close()

			_, err := newTestClient(t, srv.URL, nil).Evaluate(context.Background(), sampleRequest(t))
			require.NotNil(t, err)
			assert.Equal(t, tt.wantCode, err.Code)
		})
	}
}

func Test422SurfacesSanitizedServerDetail(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"message":"questions.department.criteria: unknown option"}`))
	}))
	defer srv.Close()

	_, err := newTestClient(t, srv.URL, nil).Evaluate(context.Background(), sampleRequest(t))
	require.NotNil(t, err)
	assert.Equal(t, jeq.CodeRequestRejected, err.Code)
	assert.Contains(t, err.Message, "questions.department.criteria")
	assert.NotEmpty(t, err.Recovery, "422 must carry an actionable recovery instruction")
}

func TestOversizeReplyRejected(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(make([]byte, 128))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, func(c *typesafeapi.Client) { c.MaxBodyBytes = 64 })
	_, err := c.Evaluate(context.Background(), sampleRequest(t))
	require.NotNil(t, err)
	assert.Equal(t, jeq.CodeResponseInvalid, err.Code)
}

func TestErrorBodyReadBounded(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write(make([]byte, 1<<20)) // 1 MiB of error body
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, func(c *typesafeapi.Client) { c.MaxErrorBytes = 1024 })
	_, err := c.Evaluate(context.Background(), sampleRequest(t))
	require.NotNil(t, err)
	assert.Equal(t, jeq.CodeServerError, err.Code)
}

func TestTimeoutClassifiesAsTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		time.Sleep(200 * time.Millisecond)
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, func(c *typesafeapi.Client) {
		c.HTTP = &http.Client{Timeout: 20 * time.Millisecond}
	})
	_, err := c.Evaluate(context.Background(), sampleRequest(t))
	require.NotNil(t, err)
	assert.Equal(t, jeq.CodeTimeout, err.Code)
}

func TestConnectionFailureClassifiesAsNetworkError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))
	srv.Close() // dead endpoint

	_, err := newTestClient(t, srv.URL, nil).Evaluate(context.Background(), sampleRequest(t))
	require.NotNil(t, err)
	assert.Equal(t, jeq.CodeNetworkError, err.Code)
}

func TestSecretNeverLeaksIntoErrors(t *testing.T) {
	// The server echoes the bearer header back in both a JSON message and a
	// raw non-JSON body; neither may carry the key into the error values.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = fmt.Fprintf(w, `{"message":"rejected %s"}`, r.Header.Get("Authorization"))
	}))
	defer srv.Close()

	_, err := newTestClient(t, srv.URL, nil).Evaluate(context.Background(), sampleRequest(t))
	require.NotNil(t, err)
	assert.NotContains(t, err.Message, testKey)
	assert.NotContains(t, err.Recovery, testKey)

	// Non-JSON error body: contained entirely.
	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = fmt.Fprintf(w, "raw %s boom", r.Header.Get("Authorization"))
	}))
	defer srv2.Close()

	_, err = newTestClient(t, srv2.URL, nil).Evaluate(context.Background(), sampleRequest(t))
	require.NotNil(t, err)
	assert.NotContains(t, err.Message, testKey)
}

func TestResponseBodyAlwaysClosed(t *testing.T) {
	var open, closed atomic.Int32
	responseBody := fixtures.MustContract(t, "response_200_full.json")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(responseBody)
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, func(c *typesafeapi.Client) {
		base := c.HTTP.Transport
		if base == nil {
			base = http.DefaultTransport
		}
		c.HTTP.Transport = countedTransport{base: base, open: &open, closed: &closed}
	})
	_, err := c.Evaluate(context.Background(), sampleRequest(t))
	require.Nil(t, err)
	_, err = c.Models(context.Background())
	require.Nil(t, err)
	assert.Equal(t, open.Load(), closed.Load(), "response bodies must always be closed")
}

type countedTransport struct {
	base         http.RoundTripper
	open, closed *atomic.Int32
}

func (c countedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := c.base.RoundTrip(req)
	if err == nil && resp.Body != nil {
		c.open.Add(1)
		resp.Body = closedBody{resp.Body, c.closed}
	}
	return resp, err
}

type closedBody struct {
	io.ReadCloser
	closed *atomic.Int32
}

func (c closedBody) Close() error {
	c.closed.Add(1)
	return c.ReadCloser.Close()
}
