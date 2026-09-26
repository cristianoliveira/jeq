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

	"github.com/cristianoliveira/jeq/internal/fixtures"
	"github.com/cristianoliveira/jeq/internal/infra/typesafeapi"
)

func TestLoopbackUnauthenticatedRequestOmitsAuthorization(t *testing.T) {
	auth := make(chan string, 1)
	responseBody := fixtures.MustContract(t, "response_200_full.json")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth <- r.Header.Get("Authorization")
		_, _ = w.Write(responseBody)
	}))
	defer srv.Close()
	c := typesafeapi.New(srv.URL, &http.Client{Timeout: time.Second}, "")
	c.AuthMode = "none"
	_, err := c.Evaluate(context.Background(), sampleRequest(t))
	require.Nil(t, err)
	assert.Empty(t, <-auth)
}

func TestRedirectResponseDoesNotFollow(t *testing.T) {
	var targetHits atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) { targetHits.Add(1) }))
	defer target.Close()
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL, http.StatusFound) }))
	defer source.Close()
	httpc := &http.Client{Timeout: time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	c := typesafeapi.New(source.URL, httpc, testKey)
	_, err := c.Evaluate(context.Background(), sampleRequest(t))
	require.NotNil(t, err)
	assert.Zero(t, targetHits.Load(), "redirect target must not be reached")
}
