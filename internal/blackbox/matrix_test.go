package blackbox_test

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBlackBoxVerboseContractMatrixKeepsStdoutExitAndRequests(t *testing.T) {
	cases := []string{"ask", "validate", "map", "rate", "reduce", "rank", "gate"}
	for _, command := range cases {
		t.Run(command, func(t *testing.T) {
			plain := runBinary(t, "", nil, command, "--help")
			verbose := runBinary(t, "", map[string]string{"JEQ_TRACE_ID": "matrix"}, "--verbose", command, "--help")
			if plain.exit != verbose.exit || plain.stdout != verbose.stdout {
				t.Fatalf("default contract changed: plain=%#v verbose=%#v", plain, verbose)
			}
			if strings.Contains(verbose.stdout, "jeq.trace.v1") {
				t.Fatal("trace entered stdout")
			}
		})
	}
}

func TestBlackBoxVerboseFailureMatrix(t *testing.T) {
	path := filepath.Join(t.TempDir(), "request.json")
	if err := os.WriteFile(path, fixture(t, "request_full.json"), 0o600); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name    string
		handler func(http.ResponseWriter, *http.Request, int)
		args    []string
	}{{"retry-exhaustion", func(w http.ResponseWriter, _ *http.Request, _ int) {
		w.Header().Set("Retry-After", "0")
		w.WriteHeader(429)
	}, []string{"--max-retries", "0"}}, {"malformed", func(w http.ResponseWriter, _ *http.Request, _ int) { _, _ = w.Write([]byte("provider-private-body")) }, nil}, {"timeout", func(_ http.ResponseWriter, r *http.Request, _ int) { <-r.Context().Done() }, []string{"--timeout", "10ms"}}}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			api := newFakeAPI(t, tc.handler)
			args := []string{"--verbose", "ask", "--request", path}
			args = append(args, tc.args...)
			result := runBinary(t, "", map[string]string{"TYPESAFE_BASE_URL": api.server.URL, "TYPESAFE_API_KEY": "matrix-secret", "JEQ_TRACE_ID": "failure-matrix"}, args...)
			if result.exit == 0 || !strings.Contains(result.stderr, `"event":"run.failed"`) || strings.Contains(result.stderr, "matrix-secret") || strings.Contains(result.stderr, "provider-private-body") {
				t.Fatalf("result=%#v", result)
			}
		})
	}
}

func TestBlackBoxVerbosePrivacyAdversarialInputs(t *testing.T) {
	api := newFakeAPI(t, func(w http.ResponseWriter, _ *http.Request, _ int) { _, _ = w.Write([]byte("provider-private-body")) })
	request := `{"state":"private-state","questions":{"q":{"type":"noul","instructions":"private-instruction"}},"extra":"candidate-id"}`
	result := runBinary(t, request, map[string]string{"JEQ_TRACE_ID": "safe-chain", "TYPESAFE_API_KEY": "credential-secret", "TYPESAFE_BASE_URL": api.server.URL + "/?token=secret#fragment"}, "--verbose", "ask", "--request", "-")
	for _, secret := range []string{"credential-secret", "user:pass", "token=secret", "fragment", "private-state", "private-instruction", "candidate-id", "provider-private-body"} {
		if strings.Contains(result.stderr, secret) {
			t.Fatalf("trace leaked %q: %s", secret, result.stderr)
		}
	}
}
