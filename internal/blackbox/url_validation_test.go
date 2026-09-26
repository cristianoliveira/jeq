package blackbox_test

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBlackBoxInvalidBaseURLPrecedesAuthAcrossCommands(t *testing.T) {
	api := newFakeAPI(t, func(w http.ResponseWriter, _ *http.Request, _ int) { w.WriteHeader(500) })
	tmp := t.TempDir()
	request := filepath.Join(tmp, "request.json")
	require.NoError(t, os.WriteFile(request, []byte(`{"model":"m","state":"s","questions":{"q":{"type":"noul","instructions":"Is this?"}}}`), 0o600))
	cases := [][]string{
		{"ask", "--request", request},
		{"map", "--as", "q", "--input", "ndjson", "--state-pointer", "/state", "--questions-json", `{"questions":{"q":{"type":"noul","instructions":"Is this?"}}}`},
		{"reduce", "--as", "q", "--input", "ndjson", "--questions-json", `{"questions":{"q":{"type":"noul","instructions":"Is this?"}}}`},
		{"rank", "--as", "q", "--state", "s", "--instruction", "Which?", "--id-pointer", "/id", "--criteria-pointer", "/criteria"},
		{"rate", "--as", "q", "--state-pointer", "/state", "--instruction", "How?", "--level", "Low", "--level", "High"},
		{"models"},
	}
	for _, args := range cases {
		input := `{"id":"a","state":"s","criteria":"A"}` + "\n"
		for _, arg := range args {
			if arg == "rank" {
				input = `[{"id":"a","state":"s","criteria":"A"}]`
			}
		}
		result := runBinary(t, input, map[string]string{"TYPESAFE_BASE_URL": "not-a-url", "TYPESAFE_API_KEY": ""}, args...)
		require.Equal(t, 2, result.exit, "args=%v", args)
		assert.Empty(t, result.stdout)
		assert.Contains(t, result.stderr, "TYPESAFE_BASE_URL")
	}
	assert.Zero(t, api.count())
}
