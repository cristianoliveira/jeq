package examples_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func codeSmellResponse() []byte {
	return []byte(`{"model":"configured-model","answers":{"primary_smell":{"type":"choice","choice":"none","probabilities":{"none":0.9},"confidence":0.9},"cohesive":{"type":"noul","noul":0.95}},"usage":{"input_tokens":12,"output_tokens":4,"server_usage_extra":"kept"},"server_response_extra":"kept"}`)
}

func TestCodeSmellReviewOneOrderedRequestAndTypedExtras(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "first file.go")
	second := filepath.Join(dir, "second.go")
	if err := os.WriteFile(first, []byte("package first\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(second, []byte("package second\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	api := newFakeAPI(t, func(int) (int, []byte) { return 200, codeSmellResponse() })
	result := runScript(t, "examples/code-smell-review/review.sh", "", api.server.URL, map[string]string{"TYPESAFE_DEFAULT_MODEL": "configured-model", "TYPESAFE_BASE_URL": api.server.URL}, first, second)
	if result.exit != 0 {
		t.Fatalf("exit=%d stdout=%q stderr=%q", result.exit, result.stdout, result.stderr)
	}
	if api.count() != 1 {
		t.Fatalf("requests=%d", api.count())
	}
	request := api.body(t, 0)
	if request["model"] != "configured-model" {
		t.Fatalf("model=%v", request["model"])
	}
	state, ok := request["state"].([]any)
	if !ok || len(state) != 2 {
		t.Fatalf("state=%#v", request["state"])
	}
	questions := request["questions"].(map[string]any)
	criteria := questions["primary_smell"].(map[string]any)["criteria"].(map[string]any)
	for _, key := range []string{"none", "mixed_responsibilities", "duplicated_policy", "hidden_ambient_state", "leaky_abstraction", "unnecessary_complexity"} {
		if _, ok := criteria[key]; !ok {
			t.Fatalf("missing criterion %q", key)
		}
	}
	if !strings.Contains(questions["cohesive"].(map[string]any)["instructions"].(string), "high=yes") {
		t.Fatal("cohesive question does not define high as safe")
	}
	for i, want := range []struct {
		path, content string
	}{{first, "package first\n"}, {second, "package second\n"}} {
		item, ok := state[i].(map[string]any)
		if !ok || item["path"] != want.path || item["content"] != want.content {
			t.Fatalf("state[%d]=%#v", i, state[i])
		}
	}
	resultDoc := oneJSON(t, result.stdout)
	evidence := resultDoc["_gev"].(map[string]any)["code_smells"].(map[string]any)
	if evidence["server_response_extra"] != "kept" || evidence["usage"].(map[string]any)["server_usage_extra"] != "kept" {
		t.Fatalf("extras=%#v", evidence)
	}
}

func TestCodeSmellReviewCleansTemporaryBundleAfterSuccessAndFailure(t *testing.T) {
	source := filepath.Join(t.TempDir(), "source.go")
	if err := os.WriteFile(source, []byte("package source\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Run("success", func(t *testing.T) {
		tmpdir := t.TempDir()
		api := newFakeAPI(t, func(int) (int, []byte) { return 200, codeSmellResponse() })
		result := runScript(t, "examples/code-smell-review/review.sh", "", api.server.URL, map[string]string{"TYPESAFE_BASE_URL": api.server.URL, "TMPDIR": tmpdir}, source)
		if result.exit != 0 {
			t.Fatalf("exit=%d stdout=%q stderr=%q", result.exit, result.stdout, result.stderr)
		}
		assertNoCodeSmellBundles(t, tmpdir)
	})

	t.Run("api failure", func(t *testing.T) {
		tmpdir := t.TempDir()
		api := newFakeAPI(t, func(int) (int, []byte) { return 500, []byte(`{"error":"failed"}`) })
		result := runScript(t, "examples/code-smell-review/review.sh", "", api.server.URL, map[string]string{"TYPESAFE_BASE_URL": api.server.URL, "TMPDIR": tmpdir}, source)
		if result.exit == 0 {
			t.Fatalf("expected failure: stdout=%q stderr=%q", result.stdout, result.stderr)
		}
		assertNoCodeSmellBundles(t, tmpdir)
	})
}

func assertNoCodeSmellBundles(t *testing.T, directory string) {
	t.Helper()
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "gev-code-smells.") {
			t.Fatalf("temporary bundle remains: %s", entry.Name())
		}
	}
}

func TestCodeSmellReviewRejectsInvalidInputsWithoutAPI(t *testing.T) {
	api := newFakeAPI(t, func(int) (int, []byte) { return 200, codeSmellResponse() })
	cases := []struct {
		name string
		args []string
	}{
		{"none", nil},
		{"missing", []string{"does not exist.go"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := runScript(t, "examples/code-smell-review/review.sh", "", api.server.URL, nil, tc.args...)
			if result.exit != 2 {
				t.Fatalf("exit=%d stdout=%q stderr=%q", result.exit, result.stdout, result.stderr)
			}
			if api.count() != 0 {
				t.Fatalf("invalid input made request: %d", api.count())
			}
		})
	}
	over := filepath.Join(t.TempDir(), "oversized.go")
	if err := os.WriteFile(over, []byte(strings.Repeat("x", 256*1024+1)), 0o644); err != nil {
		t.Fatal(err)
	}
	result := runScript(t, "examples/code-smell-review/review.sh", "", api.server.URL, nil, over)
	if result.exit != 2 || api.count() != 0 {
		t.Fatalf("oversize exit=%d requests=%d", result.exit, api.count())
	}
}

func TestCodeSmellReviewInlineQuestionsWithoutQuestionFile(t *testing.T) {
	api := newFakeAPI(t, func(int) (int, []byte) { return 200, codeSmellResponse() })
	input := "{\"path\":\"space name.go\",\"content\":\"package p\\n\"}\n"
	questions := `{"questions":{"primary_smell":{"type":"choice","instructions":"Which smell?","criteria":{"none":"none","mixed_responsibilities":"mixed","duplicated_policy":"duplicated","hidden_ambient_state":"ambient","leaky_abstraction":"leaky","unnecessary_complexity":"complex"}},"cohesive":{"type":"noul","instructions":"Answer high=yes and safe to pass; low=no."}}}`
	result := runGev(t, input, api.server.URL, []string{"reduce", "--as", "code_smells", "--input", "ndjson", "--questions-json", questions})
	if result.exit != 0 || api.count() != 1 {
		t.Fatalf("exit=%d requests=%d stdout=%q stderr=%q", result.exit, api.count(), result.stdout, result.stderr)
	}
	if api.body(t, 0)["model"] != "jev-latest" {
		t.Fatalf("default model=%v", api.body(t, 0)["model"])
	}
}

func runGev(t *testing.T, stdin, endpoint string, args []string) processResult {
	t.Helper()
	cmd := exec.Command(gevBin, args...)
	cmd.Dir = repoRoot
	home := t.TempDir()
	cmd.Env = envWith(map[string]string{"TYPESAFE_API_KEY": "examples-test-key", "TYPESAFE_BASE_URL": endpoint, "TYPESAFE_DEFAULT_MODEL": "", "HOME": home, "XDG_CONFIG_HOME": filepath.Join(home, "config"), "NO_COLOR": "1", "TERM": "dumb"}, nil)
	cmd.Stdin = strings.NewReader(stdin)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	_ = cmd.Run()
	return processResult{stdout: stdout.String(), stderr: stderr.String(), exit: processExit(cmd)}
}
