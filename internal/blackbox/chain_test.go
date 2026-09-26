package blackbox_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBlackBoxEnvironmentChainMapRateReduce(t *testing.T) {
	api := newFakeAPI(t, func(w http.ResponseWriter, r *http.Request, _ int) {
		var request struct {
			Model     string `json:"model"`
			Questions map[string]struct {
				Type string `json:"type"`
			} `json:"questions"`
		}
		_ = json.NewDecoder(r.Body).Decode(&request)
		name, question := "", struct {
			Type string `json:"type"`
		}{}
		for key, value := range request.Questions {
			name, question = key, value
		}
		var answer map[string]any
		switch question.Type {
		case "score":
			answer = map[string]any{"type": "score", "score": 2, "probabilities": map[string]float64{"0": .1, "1": .9}}
		default:
			answer = map[string]any{"type": "noul", "noul": .9}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"model": "chain-model", "answers": map[string]any{name: answer}, "usage": map[string]int{"input_tokens": 1, "output_tokens": 1}})
	})
	tmp := t.TempDir()
	config := filepath.Join(tmp, "config.json")
	require.NoError(t, os.WriteFile(config, []byte(`{"default_model":"chain-model"}`), 0o600))
	env := mergedEnv(map[string]string{"TYPESAFE_API_KEY": "chain-key", "TYPESAFE_BASE_URL": api.server.URL, "JEQ_CONFIG": config, "JEQ_TRACE_ID": "chain-42"})
	run := func(input string, args ...string) (string, string) {
		cmd := exec.Command(jeqBin, args...)
		cmd.Dir = repoRoot
		cmd.Env = env
		cmd.Stdin = strings.NewReader(input)
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		require.NoError(t, cmd.Run(), "args=%v stdout=%q stderr=%q", args, stdout.String(), stderr.String())
		return stdout.String(), stderr.String()
	}
	mapped, mapTrace := run(`{"description":"incident"}
`, "--verbose", "map", "--as", "triage", "--input", "ndjson", "--state-pointer", "/description", "--questions-json", `{"questions":{"triage":{"type":"noul","instructions":"Is this urgent?"}}}`)
	rated, rateTrace := run(mapped, "--verbose", "rate", "--as", "severity", "--input", "ndjson", "--state-pointer", "/description", "--instruction", "How severe?", "--level", "Low", "--level", "High")
	reduced, reduceTrace := run(rated, "--verbose", "reduce", "--as", "aggregate", "--input", "ndjson", "--questions-json", `{"questions":{"aggregate":{"type":"noul","instructions":"Is this coherent?"}}}`)
	assert.Contains(t, mapTrace, `"trace_id":"chain-42"`)
	assert.Contains(t, rateTrace, `"trace_id":"chain-42"`)
	assert.Contains(t, reduceTrace, `"trace_id":"chain-42"`)
	assert.Contains(t, mapped, `"triage"`)
	assert.Contains(t, rated, `"severity"`)
	assert.Contains(t, reduced, `"aggregate"`)
	assert.Equal(t, 3, api.count())
	for i := 0; i < 3; i++ {
		var request map[string]any
		require.NoError(t, json.Unmarshal(api.bodyAt(i), &request))
		assert.Equal(t, "chain-model", request["model"], "request %d model", i)
	}
}
