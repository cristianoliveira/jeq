package typesafeapi_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cristianoliveira/jeq/internal/fixtures"
	"github.com/cristianoliveira/jeq/internal/infra/typesafeapi"
)

func TestCustomLoopbackProviderEvaluatesWithoutModelsPreflight(t *testing.T) {
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.Method+" "+r.URL.Path)
		if r.URL.Path != "/v1/systemone" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(fixtures.MustContract(t, "response_200_full.json"))
	}))
	defer srv.Close()

	client := typesafeapi.New(srv.URL, srv.Client(), "")
	client.AuthMode = "none"
	if _, err := client.Evaluate(context.Background(), sampleRequest(t)); err != nil {
		t.Fatalf("loopback evaluate failed: %v", err)
	}
	if len(paths) != 1 || paths[0] != "POST /v1/systemone" {
		t.Fatalf("requests=%v, want only judgment POST", paths)
	}
}
