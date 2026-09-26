package blackbox_test

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type successCase struct {
	name, input                    string
	args                           []string
	network                        bool
	expectedExit, expectedRequests int
}

func TestBlackBoxVerboseSuccessMatrixPreservesEvidenceAndRequests(t *testing.T) {
	requestPath := filepath.Join(t.TempDir(), "request.json")
	require.NoError(t, os.WriteFile(requestPath, []byte(`{"model":"matrix","state":"hello","questions":{"q":{"type":"noul","instructions":"safe"}}}`), 0o600))
	cases := []successCase{
		{"ask", "", []string{"ask", "--request", requestPath}, true, 0, 1},
		{"validate", `{"model":"matrix","state":"hello","questions":{"q":{"type":"noul","instructions":"safe"}}}`, []string{"validate", "--request", "-"}, false, 0, 0},
		{"map", `{"description":"a"}\n{"description":"b"}\n`, []string{"map", "--as", "q", "--input", "ndjson", "--state-pointer", "/description", "--questions-json", `{"questions":{"q":{"type":"noul","instructions":"safe"}}}`}, true, 0, 2},
		{"rate", `{"description":"a"}\n{"description":"b"}\n`, []string{"rate", "--as", "q", "--input", "ndjson", "--state-pointer", "/description", "--instruction", "safe", "--level", "Low", "--level", "High"}, true, 0, 2},
		{"reduce", `{"description":"a"}\n{"description":"b"}\n`, []string{"reduce", "--as", "q", "--input", "ndjson", "--questions-json", `{"questions":{"q":{"type":"noul","instructions":"safe"}}}`}, true, 0, 1},
		{"rank", `[{"id":"a","criteria":"A"},{"id":"b","criteria":"B"}]`, []string{"rank", "--as", "q", "--input", "json", "--state", "safe", "--instruction", "safe", "--id-pointer", "/id", "--criteria-pointer", "/criteria"}, true, 0, 1},
		{"gate", `{"score":0.9}\n{"score":0.1}\n`, []string{"gate", "--as", "policy", "--input", "ndjson", "--value-pointer", "/score", "--pass-min", "0.8", "--reject-max", "0.2"}, false, 10, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			run := func(verbose bool) (processResult, int) {
				env := map[string]string{}
				var api *fakeAPI
				if tc.network {
					api = newFakeAPI(t, func(w http.ResponseWriter, r *http.Request, _ int) {
						var req struct {
							Questions map[string]struct {
								Type string `json:"type"`
							} `json:"questions"`
						}
						_ = json.NewDecoder(r.Body).Decode(&req)
						answers := map[string]any{}
						for k, q := range req.Questions {
							switch q.Type {
							case "choice":
								answers[k] = map[string]any{"type": "choice", "choice": "a", "probabilities": map[string]float64{"a": 0.8, "b": 0.2}, "confidence": 0.8}
							case "score":
								answers[k] = map[string]any{"type": "score", "score": 1, "probabilities": map[string]float64{"0": 0.1, "1": 0.9}, "legend": map[string]string{"0": "Low", "1": "High"}, "confidence": 0.9}
							default:
								answers[k] = map[string]any{"type": "noul", "noul": 0.9}
							}
						}
						answers["q"] = map[string]any{"type": "choice", "choice": "a", "probabilities": map[string]float64{"a": 0.8, "b": 0.2}, "confidence": 0.8}
						answers["department"] = answers["q"]
						_ = json.NewEncoder(w).Encode(map[string]any{"model": "matrix", "answers": answers, "usage": map[string]int{"input_tokens": 1, "output_tokens": 1}})
					})
					env = map[string]string{"TYPESAFE_BASE_URL": api.server.URL, "TYPESAFE_API_KEY": "matrix-secret"}
				}
				args := append([]string{}, tc.args...)
				if verbose {
					args = append([]string{"--verbose", "--trace-id", "matrix"}, args...)
				}
				input := strings.ReplaceAll(tc.input, `\n`, "\n")
				return runBinary(t, input, env, args...), func() int {
					if api != nil {
						return api.count()
					}
					return 0
				}()
			}
			plain, pcount := run(false)
			verbose, vcount := run(true)
			require.Equal(t, tc.expectedExit, plain.exit, "plain=%#v", plain)
			require.Equal(t, tc.expectedExit, verbose.exit, "verbose=%#v", verbose)
			require.Equal(t, tc.expectedRequests, pcount)
			require.Equal(t, tc.expectedRequests, vcount)
			assert.Equal(t, plain.stdout, verbose.stdout, "default output contract must be unchanged")
			assert.NotContains(t, verbose.stderr, "matrix-secret")
			assert.NotContains(t, verbose.stderr, "safe")
			if verbose.exit == 0 {
				assert.Contains(t, verbose.stderr, `"event":"operation.started"`)
			}
		})
	}
}
