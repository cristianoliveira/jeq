package typesafeapi_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cristianoliveira/jeq/internal/fixtures"
	"github.com/cristianoliveira/jeq/internal/infra/typesafeapi"
)

func TestLoopbackUnauthenticatedRequestOmitsAuthorization(t *testing.T) {
	var auth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		_, _ = w.Write(fixtures.MustContract(t, "response_200_full.json"))
	}))
	defer srv.Close()
	c := typesafeapi.New(srv.URL, &http.Client{Timeout: time.Second}, "")
	c.AuthMode = "none"
	if _, err := c.Evaluate(context.Background(), sampleRequest(t)); err != nil {
		t.Fatal(err)
	}
	if auth != "" {
		t.Fatalf("authorization leaked: %q", auth)
	}
}

func TestRedirectResponseDoesNotFollow(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) { t.Error("redirect target was reached") }))
	defer target.Close()
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL, http.StatusFound) }))
	defer source.Close()
	httpc := &http.Client{Timeout: time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	c := typesafeapi.New(source.URL, httpc, testKey)
	if _, err := c.Evaluate(context.Background(), sampleRequest(t)); err == nil {
		t.Fatal("redirect unexpectedly accepted")
	}
}
