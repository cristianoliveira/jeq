package typesafeapi_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

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

func okOnceThenFull(t *testing.T, w http.ResponseWriter, callN int32, retryStatus int, retryAfter string) {
	t.Helper()
	if callN == 1 {
		if retryAfter != "" {
			w.Header().Set("Retry-After", retryAfter)
		}
		w.WriteHeader(retryStatus)
		return
	}
	_, _ = w.Write(fixtures.MustContract(t, "response_200_full.json"))
}

func newScriptClient(t *testing.T, srvURL string, maxRetries int, sleeper *recordingSleeper) *typesafeapi.Client {
	t.Helper()
	c := typesafeapi.New(srvURL, &http.Client{Timeout: 5 * time.Second}, testKey)
	c.MaxRetries = maxRetries
	c.Sleep = sleeper.Sleep
	return c
}

func equalDurations(got, want []time.Duration) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func TestRetryOn429ThenSucceed(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		okOnceThenFull(t, w, calls.Add(1), http.StatusTooManyRequests, "")
	}))
	defer srv.Close()

	sleeper := &recordingSleeper{}
	if _, err := newScriptClient(t, srv.URL, 2, sleeper).Evaluate(context.Background(), sampleRequest(t)); err != nil {
		t.Fatalf("evaluate failed: %v", err)
	}
	if got := calls.Load(); got != 2 {
		t.Errorf("requests = %d, want 2 (one retry)", got)
	}
	if len(sleeper.waits) != 1 || sleeper.waits[0] != 250*time.Millisecond {
		t.Errorf("waits = %v, want [250ms]", sleeper.waits)
	}
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
	if err == nil || err.Code != jeq.CodeRateLimited {
		t.Fatalf("expected %q, got %v", jeq.CodeRateLimited, err)
	}
	if got := calls.Load(); got != 3 {
		t.Errorf("requests = %d, want 3 (default 2 retries)", got)
	}
	want := []time.Duration{250 * time.Millisecond, 500 * time.Millisecond}
	if !equalDurations(sleeper.waits, want) {
		t.Errorf("waits = %v, want %v", sleeper.waits, want)
	}
	if err.Recovery == "" {
		t.Error("exhaustion must keep a recovery instruction")
	}
}

func Test529RetriesLike429(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) == 1 {
			w.WriteHeader(529)
			return
		}
		_, _ = w.Write(fixtures.MustContract(t, "response_200_full.json"))
	}))
	defer srv.Close()

	sleeper := &recordingSleeper{}
	if _, err := newScriptClient(t, srv.URL, 2, sleeper).Evaluate(context.Background(), sampleRequest(t)); err != nil {
		t.Fatalf("evaluate failed: %v", err)
	}
	if got := calls.Load(); got != 2 {
		t.Errorf("requests = %d, want 2", got)
	}
	if len(sleeper.waits) != 1 || sleeper.waits[0] != 250*time.Millisecond {
		t.Errorf("waits = %v, want [250ms]", sleeper.waits)
	}
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
	if err == nil || err.Code != jeq.CodeRateLimited {
		t.Fatalf("expected %q, got %v", jeq.CodeRateLimited, err)
	}
	if got := calls.Load(); got != 1 {
		t.Errorf("requests = %d, want 1", got)
	}
	if len(sleeper.waits) != 0 {
		t.Errorf("waits = %v, want none", sleeper.waits)
	}
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
	if got := calls.Load(); got != 6 {
		t.Errorf("requests = %d, want 6 (5 retries max)", got)
	}
	want := []time.Duration{250 * time.Millisecond, 500 * time.Millisecond, time.Second, 2 * time.Second, 4 * time.Second}
	if !equalDurations(sleeper.waits, want) {
		t.Errorf("waits = %v, want %v", sleeper.waits, want)
	}
}

func TestRetryAfterSecondsHonored(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) == 1 {
			w.Header().Set("Retry-After", "3")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_, _ = w.Write(fixtures.MustContract(t, "response_200_full.json"))
	}))
	defer srv.Close()

	sleeper := &recordingSleeper{}
	if _, err := newScriptClient(t, srv.URL, 2, sleeper).Evaluate(context.Background(), sampleRequest(t)); err != nil {
		t.Fatalf("evaluate failed: %v", err)
	}
	if len(sleeper.waits) != 1 || sleeper.waits[0] != 3*time.Second {
		t.Errorf("waits = %v, want [3s] from Retry-After", sleeper.waits)
	}
}

func TestRetryAfterHTTPDateHonored(t *testing.T) {
	now := time.Date(2026, 9, 19, 12, 0, 0, 0, time.UTC)
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) == 1 {
			w.Header().Set("Retry-After", now.Add(4*time.Second).UTC().Format(http.TimeFormat))
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_, _ = w.Write(fixtures.MustContract(t, "response_200_full.json"))
	}))
	defer srv.Close()

	sleeper := &recordingSleeper{}
	c := newScriptClient(t, srv.URL, 2, sleeper)
	c.Now = func() time.Time { return now }
	if _, err := c.Evaluate(context.Background(), sampleRequest(t)); err != nil {
		t.Fatalf("evaluate failed: %v", err)
	}
	if len(sleeper.waits) != 1 || sleeper.waits[0] != 4*time.Second {
		t.Errorf("waits = %v, want [4s] from Retry-After date", sleeper.waits)
	}
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
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if calls.Add(1) == 1 {
					w.Header().Set("Retry-After", tt.retryAfter)
					w.WriteHeader(http.StatusTooManyRequests)
					return
				}
				_, _ = w.Write(fixtures.MustContract(t, "response_200_full.json"))
			}))
			defer srv.Close()

			sleeper := &recordingSleeper{}
			if _, err := newScriptClient(t, srv.URL, 2, sleeper).Evaluate(context.Background(), sampleRequest(t)); err != nil {
				t.Fatalf("evaluate failed: %v", err)
			}
			if len(sleeper.waits) != 1 || sleeper.waits[0] != 250*time.Millisecond {
				t.Errorf("waits = %v, want [250ms] from the backoff schedule", sleeper.waits)
			}
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
			if got := calls.Load(); got != 1 {
				t.Errorf("requests = %d, want exactly 1 (status %d is never retried)", got, status)
			}
			if len(sleeper.waits) != 0 {
				t.Errorf("waits = %v, want none", sleeper.waits)
			}
		})
	}

	t.Run("transport drop", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))
		srv.Close()
		sleeper := &recordingSleeper{}
		_, err := newScriptClient(t, srv.URL, 2, sleeper).Evaluate(context.Background(), sampleRequest(t))
		if err == nil || err.Code != jeq.CodeNetworkError {
			t.Fatalf("expected %q, got %v", jeq.CodeNetworkError, err)
		}
		if len(sleeper.waits) != 0 {
			t.Errorf("waits = %v; ambiguous transport failures must never be replayed", sleeper.waits)
		}
	})
}

func TestRetryDiagnosticsGoToHook(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_, _ = w.Write(fixtures.MustContract(t, "response_200_full.json"))
	}))
	defer srv.Close()

	sleeper := &recordingSleeper{}
	var lines []string
	c := newScriptClient(t, srv.URL, 2, sleeper)
	c.Diagnostic = func(line string) { lines = append(lines, line) }
	if _, err := c.Evaluate(context.Background(), sampleRequest(t)); err != nil {
		t.Fatal(err)
	}

	if len(lines) != 1 {
		t.Fatalf("diagnostic lines = %v, want exactly one", lines)
	}
	if !strings.Contains(lines[0], "250ms") || !strings.Contains(lines[0], "429") {
		t.Errorf("diagnostic line %q lacks attempt detail", lines[0])
	}
}
