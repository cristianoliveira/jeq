package blackbox_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBlackBoxVerbosePartialMapFailurePreservesPriorOutput(t *testing.T) {
	response := fixture(t, "response_200_full.json")
	api := newFakeAPI(t, func(w http.ResponseWriter, r *http.Request, _ int) {
		if !strings.Contains(r.Header.Get("X-Test-Body"), "second") {
			_, _ = w.Write(response)
			return
		}
		http.Error(w, "private provider detail", 500)
	})
	input := `{"description":"first"}` + "\n" + `{"description":"second"}` + "\n"
	result := runBinary(t, input, map[string]string{"TYPESAFE_BASE_URL": api.server.URL, "TYPESAFE_API_KEY": "partial-secret", "JEQ_TRACE_ID": "partial"}, "--verbose", "map", "--as", "risk", "--input", "ndjson", "--state-pointer", "/description", "--questions-json", `{"questions":{"risk":{"type":"noul","instructions":"private prompt"}}}`)
	require.Equal(t, 1, result.exit)
	assert.Contains(t, result.stdout, `"first"`)
	assert.NotContains(t, result.stdout, "second")
	assert.Contains(t, result.stderr, `"operation_index":2`)
	assert.NotContains(t, result.stderr, "private")
	assert.NotContains(t, result.stderr, "partial-secret")
}

func TestBlackBoxFailureTraceNamesCommandPhase(t *testing.T) {
	cases := [][]string{{"reduce", "--as", "x", "--input", "ndjson", "--questions-json", `{"questions":{"x":{"type":"noul","instructions":"x"}}}`}, {"rank", "--as", "x", "--input", "json", "--state", "s", "--instruction", "x", "--id-pointer", "/id", "--criteria-pointer", "/criteria"}, {"gate", "--as", "x", "--input", "ndjson", "--value-pointer", "/missing", "--pass-min", "0.8", "--reject-max", "0.2"}}
	for _, args := range cases {
		result := runBinary(t, "not-json\n", map[string]string{"JEQ_TRACE_ID": "failure"}, append([]string{"--verbose"}, args...)...)
		require.NotEqual(t, 0, result.exit, "args=%v", args)
		assert.Contains(t, result.stderr, `"event":"run.failed"`, "args=%v", args)
		assert.True(t, strings.Contains(result.stderr, `"phase":"input_validation"`) || strings.Contains(result.stderr, `"phase":"offline_policy"`), "args=%v stderr=%s", args, result.stderr)
	}
}
