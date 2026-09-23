package blackbox_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/cristianoliveira/jeq/internal/fixtures"
)

func TestCustomProviderRoutesAskWithoutModelsOrAuth(t *testing.T) {
	var paths []string
	var auth string
	var body string
	api := newFakeAPI(t, func(w http.ResponseWriter, r *http.Request, _ int) {
		paths = append(paths, r.Method+" "+r.URL.Path)
		auth = r.Header.Get("Authorization")

		if r.URL.Path != "/v1/systemone" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(fixtures.MustContract(t, "response_200_full.json"))
	})
	result := runBinary(t, `{"model":"jev-loopback","state":{"message":"hello"},"questions":{"urgent":{"type":"noul","instructions":"Is this urgent?"}}}`+"\n", map[string]string{"JEQ_PROVIDER": "custom", "JEQ_BASE_URL": api.server.URL, "JEQ_AUTH": "none"}, "ask", "--request", "-")
	if result.exit != 0 {
		t.Fatalf("exit=%d stderr=%q", result.exit, result.stderr)
	}
	body = string(api.body())
	if strings.Join(paths, ",") != "POST /v1/systemone" || auth != "" || !strings.Contains(body, `"model":"jev-loopback"`) {
		t.Fatalf("paths=%v auth=%q", paths, auth)
	}
}

func TestCustomProviderDoesNotFallbackAfterLoopbackFailure(t *testing.T) {
	var paths []string
	api := newFakeAPI(t, func(w http.ResponseWriter, r *http.Request, _ int) {
		paths = append(paths, r.Method+" "+r.URL.Path)
		http.Error(w, "local failure", http.StatusBadGateway)
	})
	result := runBinary(t, `{"model":"jev-loopback","state":{"message":"hello"},"questions":{"urgent":{"type":"noul","instructions":"Is this urgent?"}}}`+"\n", map[string]string{"JEQ_PROVIDER": "custom", "JEQ_BASE_URL": api.server.URL, "JEQ_AUTH": "none"}, "ask", "--request", "-")
	if result.exit == 0 || len(paths) == 0 || strings.Contains(result.stdout, "answers") {
		t.Fatalf("fallback or success after local failure: exit=%d paths=%v stdout=%q", result.exit, paths, result.stdout)
	}
}
