package blackbox_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCustomProviderRoutesAskWithoutModelsOrAuth(t *testing.T) {
	type requestObservation struct {
		path, auth string
	}
	observations := make(chan requestObservation, 16)
	response := fixture(t, "response_200_full.json")
	api := newFakeAPI(t, func(w http.ResponseWriter, r *http.Request, _ int) {
		observations <- requestObservation{path: r.Method + " " + r.URL.Path, auth: r.Header.Get("Authorization")}
		if r.URL.Path != "/v1/systemone" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(response)
	})
	result := runBinary(t, `{"model":"jev-loopback","state":{"message":"hello"},"questions":{"urgent":{"type":"noul","instructions":"Is this urgent?"}}}`+"\n", map[string]string{"JEQ_PROVIDER": "custom", "JEQ_BASE_URL": api.server.URL, "JEQ_AUTH": "none"}, "ask", "--request", "-")
	require.Equal(t, 0, result.exit)
	require.Equal(t, 1, api.count())
	got := <-observations
	assert.Equal(t, "POST /v1/systemone", got.path)
	assert.Empty(t, got.auth)
	assert.Contains(t, string(api.body()), `"model":"jev-loopback"`)
}

func TestCustomProviderDoesNotFallbackAfterLoopbackFailure(t *testing.T) {
	api := newFakeAPI(t, func(w http.ResponseWriter, _ *http.Request, _ int) {
		http.Error(w, "local failure", http.StatusBadGateway)
	})
	result := runBinary(t, `{"model":"jev-loopback","state":{"message":"hello"},"questions":{"urgent":{"type":"noul","instructions":"Is this urgent?"}}}`+"\n", map[string]string{"JEQ_PROVIDER": "custom", "JEQ_BASE_URL": api.server.URL, "JEQ_AUTH": "none"}, "ask", "--request", "-")
	require.NotEqual(t, 0, result.exit)
	assert.Greater(t, api.count(), 0, "custom provider should make a local request")
	assert.NotContains(t, result.stdout, "answers")
}
