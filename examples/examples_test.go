package examples_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

var (
	repoRoot string
	jeqBin   string
)

func TestMain(m *testing.M) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		fmt.Fprintln(os.Stderr, "examples suite cannot locate itself")
		os.Exit(2)
	}
	repoRoot = filepath.Clean(filepath.Join(filepath.Dir(file), ".."))
	temp, err := os.MkdirTemp("", "jeq-examples-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	jeqBin = filepath.Join(temp, "jeq")
	build := exec.Command("go", "build", "-o", jeqBin, "./cmd/jeq")
	build.Dir = repoRoot
	build.Stdout = os.Stdout
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "building examples binary:", err)
		_ = os.RemoveAll(temp)
		os.Exit(2)
	}
	code := m.Run()
	_ = os.RemoveAll(temp)
	os.Exit(code)
}

type processResult struct {
	stdout string
	stderr string
	exit   int
}

func runScript(t *testing.T, script, input, endpoint string, extra map[string]string, args ...string) processResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	commandArgs := append([]string{filepath.Join(repoRoot, script)}, args...)
	cmd := exec.CommandContext(ctx, "bash", commandArgs...)
	cmd.Dir = repoRoot
	cmd.Env = envWith(map[string]string{
		"JEQ_BIN":           jeqBin,
		"TYPESAFE_BASE_URL": endpoint,
		"JEQ_MODEL":         "jev-latest",
		"TYPESAFE_API_KEY":  "examples-test-key",
		"NO_COLOR":          "1",
		"TERM":              "dumb",
	}, extra)
	cmd.Stdin = strings.NewReader(input)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil && ctx.Err() != nil {
		t.Fatalf("%s timed out: stdout=%q stderr=%q", script, stdout.String(), stderr.String())
	}
	return processResult{stdout: stdout.String(), stderr: stderr.String(), exit: processExit(cmd)}
}

func envWith(base, extra map[string]string) []string {
	overrides := make(map[string]string, len(base)+len(extra))
	for key, value := range base {
		overrides[key] = value
	}
	for key, value := range extra {
		overrides[key] = value
	}
	env := make([]string, 0, len(os.Environ())+len(overrides))
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if _, replaced := overrides[key]; !replaced {
			env = append(env, entry)
		}
	}
	for key, value := range overrides {
		env = append(env, key+"="+value)
	}
	return env
}

func processExit(cmd *exec.Cmd) int {
	if cmd.ProcessState == nil {
		return -1
	}
	return cmd.ProcessState.ExitCode()
}

type fakeAPI struct {
	server   *httptest.Server
	requests atomic.Int32
	first    chan struct{}
	once     sync.Once
	mu       sync.Mutex
	bodies   []map[string]any
	respond  func(int) (int, []byte)
}

func newFakeAPI(t *testing.T, respond func(int) (int, []byte)) *fakeAPI {
	t.Helper()
	api := &fakeAPI{first: make(chan struct{}), respond: respond}
	api.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := int(api.requests.Add(1))
		api.once.Do(func() { close(api.first) })
		body, _ := io.ReadAll(r.Body)
		var document map[string]any
		_ = json.Unmarshal(body, &document)
		api.mu.Lock()
		api.bodies = append(api.bodies, document)
		api.mu.Unlock()
		status, response := api.respond(count)
		w.WriteHeader(status)
		_, _ = w.Write(response)
	}))
	t.Cleanup(api.server.Close)
	return api
}

func (a *fakeAPI) count() int { return int(a.requests.Load()) }

func (a *fakeAPI) body(t *testing.T, index int) map[string]any {
	t.Helper()
	a.mu.Lock()
	defer a.mu.Unlock()
	if index >= len(a.bodies) {
		t.Fatalf("request %d was not received; count=%d", index+1, len(a.bodies))
	}
	return a.bodies[index]
}

func responseDocument(answers map[string]any) []byte {
	document := map[string]any{
		"model":   "jev-latest",
		"answers": answers,
		"usage":   map[string]any{"input_tokens": 10, "output_tokens": 2},
	}
	data, _ := json.Marshal(document)
	return data
}

func answerNoul(value float64) map[string]any {
	return map[string]any{"type": "noul", "noul": value}
}

func answerScore(value float64) map[string]any {
	return answerScoreWithConfidence(value, 0.9)
}

func answerScoreWithConfidence(value, confidence float64) map[string]any {
	return map[string]any{
		"type": "score", "score": value, "confidence": confidence,
		"legend":        map[string]string{"0": "low", "1": "medium", "2": "high", "3": "critical"},
		"probabilities": map[string]float64{"0": 0.1, "1": 0.2, "2": 0.3, "3": 0.4},
	}
}

func oneJSON(t *testing.T, output string) map[string]any {
	t.Helper()
	if !strings.HasSuffix(output, "\n") || strings.Count(output, "\n") != 1 {
		t.Fatalf("want one JSON document and newline, got %q", output)
	}
	var document map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSuffix(output, "\n")), &document); err != nil {
		t.Fatalf("invalid JSON output %q: %v", output, err)
	}
	return document
}

func ndjson(t *testing.T, output string) []map[string]any {
	t.Helper()
	if output == "" || !strings.HasSuffix(output, "\n") {
		t.Fatalf("want NDJSON with trailing newline, got %q", output)
	}
	lines := strings.Split(strings.TrimSpace(output), "\n")
	result := make([]map[string]any, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var document map[string]any
		if err := json.Unmarshal([]byte(line), &document); err != nil {
			t.Fatalf("invalid NDJSON line %q: %v", line, err)
		}
		result = append(result, document)
	}
	return result
}

func fixture(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repoRoot, path))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func writeWrapper(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "jeq-wrapper.sh")
	if err := os.WriteFile(path, []byte("#!/usr/bin/env bash\nset -euo pipefail\n"+body+"\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}

func assertQuestions(t *testing.T, body map[string]any, names ...string) {
	t.Helper()
	questions, ok := body["questions"].(map[string]any)
	if !ok {
		t.Fatalf("request questions=%#v", body["questions"])
	}
	for _, name := range names {
		if _, ok := questions[name]; !ok {
			t.Fatalf("question %q missing from request: %#v", name, questions)
		}
	}
}

func assertUsage(t *testing.T, document map[string]any) {
	t.Helper()
	usage, ok := document["usage"].(map[string]any)
	if !ok || usage["input_tokens"] != float64(10) || usage["output_tokens"] != float64(2) {
		t.Fatalf("usage=%#v", document["usage"])
	}
}

func TestSupportRoutingReceiptsAndAllowlist(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		route  string
		conf   float64
		action string
	}{
		{name: "allowlisted route", route: "technical", conf: 0.95, action: "technical_queue"},
		{name: "low confidence review", route: "billing", conf: 0.40, action: "human_review"},
		{name: "unknown route review", route: "other", conf: 0.99, action: "human_review"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			api := newFakeAPI(t, func(int) (int, []byte) {
				return http.StatusOK, responseDocument(map[string]any{
					"route":    map[string]any{"type": "choice", "choice": tc.route, "confidence": tc.conf, "probabilities": map[string]float64{tc.route: 1}},
					"urgent":   answerNoul(0.8),
					"escalate": answerNoul(0.2),
				})
			})
			result := runScript(t, "examples/support-routing/route.sh", fixture(t, "examples/support-routing/fixtures/ticket.txt"), api.server.URL, nil)
			if result.exit != 0 || result.stderr != "" {
				t.Fatalf("exit=%d stderr=%q stdout=%q", result.exit, result.stderr, result.stdout)
			}
			receipt := oneJSON(t, result.stdout)
			if receipt["action"] != tc.action || receipt["route"] != tc.route {
				t.Fatalf("receipt=%#v", receipt)
			}
			assertUsage(t, receipt)
			if api.count() != 1 {
				t.Fatalf("request count=%d", api.count())
			}
			body := api.body(t, 0)
			assertQuestions(t, body, "route", "urgent", "escalate")
			if body["state"] != fixture(t, "examples/support-routing/fixtures/ticket.txt") {
				t.Fatalf("state=%#v", body["state"])
			}
		})
	}
}

func TestChangeRiskGatePolicyAndOperationalStatus(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		safe float64
		exit int
		kind string
	}{
		{name: "exact review boundary", safe: 0.30, exit: 10, kind: "review_or_block"},
		{name: "just above review boundary", safe: 0.3001, exit: 11, kind: "uncertain"},
		{name: "representative uncertain midpoint", safe: 0.50, exit: 11, kind: "uncertain"},
		{name: "just below pass boundary", safe: 0.7999, exit: 11, kind: "uncertain"},
		{name: "exact pass boundary", safe: 0.80, exit: 0, kind: "pass"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			api := newFakeAPI(t, func(int) (int, []byte) {
				return http.StatusOK, responseDocument(map[string]any{"safe_to_ship": answerNoul(tc.safe)})
			})
			result := runScript(t, "examples/change-risk-gate/gate.sh", fixture(t, "examples/change-risk-gate/fixtures/change.diff"), api.server.URL, nil)
			if result.exit != tc.exit || result.stderr != "" {
				t.Fatalf("exit=%d want=%d stderr=%q stdout=%q", result.exit, tc.exit, result.stderr, result.stdout)
			}
			receipt := oneJSON(t, result.stdout)
			if receipt["status"] != tc.kind || receipt["safe_to_ship"] != tc.safe {
				t.Fatalf("receipt=%#v", receipt)
			}
			assertUsage(t, receipt)
			if api.count() != 1 {
				t.Fatalf("request count=%d", api.count())
			}
			assertQuestions(t, api.body(t, 0), "safe_to_ship")
		})
	}

	t.Run("jeq status 1 is unchanged", func(t *testing.T) {
		api := newFakeAPI(t, func(int) (int, []byte) { return http.StatusUnauthorized, []byte(`{"message":"denied"}`) })
		result := runScript(t, "examples/change-risk-gate/gate.sh", "diff", api.server.URL, nil)
		if result.exit != 1 || strings.TrimSpace(result.stdout) != "" || !strings.Contains(result.stderr, "JEQ_AUTH_REJECTED") {
			t.Fatalf("exit=%d stderr=%q stdout=%q", result.exit, result.stderr, result.stdout)
		}
	})
	t.Run("jeq status 2 is unchanged", func(t *testing.T) {
		wrapper := writeWrapper(t, `exec "$JEQ_REAL" "$@" --unknown-flag`)
		result := runScript(t, "examples/change-risk-gate/gate.sh", "diff", "", map[string]string{"JEQ_BIN": wrapper, "JEQ_REAL": jeqBin})
		if result.exit != 2 || strings.TrimSpace(result.stdout) != "" || !strings.HasPrefix(result.stderr, "Error: ") {
			t.Fatalf("exit=%d stderr=%q stdout=%q", result.exit, result.stderr, result.stdout)
		}
	})
	t.Run("jeq status 130 is unchanged", func(t *testing.T) {
		wrapper := writeWrapper(t, `printf '%s\n' '{"code":"JEQ_INTERRUPTED","message":"request interrupted","recovery":"rerun the command when ready"}'; exit 130`)
		result := runScript(t, "examples/change-risk-gate/gate.sh", "diff", "", map[string]string{"JEQ_BIN": wrapper})
		if result.exit != 130 || result.stderr != "" {
			t.Fatalf("exit=%d stderr=%q stdout=%q", result.exit, result.stderr, result.stdout)
		}
		doc := oneJSON(t, result.stdout)
		if doc["code"] != "JEQ_INTERRUPTED" {
			t.Fatalf("error=%#v", doc)
		}
	})
}

func TestIssueRankingOrderRequestCountAndFailFast(t *testing.T) {
	t.Parallel()
	responseByRequest := func(count int) (int, []byte) {
		return http.StatusOK, responseDocument(map[string]any{
			"priority": answerScore(float64(count)),
			"impact":   answerScore(float64(count - 1)),
		})
	}
	api := newFakeAPI(t, responseByRequest)
	input := fixture(t, "examples/issue-ranking/fixtures/issues.ndjson")
	result := runScript(t, "examples/issue-ranking/rank.sh", input, api.server.URL, nil)
	if result.exit != 0 || result.stderr != "" {
		t.Fatalf("exit=%d stderr=%q stdout=%q", result.exit, result.stderr, result.stdout)
	}
	lines := ndjson(t, result.stdout)
	if len(lines) != 3 || api.count() != 3 {
		t.Fatalf("lines=%d requests=%d", len(lines), api.count())
	}
	wantIDs := []string{"ISSUE-101", "ISSUE-102", "ISSUE-103"}
	for i, line := range lines {
		if line["id"] != wantIDs[i] || line["order"] != float64(i+1) ||
			line["priority_confidence"] != float64(0.9) || line["impact_confidence"] != float64(0.9) {
			t.Fatalf("line %d=%#v", i, line)
		}
		assertUsage(t, line)
		assertQuestions(t, api.body(t, i), "priority", "impact")
	}
	if api.body(t, 0)["state"] == nil || api.body(t, 1)["state"] == nil || api.body(t, 2)["state"] == nil {
		t.Fatal("ranking requests did not carry state")
	}

	t.Run("second operational error preserves partial output and stops", func(t *testing.T) {
		failAPI := newFakeAPI(t, func(count int) (int, []byte) {
			if count == 1 {
				return http.StatusOK, responseDocument(map[string]any{
					"priority": answerScore(1),
					"impact":   answerScore(0),
				})
			}
			return http.StatusInternalServerError, []byte("server detail")
		})
		failed := runScript(t, "examples/issue-ranking/rank.sh", input, failAPI.server.URL, nil)
		if failed.exit != 1 || failAPI.count() != 2 || !strings.Contains(failed.stderr, "JEQ_SERVER_ERROR") {
			t.Fatalf("exit=%d requests=%d stderr=%q stdout=%q", failed.exit, failAPI.count(), failed.stderr, failed.stdout)
		}
		lines := ndjson(t, failed.stdout)
		if len(lines) != 1 || lines[0]["id"] != "ISSUE-101" {
			t.Fatalf("partial output=%#v", lines)
		}
	})
	t.Run("missing and invalid IDs are local input errors", func(t *testing.T) {
		cases := []string{`{"title":"missing id"}`, `{"id":42,"title":"non-string id"}`}
		for _, input := range cases {
			result := runScript(t, "examples/issue-ranking/rank.sh", input+"\n", "", nil)
			if result.exit != 2 || result.stderr != "" {
				t.Fatalf("exit=%d stderr=%q stdout=%q", result.exit, result.stderr, result.stdout)
			}
			receipt := oneJSON(t, result.stdout)
			if receipt["status"] != "input_invalid" || receipt["reason"] == nil {
				t.Fatalf("receipt=%#v", receipt)
			}
		}
	})
	t.Run("missing numeric score is uncertain", func(t *testing.T) {
		missingScoreAPI := newFakeAPI(t, func(int) (int, []byte) {
			return http.StatusOK, responseDocument(map[string]any{
				"priority": answerScore(2),
				"impact":   answerNoul(0.5),
			})
		})
		result := runScript(t, "examples/issue-ranking/rank.sh", "{\"id\":\"ISSUE-404\",\"title\":\"missing score\"}\n", missingScoreAPI.server.URL, nil)
		if result.exit != 11 || result.stderr != "" || missingScoreAPI.count() != 1 {
			t.Fatalf("exit=%d requests=%d stderr=%q stdout=%q", result.exit, missingScoreAPI.count(), result.stderr, result.stdout)
		}
		receipt := oneJSON(t, result.stdout)
		if receipt["status"] != "uncertain" || receipt["id"] != "ISSUE-404" {
			t.Fatalf("receipt=%#v", receipt)
		}
	})
	t.Run("out of range confidence is uncertain", func(t *testing.T) {
		confidenceAPI := newFakeAPI(t, func(int) (int, []byte) {
			return http.StatusOK, responseDocument(map[string]any{
				"priority": answerScoreWithConfidence(2, 1.01),
				"impact":   answerScoreWithConfidence(1, 0.9),
			})
		})
		result := runScript(t, "examples/issue-ranking/rank.sh", "{\"id\":\"ISSUE-405\"}\n", confidenceAPI.server.URL, nil)
		if result.exit != 11 || result.stderr != "" || confidenceAPI.count() != 1 {
			t.Fatalf("exit=%d requests=%d stderr=%q stdout=%q", result.exit, confidenceAPI.count(), result.stderr, result.stdout)
		}
		receipt := oneJSON(t, result.stdout)
		if receipt["status"] != "uncertain" || receipt["id"] != "ISSUE-405" {
			t.Fatalf("receipt=%#v", receipt)
		}
	})
}

func TestReleaseReadinessScriptPassesExactReduceRequest(t *testing.T) {
	fake := filepath.Join(t.TempDir(), "fake-jeq")
	if err := os.WriteFile(fake, []byte("#!/usr/bin/env bash\nset -euo pipefail\nprintf '%s' \"$*\" >\"$FAKE_ARGS\"\ncat >\"$FAKE_INPUT\"\nprintf '%s\\n' '{\"model\":\"fake\"}'\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	argsFile, inputFile := filepath.Join(t.TempDir(), "args"), filepath.Join(t.TempDir(), "input")
	result := runScript(t, "examples/release-readiness/review.sh", "", "", map[string]string{
		"JEQ_BIN": fake, "FAKE_ARGS": argsFile, "FAKE_INPUT": inputFile,
	})
	if result.exit != 0 || result.stderr != "" {
		t.Fatalf("exit=%d stderr=%q", result.exit, result.stderr)
	}
	args, _ := os.ReadFile(argsFile)
	if !strings.HasPrefix(string(args), "reduce --as release_ready --input ndjson --questions-json ") {
		t.Fatalf("args=%q", args)
	}
	input, _ := os.ReadFile(inputFile)
	expected, _ := os.ReadFile(filepath.Join(repoRoot, "examples/release-readiness/findings.ndjson"))
	if string(input) != string(expected) {
		t.Fatalf("input=%q expected=%q", input, expected)
	}
}

func TestReleaseReadinessScriptPropagatesFailure(t *testing.T) {
	fake := filepath.Join(t.TempDir(), "fake-jeq")
	if err := os.WriteFile(fake, []byte("#!/usr/bin/env bash\nprintf 'failure\\n' >&2\nexit 23\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	result := runScript(t, "examples/release-readiness/review.sh", "", "", map[string]string{"JEQ_BIN": fake})
	if result.exit != 23 || result.stderr != "failure\n" {
		t.Fatalf("exit=%d stderr=%q", result.exit, result.stderr)
	}
}
