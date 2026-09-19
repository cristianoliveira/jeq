package typesafeapi_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

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
	if err != nil {
		t.Fatal(err)
	}
	return req
}

func TestEvaluateHitsSystemOneWithBearerAndContentType(t *testing.T) {
	var sawAuth, sawContentType, sawPath, sawMethod bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawAuth = r.Header.Get("Authorization") == "Bearer "+testKey
		sawContentType = r.Header.Get("Content-Type") == "application/json"
		sawPath = r.URL.Path == "/v1/systemone"
		sawMethod = r.Method == http.MethodPost
		_, _ = w.Write(fixtures.MustContract(t, "response_200_full.json"))
	}))
	defer srv.Close()

	resp, err := newTestClient(t, srv.URL, nil).Evaluate(context.Background(), sampleRequest(t))
	if err != nil {
		t.Fatalf("evaluate failed: %v", err)
	}
	if !sawAuth || !sawContentType || !sawPath || !sawMethod {
		t.Errorf("auth=%v content-type=%v path=%v method=%v", sawAuth, sawContentType, sawPath, sawMethod)
	}
	if resp.Model != "jev-1.13.0" {
		t.Errorf("model = %q", resp.Model)
	}
}

func TestModelsHitsModelsWithBearer(t *testing.T) {
	var sawAuth, sawPath, sawMethod bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawAuth = r.Header.Get("Authorization") == "Bearer "+testKey
		sawPath = r.URL.Path == "/v1/models"
		sawMethod = http.MethodGet == r.Method
		_, _ = w.Write(fixtures.MustContract(t, "models.json"))
	}))
	defer srv.Close()

	models, err := newTestClient(t, srv.URL, nil).Models(context.Background())
	if err != nil {
		t.Fatalf("models failed: %v", err)
	}
	if !sawAuth || !sawPath || !sawMethod {
		t.Errorf("auth=%v path=%v method=%v", sawAuth, sawPath, sawMethod)
	}
	if len(models.Models) != 2 || models.Models[0].Name != "jev-latest" {
		t.Errorf("models = %v", models.Models)
	}
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
	if err == nil || err.Code != jeq.CodeAuthMissing {
		t.Fatalf("expected %q, got %v", jeq.CodeAuthMissing, err)
	}
	if hits.Load() != 0 {
		t.Errorf("missing key made %d requests; must fail pre-network", hits.Load())
	}
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
			if err == nil || err.Code != tt.wantCode {
				t.Fatalf("expected %q, got %v", tt.wantCode, err)
			}
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
	if err == nil || err.Code != jeq.CodeRequestRejected {
		t.Fatalf("expected %q, got %v", jeq.CodeRequestRejected, err)
	}
	if !strings.Contains(err.Message, "questions.department.criteria") {
		t.Errorf("message %q does not surface the server's field detail", err.Message)
	}
	if err.Recovery == "" {
		t.Error("422 must carry an actionable recovery instruction")
	}
}

func TestOversizeReplyRejected(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(make([]byte, 128))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, func(c *typesafeapi.Client) { c.MaxBodyBytes = 64 })
	_, err := c.Evaluate(context.Background(), sampleRequest(t))
	if err == nil || err.Code != jeq.CodeResponseInvalid {
		t.Fatalf("expected %q for oversize reply, got %v", jeq.CodeResponseInvalid, err)
	}
}

func TestErrorBodyReadBounded(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write(make([]byte, 1<<20)) // 1 MiB of error body
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, func(c *typesafeapi.Client) { c.MaxErrorBytes = 1024 })
	_, err := c.Evaluate(context.Background(), sampleRequest(t))
	if err == nil || err.Code != jeq.CodeServerError {
		t.Fatalf("expected %q, got %v", jeq.CodeServerError, err)
	}
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
	if err == nil || err.Code != jeq.CodeTimeout {
		t.Fatalf("expected %q, got %v", jeq.CodeTimeout, err)
	}
}

func TestConnectionFailureClassifiesAsNetworkError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))
	srv.Close() // dead endpoint

	_, err := newTestClient(t, srv.URL, nil).Evaluate(context.Background(), sampleRequest(t))
	if err == nil || err.Code != jeq.CodeNetworkError {
		t.Fatalf("expected %q, got %v", jeq.CodeNetworkError, err)
	}
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
	if err == nil {
		t.Fatal("expected an error")
	}
	if strings.Contains(err.Message, testKey) || strings.Contains(err.Recovery, testKey) {
		t.Errorf("secret leaked: %q / %q", err.Message, err.Recovery)
	}

	// Non-JSON error body: contained entirely.
	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = fmt.Fprintf(w, "raw %s boom", r.Header.Get("Authorization"))
	}))
	defer srv2.Close()

	_, err = newTestClient(t, srv2.URL, nil).Evaluate(context.Background(), sampleRequest(t))
	if err == nil {
		t.Fatal("expected an error")
	}
	if strings.Contains(err.Message, testKey) {
		t.Errorf("raw body leaked the secret: %q", err.Message)
	}
}

func TestResponseBodyAlwaysClosed(t *testing.T) {
	var open, closed atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(fixtures.MustContract(t, "response_200_full.json"))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, func(c *typesafeapi.Client) {
		base := c.HTTP.Transport
		if base == nil {
			base = http.DefaultTransport
		}
		c.HTTP.Transport = countedTransport{base: base, open: &open, closed: &closed}
	})
	if _, err := c.Evaluate(context.Background(), sampleRequest(t)); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Models(context.Background()); err != nil {
		t.Fatal(err)
	}
	if open.Load() != closed.Load() {
		t.Errorf("open=%d closed=%d; response bodies must always be closed", open.Load(), closed.Load())
	}
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
