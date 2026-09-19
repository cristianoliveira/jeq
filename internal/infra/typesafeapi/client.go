// Package typesafeapi is the net/http adapter for the TypeSafe System One
// endpoints: bearer auth, bounded body reads, bounded retry on documented
// throttling statuses, and status-to-error classification. It is the only
// package that touches the wire; construction happens in the composition root.
package typesafeapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/cristianoliveira/gev/internal/domain/contract"
	"github.com/cristianoliveira/gev/internal/domain/gev"
)

// Body bounds: normal replies may be large; error details are read only up
// to a small window since they never enter machine output verbatim.
const (
	DefaultMaxBodyBytes  int64 = 8 << 20  // 8 MiB
	DefaultMaxErrorBytes int64 = 64 << 10 // 64 KiB
	defaultHTTPTimeout         = 10 * time.Second
)

// Retry bounds (OQ-3): default 2 retries (3 attempts), hard max 5; backoff
// starts at 250ms and doubles; no single wait may exceed 10s.
const (
	DefaultMaxRetries = 2
	MaxRetriesLimit   = 5
	retryBaseBackoff  = 250 * time.Millisecond
	retryWaitCap      = 10 * time.Second
)

// Client calls the two TypeSafe endpoints. All fields are injected; there
// are no globals and the domain never constructs it.
type Client struct {
	BaseURL string // API root, e.g. https://api.typesafe.ai
	HTTP    *http.Client
	APIKey  string

	MaxBodyBytes  int64
	MaxErrorBytes int64

	// Retry policy. MaxRetries is clamped to 0..5; Sleep and Now are
	// injectable so tests record waits instead of sleeping; Diagnostic
	// receives one line per retry attempt (never stdout, never the key).
	MaxRetries int
	Sleep      func(time.Duration)
	Now        func() time.Time
	Diagnostic func(line string)
}

// New builds a client with the documented defaults applied.
func New(baseURL string, httpc *http.Client, apiKey string) *Client {
	if httpc == nil {
		httpc = &http.Client{Timeout: defaultHTTPTimeout}
	}
	return &Client{
		BaseURL:       strings.TrimSuffix(baseURL, "/"),
		HTTP:          httpc,
		APIKey:        apiKey,
		MaxBodyBytes:  DefaultMaxBodyBytes,
		MaxErrorBytes: DefaultMaxErrorBytes,
		MaxRetries:    DefaultMaxRetries,
		Sleep:         time.Sleep,
		Now:           time.Now,
	}
}

// Evaluate posts one System One request and decodes the response tolerantly.
func (c *Client) Evaluate(ctx context.Context, req contract.Request) (contract.Response, *gev.Error) {
	body, err := req.Encode()
	if err != nil {
		return contract.Response{}, withRecovery(gev.WrapError(gev.CodeRequestInvalid, err, "encoding request"),
			"report this as a gev bug; the document came from gev's own composer")
	}

	raw, cerr := c.call(ctx, http.MethodPost, "/v1/systemone", body)
	if cerr != nil {
		return contract.Response{}, cerr
	}

	resp, derr := contract.DecodeResponse(raw)
	if derr != nil {
		return contract.Response{}, withRecovery(derr, "retry the request; if it persists the server reply broke the contract")
	}
	return resp, nil
}

// Models fetches the account's model list.
func (c *Client) Models(ctx context.Context) (contract.Models, *gev.Error) {
	raw, cerr := c.call(ctx, http.MethodGet, "/v1/models", nil)
	if cerr != nil {
		return contract.Models{}, cerr
	}

	models, derr := contract.DecodeModels(raw)
	if derr != nil {
		return contract.Models{}, withRecovery(derr, "retry the request; if it persists the server reply broke the contract")
	}
	return models, nil
}

func (c *Client) call(ctx context.Context, method, path string, body []byte) ([]byte, *gev.Error) {
	if c.APIKey == "" {
		return nil, withRecovery(gev.NewError(gev.CodeAuthMissing, "TYPESAFE_API_KEY is not set"),
			"export TYPESAFE_API_KEY with the account key")
	}

	retries := c.MaxRetries
	if retries < 0 {
		retries = 0
	} else if retries > MaxRetriesLimit {
		retries = MaxRetriesLimit
	}

	for attempt := 0; ; attempt++ {
		status, header, raw, cerr := c.attemptOnce(ctx, method, path, body)
		if cerr != nil {
			return nil, cerr
		}
		if status == http.StatusOK {
			return raw, nil
		}

		// Only documented throttling statuses replay; exhaustion keeps the
		// same stable classification with its recovery instruction.
		if (status == http.StatusTooManyRequests || status == 529) && attempt < retries {
			wait, source := c.retryWait(header.Get("Retry-After"), attempt)
			c.noteRetry(attempt, retries, wait, status, source)
			sleep := c.Sleep
			if sleep != nil {
				sleep(wait)
			}
			if ctxErr := ctx.Err(); ctxErr != nil {
				return nil, gev.WrapError(gev.CodeInterrupted, ctxErr, "interrupted while waiting to retry")
			}
			continue
		}
		return nil, classifyStatus(status, raw, c.MaxErrorBytes, c.APIKey)
	}
}

// attemptOnce performs exactly one HTTP exchange with bounded reads and an
// always-closed body. Transport failures are never retried here: the caller
// returns them immediately because a sent request may already have executed.
func (c *Client) attemptOnce(ctx context.Context, method, path string, body []byte) (status int, header http.Header, raw []byte, cerr *gev.Error) {
	var reader io.Reader
	if body != nil {
		reader = strings.NewReader(string(body))
	}
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, reader)
	if err != nil {
		return 0, nil, nil, withRecovery(gev.WrapError(gev.CodeNetworkError, err, "building request"),
			"check TYPESAFE_BASE_URL; it must be a valid API root")
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return 0, nil, nil, c.transportError(err)
	}
	defer func() { _ = resp.Body.Close() }() // read-side body; nothing to act on at close

	limit := c.MaxBodyBytes
	raw, readErr := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if readErr != nil {
		return 0, nil, nil, c.transportError(readErr)
	}
	if int64(len(raw)) > limit {
		return 0, nil, nil, withRecovery(gev.NewError(gev.CodeResponseInvalid,
			fmt.Sprintf("response body exceeds the %d byte safety bound", limit)),
			"retry; if it persists, the server reply is too large for gev's safety bound")
	}
	return resp.StatusCode, resp.Header, raw, nil
}

// retryWait decides the next wait: Retry-After when it parses and fits the
// 10s cap, otherwise the doubling backoff schedule.
func (c *Client) retryWait(retryAfter string, attempt int) (time.Duration, string) {
	if d, ok := parseRetryAfter(retryAfter, c.Now(), retryWaitCap); ok {
		return d, "retry-after"
	}
	d := retryBaseBackoff << attempt
	if d > retryWaitCap {
		d = retryWaitCap
	}
	return d, "backoff"
}

func (c *Client) noteRetry(attempt, retries int, wait time.Duration, status int, source string) {
	if c.Diagnostic == nil {
		return
	}
	c.Diagnostic(fmt.Sprintf("retry %d/%d after %s (%s; status %d)", attempt+1, retries, wait, source, status))
}

// parseRetryAfter accepts delta-seconds and HTTP-dates. Absent, malformed,
// negative, or over-cap values are invalid and fall back to the schedule.
func parseRetryAfter(v string, now time.Time, maxWait time.Duration) (time.Duration, bool) {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0, false
	}
	if secs, err := strconv.Atoi(v); err == nil {
		if secs < 0 {
			return 0, false
		}
		d := time.Duration(secs) * time.Second
		return d, d <= maxWait
	}
	if t, err := http.ParseTime(v); err == nil {
		d := t.Sub(now)
		if d <= 0 {
			return 0, false
		}
		return d, d <= maxWait
	}
	return 0, false
}

// classifyStatus maps every non-200 status to its stable code. Server detail
// is sanitized: JSON message fields only, bounded, key material redacted.
func classifyStatus(status int, body []byte, maxError int64, apiKey string) *gev.Error {
	detail := sanitizeDetail(body, maxError, apiKey)

	switch status {
	case http.StatusUnauthorized:
		e := gev.NewError(gev.CodeAuthRejected, "the server rejected the credential")
		e.Message = appendDetail(e.Message, detail)
		return withRecovery(e, "check TYPESAFE_API_KEY; it is missing, revoked, or mistyped")
	case http.StatusUnprocessableEntity:
		e := gev.NewError(gev.CodeRequestRejected, "the server rejected request fields gev cannot check locally")
		e.Message = appendDetail(e.Message, detail)
		return withRecovery(e, "fix the field the server names and resubmit; gev validates only local rules")
	case http.StatusTooManyRequests, 529:
		e := gev.NewError(gev.CodeRateLimited, fmt.Sprintf("the server asked to slow down (status %d)", status))
		e.Message = appendDetail(e.Message, detail)
		return withRecovery(e, "retry after a delay; honor Retry-After when the server sends one")
	}

	if status >= 500 {
		e := gev.NewError(gev.CodeServerError, fmt.Sprintf("server failure (status %d)", status))
		e.Message = appendDetail(e.Message, detail)
		return withRecovery(e, "retry later; the failure is on the server side")
	}
	e := gev.NewError(gev.CodeResponseInvalid, fmt.Sprintf("unexpected status %d", status))
	e.Message = appendDetail(e.Message, detail)
	return withRecovery(e, "retry; if it persists the server behavior broke the contract")
}

func (c *Client) transportError(err error) *gev.Error {
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() ||
		errors.Is(err, context.DeadlineExceeded) || errors.Is(err, os.ErrDeadlineExceeded) {
		return withRecovery(gev.WrapError(gev.CodeTimeout, err, "the server did not answer in time"),
			"increase --timeout or check connectivity")
	}
	return withRecovery(gev.WrapError(gev.CodeNetworkError, err, "request failed before a usable reply"),
		"check network, DNS, TLS, and TYPESAFE_BASE_URL")
}

// sanitizeDetail extracts a bounded, key-redacted message from a JSON error
// body. Non-JSON bodies and unknown shapes are contained entirely.
func sanitizeDetail(body []byte, maxError int64, apiKey string) string {
	if int64(len(body)) > maxError {
		body = body[:maxError]
	}
	var m map[string]any
	if json.Unmarshal(body, &m) != nil {
		return ""
	}
	for _, key := range []string{"message", "detail", "error"} {
		if s, ok := m[key].(string); ok && s != "" {
			return redact(truncate(s, 200), apiKey)
		}
	}
	return ""
}

func appendDetail(message, detail string) string {
	if detail == "" {
		return message
	}
	return message + ": " + detail
}

func redact(s, secret string) string {
	if secret == "" {
		return s
	}
	return strings.ReplaceAll(s, secret, "[redacted]")
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func withRecovery(e *gev.Error, recovery string) *gev.Error {
	return e.WithRecovery(recovery)
}
