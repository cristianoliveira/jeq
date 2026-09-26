package typesafeapi_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cristianoliveira/jeq/internal/fixtures"
	"github.com/cristianoliveira/jeq/internal/infra/typesafeapi"
)

func TestCustomLoopbackProviderEvaluatesWithoutModelsPreflight(t *testing.T) {
	var requests atomic.Int32
	var path atomic.Value
	responseBody := fixtures.MustContract(t, "response_200_full.json")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		path.Store(r.Method + " " + r.URL.Path)
		if r.URL.Path != "/v1/systemone" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(responseBody)
	}))
	defer srv.Close()

	client := typesafeapi.New(srv.URL, srv.Client(), "")
	client.AuthMode = "none"
	_, err := client.Evaluate(context.Background(), sampleRequest(t))
	require.Nil(t, err)
	assert.Equal(t, int32(1), requests.Load(), "only the judgment endpoint should be called")
	assert.Equal(t, "POST /v1/systemone", path.Load())
}
