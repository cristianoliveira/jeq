package typesafeapi_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cristianoliveira/jeq/internal/domain/jeq"
	"github.com/cristianoliveira/jeq/internal/fixtures"
	"github.com/cristianoliveira/jeq/internal/infra/typesafeapi"
)

// recordingSleeper records every wait; tests never really sleep.
type recordingSleeper struct {
	waits []time.Duration
}

func (s *recordingSleeper) Sleep(d time.Duration) {
	s.waits = append(s.waits, d)
}

func okOnceThenFull(w http.ResponseWriter, callN int32, retryStatus int, retryAfter string, successBody []byte) {
	if callN == 1 {
		if retryAfter != "" {
			w.Header().Set("Retry-After", retryAfter)
		}
		w.WriteHeader(retryStatus)
		return
	}
	_, _ = w.Write(successBody)
}

func newScriptClient(t *testing.T, srvURL string, maxRetries int, sleeper *recordingSleeper) *typesafeapi.Client {
	t.Helper()
	c := typesafeapi.New(srvURL, &http.Client{Timeout: 5 * time.Second}, testKey)
	c.MaxRetries = maxRetries
	c.Sleep = sleeper.Sleep
	return c
}

func TestRetryOn429ThenSucceed(t *testing.T) {
	var calls atomic.Int32
	successBody := fixtures.MustContract(t, "response_200_full.json")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		okOnceThenFull(w, calls.Add(1), http.StatusTooManyRequests, "", successBody)
	}))
	defer srv.Close()

	sleeper := &recordingSleeper{}
	_, err := newScriptClient(t, srv.URL, 2, sleeper).Evaluate(context.Background(), sampleRequest(t))
	require.Nil(t, err)
	assert.Equal(t, int32(2), calls.Load(), "one retry should produce two requests")
	assert.Equal(t, []time.Duration{250 * time.Millisecond}, sleeper.waits)
}

func Test429ExhaustsBound(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	sleeper := &recordingSleeper{}
	_, err := newScriptClient(t, srv.URL, 2, sleeper).Evaluate(context.Background(), sampleRequest(t))
	require.NotNil(t, err)
	assert.Equal(t, jeq.CodeRateLimited, err.Code)
	assert.Equal(t, int32(3), calls.Load(), "default two retries should produce three requests")
	assert.Equal(t, []time.Duration{250 * time.Millisecond, 500 * time.Millisecond}, sleeper.waits)
	assert.NotEmpty(t, err.Recovery, "exhaustion must keep a recovery instruction")
}

func Test529RetriesLike429(t *testing.T) {
	var calls atomic.Int32
	successBody := fixtures.MustContract(t, "response_200_full.json")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) == 1 {
			w.WriteHeader(529)
			return
		}
		_, _ = w.Write(successBody)
	}))
	defer srv.Close()

	sleeper := &recordingSleeper{}
	_, err := newScriptClient(t, srv.URL, 2, sleeper).Evaluate(context.Background(), sampleRequest(t))
	require.Nil(t, err)
	assert.Equal(t, int32(2), calls.Load())
	assert.Equal(t, []time.Duration{250 * time.Millisecond}, sleeper.waits)
}

func TestZeroRetriesMeansSingleAttempt(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	sleeper := &recordingSleeper{}
	_, err := newScriptClient(t, srv.URL, 0, sleeper).Evaluate(context.Background(), sampleRequest(t))
	require.NotNil(t, err)
	assert.Equal(t, jeq.CodeRateLimited, err.Code)
	assert.Equal(t, int32(1), calls.Load())
	assert.Empty(t, sleeper.waits)
}

func TestMaxRetriesClampedToFive(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	sleeper := &recordingSleeper{}
	_, _ = newScriptClient(t, srv.URL, 99, sleeper).Evaluate(context.Background(), sampleRequest(t))
	assert.Equal(t, int32(6), calls.Load(), "five retries should produce six requests")
	want := []time.Duration{250 * time.Millisecond, 500 * time.Millisecond, time.Second, 2 * time.Second, 4 * time.Second}
	assert.Equal(t, want, sleeper.waits)
}

func TestRetryAfterSecondsHonored(t *testing.T) {
	var calls atomic.Int32
	successBody := fixtures.MustContract(t, "response_200_full.json")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) == 1 {
			w.Header().Set("Retry-After", "3")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_, _ = w.Write(successBody)
	}))
	defer srv.Close()

	sleeper := &recordingSleeper{}
	_, err := newScriptClient(t, srv.URL, 2, sleeper).Evaluate(context.Background(), sampleRequest(t))
	require.Nil(t, err)
	assert.Equal(t, []time.Duration{3 * time.Second}, sleeper.waits)
}

func TestRetryAfterHTTPDateHonored(t *testing.T) {
	now := time.Date(2026, 9, 19, 12, 0, 0, 0, time.UTC)
	var calls atomic.Int32
	successBody := fixtures.MustContract(t, "response_200_full.json")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) == 1 {
			w.Header().Set("Retry-After", now.Add(4*time.Second).UTC().Format(http.TimeFormat))
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_, _ = w.Write(successBody)
	}))
	defer srv.Close()

	sleeper := &recordingSleeper{}
	c := newScriptClient(t, srv.URL, 2, sleeper)
	c.Now = func() time.Time { return now }
	_, err := c.Evaluate(context.Background(), sampleRequest(t))
	require.Nil(t, err)
	assert.Equal(t, []time.Duration{4 * time.Second}, sleeper.waits)
}

func TestRetryAfterFallbacks(t *testing.T) {
	tests := []struct {
		name       string
		retryAfter string
	}{
		{"malformed value falls back", "soon"},
		{"seconds above cap fall back", "60"},
		{"negative falls back", "-5"},
		{"past date falls back", "Mon, 02 Jan 2006 15:04:05 GMT"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var calls atomic.Int32
			successBody := fixtures.MustContract(t, "response_200_full.json")
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if calls.Add(1) == 1 {
					w.Header().Set("Retry-After", tt.retryAfter)
					w.WriteHeader(http.StatusTooManyRequests)
					return
				}
				_, _ = w.Write(successBody)
			}))
			defer srv.Close()

			sleeper := &recordingSleeper{}
			_, err := newScriptClient(t, srv.URL, 2, sleeper).Evaluate(context.Background(), sampleRequest(t))
			require.Nil(t, err)
			assert.Equal(t, []time.Duration{250 * time.Millisecond}, sleeper.waits)
		})
	}
}

func TestNeverRetryOtherStatusesOrTransport(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusUnprocessableEntity, http.StatusInternalServerError} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			var calls atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				w.WriteHeader(status)
			}))
			defer srv.Close()

			sleeper := &recordingSleeper{}
			_, _ = newScriptClient(t, srv.URL, 2, sleeper).Evaluate(context.Background(), sampleRequest(t))
			assert.Equal(t, int32(1), calls.Load(), "status %d must not be retried", status)
			assert.Empty(t, sleeper.waits)
		})
	}

	t.Run("transport drop", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))
		srv.Close()
		sleeper := &recordingSleeper{}
		_, err := newScriptClient(t, srv.URL, 2, sleeper).Evaluate(context.Background(), sampleRequest(t))
		require.NotNil(t, err)
		assert.Equal(t, jeq.CodeNetworkError, err.Code)
		assert.Empty(t, sleeper.waits, "ambiguous transport failures must never be replayed")
	})
}

func TestRetryDiagnosticsGoToHook(t *testing.T) {
	var calls atomic.Int32
	successBody := fixtures.MustContract(t, "response_200_full.json")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_, _ = w.Write(successBody)
	}))
	defer srv.Close()

	sleeper := &recordingSleeper{}
	var lines []string
	c := newScriptClient(t, srv.URL, 2, sleeper)
	c.Diagnostic = func(line string) { lines = append(lines, line) }
	_, err := c.Evaluate(context.Background(), sampleRequest(t))
	require.Nil(t, err)

	require.Len(t, lines, 1)
	assert.Contains(t, lines[0], "250ms")
	assert.Contains(t, lines[0], "429")
}
