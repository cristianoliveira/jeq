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
	if err := os.WriteFile(config, []byte(`{"default_model":"chain-model"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	env := mergedEnv(map[string]string{"TYPESAFE_API_KEY": "chain-key", "TYPESAFE_BASE_URL": api.server.URL, "JEQ_CONFIG": config})
	run := func(input string, args ...string) string {
		cmd := exec.Command(jeqBin, args...)
		cmd.Dir = repoRoot
		cmd.Env = env
		cmd.Stdin = strings.NewReader(input)
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			t.Fatalf("args=%v err=%v stdout=%q stderr=%q", args, err, stdout.String(), stderr.String())
		}
		if stderr.Len() != 0 {
			t.Fatalf("args=%v stderr=%q", args, stderr.String())
		}
		return stdout.String()
	}
	mapped := run(`{"description":"incident"}
`, "map", "--as", "triage", "--input", "ndjson", "--state-pointer", "/description", "--questions-json", `{"questions":{"triage":{"type":"noul","instructions":"Is this urgent?"}}}`)
	rated := run(mapped, "rate", "--as", "severity", "--input", "ndjson", "--state-pointer", "/description", "--instruction", "How severe?", "--level", "Low", "--level", "High")
	reduced := run(rated, "reduce", "--as", "aggregate", "--input", "ndjson", "--questions-json", `{"questions":{"aggregate":{"type":"noul","instructions":"Is this coherent?"}}}`)
	if !strings.Contains(mapped, `"triage"`) || !strings.Contains(rated, `"severity"`) || !strings.Contains(reduced, `"aggregate"`) {
		t.Fatalf("chain outputs missing evidence: mapped=%q rated=%q reduced=%q", mapped, rated, reduced)
	}
	if api.count() != 3 {
		t.Fatalf("requests=%d want 3", api.count())
	}
	for i := 0; i < 3; i++ {
		var request map[string]any
		if err := json.Unmarshal(api.bodyAt(i), &request); err != nil {
			t.Fatal(err)
		}
		if request["model"] != "chain-model" {
			t.Fatalf("request %d model=%v", i, request["model"])
		}
	}
}
