package blackbox_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type successCase struct {
	name, input string
	args        []string
	network     bool
}

func TestBlackBoxVerboseSuccessMatrixPreservesEvidenceAndRequests(t *testing.T) {
	requestPath := filepath.Join(t.TempDir(), "request.json")
	if err := os.WriteFile(requestPath, []byte(`{"model":"matrix","state":"hello","questions":{"q":{"type":"noul","instructions":"safe"}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	cases := []successCase{
		{"ask", "", []string{"ask", "--request", requestPath}, true},
		{"validate", `{"model":"matrix","state":"hello","questions":{"q":{"type":"noul","instructions":"safe"}}}`, []string{"validate", "--request", "-"}, false},
		{"map", `{"description":"a"}\n{"description":"b"}\n`, []string{"map", "--as", "q", "--input", "ndjson", "--state-pointer", "/description", "--questions-json", `{"questions":{"q":{"type":"noul","instructions":"safe"}}}`}, true},
		{"rate", `{"description":"a"}\n{"description":"b"}\n`, []string{"rate", "--as", "q", "--input", "ndjson", "--state-pointer", "/description", "--instruction", "safe", "--level", "Low", "--level", "High"}, true},
		{"reduce", `{"description":"a"}\n{"description":"b"}\n`, []string{"reduce", "--as", "q", "--input", "ndjson", "--questions-json", `{"questions":{"q":{"type":"noul","instructions":"safe"}}}`}, true},
		{"rank", `[{"id":"a","criteria":"A"},{"id":"b","criteria":"B"}]`, []string{"rank", "--as", "q", "--input", "json", "--state", "safe", "--instruction", "safe", "--id-pointer", "/id", "--criteria-pointer", "/criteria"}, true},
		{"gate", `{"score":0.9}\n{"score":0.1}\n`, []string{"gate", "--as", "policy", "--input", "ndjson", "--value-pointer", "/score", "--pass-min", "0.8", "--reject-max", "0.2"}, false},
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
								answers[k] = map[string]any{"type": "choice", "choice": "a"}
							case "score":
								answers[k] = map[string]any{"type": "score", "score": 1, "probabilities": map[string]float64{"0": 0.1, "1": 0.9}}
							default:
								answers[k] = map[string]any{"type": "noul", "noul": 0.9}
							}
						}
						_ = json.NewEncoder(w).Encode(map[string]any{"model": "matrix", "answers": answers})
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
			if plain.exit != verbose.exit || plain.stdout != verbose.stdout || pcount != vcount {
				t.Fatalf("default contract changed: plain=%#v verbose=%#v counts=%d/%d", plain, verbose, pcount, vcount)
			}
			if strings.Contains(verbose.stderr, "matrix-secret") || strings.Contains(verbose.stderr, "safe") {
				t.Fatalf("payload or credential leaked: %s", verbose.stderr)
			}
			if verbose.exit == 0 && !bytes.Contains([]byte(verbose.stderr), []byte(`"event":"operation.started"`)) {
				t.Fatalf("lifecycle missing: %s", verbose.stderr)
			}
		})
	}
}
