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

	"github.com/cristianoliveira/jeq/internal/cli"
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
	if err := os.Symlink(jeqBin, filepath.Join(temp, "jeq-test")); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
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
	if fake, ok := extra["TEST_FAKE_JEQ"]; ok {
		toolDir := t.TempDir()
		if err := os.Symlink(fake, filepath.Join(toolDir, "jeq")); err != nil {
			t.Fatal(err)
		}
		extra = cloneEnv(extra)
		delete(extra, "TEST_FAKE_JEQ")
		extra["PATH"] = toolDir + string(os.PathListSeparator) + os.Getenv("PATH")
	}
	cmd.Env = envWith(map[string]string{
		"PATH":              filepath.Dir(jeqBin) + string(os.PathListSeparator) + os.Getenv("PATH"),
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

func cloneEnv(input map[string]string) map[string]string {
	output := make(map[string]string, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
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
		result := runScript(t, "examples/change-risk-gate/gate.sh", "diff", "", map[string]string{"TEST_FAKE_JEQ": wrapper, "JEQ_REAL": jeqBin})
		if result.exit != 2 || strings.TrimSpace(result.stdout) != "" || !strings.HasPrefix(result.stderr, "Error: ") {
			t.Fatalf("exit=%d stderr=%q stdout=%q", result.exit, result.stderr, result.stdout)
		}
	})
	t.Run("jeq status 130 is unchanged", func(t *testing.T) {
		wrapper := writeWrapper(t, `printf '%s\n' '{"code":"JEQ_INTERRUPTED","message":"request interrupted","recovery":"rerun the command when ready"}'; exit 130`)
		result := runScript(t, "examples/change-risk-gate/gate.sh", "diff", "", map[string]string{"TEST_FAKE_JEQ": wrapper})
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
	if err := os.WriteFile(fake, []byte("#!/usr/bin/env bash\nset -euo pipefail\nprintf '1\\n' >>\"$FAKE_COUNT\"\nprintf '%s\\0' \"$@\" >\"$FAKE_ARGS\"\ncat >\"$FAKE_INPUT\"\nprintf '%s\\n' '{\"model\":\"fake\"}'\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	temp := t.TempDir()
	argsFile, inputFile, countFile := filepath.Join(temp, "args"), filepath.Join(temp, "input"), filepath.Join(temp, "count")
	result := runScript(t, "examples/release-readiness/review.sh", "", "", map[string]string{
		"TEST_FAKE_JEQ": fake, "FAKE_ARGS": argsFile, "FAKE_INPUT": inputFile, "FAKE_COUNT": countFile,
	})
	if result.exit != 0 || result.stderr != "" {
		t.Fatalf("exit=%d stderr=%q", result.exit, result.stderr)
	}
	argsData, _ := os.ReadFile(argsFile)
	args := bytes.Split(bytes.TrimSuffix(argsData, []byte{0}), []byte{0})
	questions, _ := os.ReadFile(filepath.Join(repoRoot, "examples/release-readiness/questions.json"))
	expectedArgs := []string{"reduce", "--as", "release_ready", "--input", "ndjson", "--questions-json", string(questions)}
	if got := stringSlice(args); !slicesEqual(got, expectedArgs) {
		t.Fatalf("args=%q expected=%q", args, expectedArgs)
	}
	count, _ := os.ReadFile(countFile)
	if string(count) != "1\n" {
		t.Fatalf("invocations=%q", count)
	}
	input, _ := os.ReadFile(inputFile)
	expected, _ := os.ReadFile(filepath.Join(repoRoot, "examples/release-readiness/findings.ndjson"))
	if string(input) != string(expected) {
		t.Fatalf("input=%q expected=%q", input, expected)
	}
}

func TestReleaseReadinessScriptPropagatesFailure(t *testing.T) {
	temp := t.TempDir()
	fake := filepath.Join(temp, "fake-jeq")
	countFile := filepath.Join(temp, "count")
	if err := os.WriteFile(fake, []byte("#!/usr/bin/env bash\nprintf '1\\n' >>\"$FAKE_COUNT\"\nprintf 'kept stdout\\n'\nprintf 'failure\\n' >&2\nexit 23\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	result := runScript(t, "examples/release-readiness/review.sh", "", "", map[string]string{"TEST_FAKE_JEQ": fake, "FAKE_COUNT": countFile})
	count, _ := os.ReadFile(countFile)
	if result.exit != 23 || result.stdout != "kept stdout\n" || result.stderr != "failure\n" || string(count) != "1\n" {
		t.Fatalf("exit=%d stdout=%q stderr=%q invocations=%q", result.exit, result.stdout, result.stderr, count)
	}
}

func stringSlice(values [][]byte) []string {
	result := make([]string, len(values))
	for i, value := range values {
		result[i] = string(value)
	}
	return result
}

func builtinRecipeShell(t *testing.T, recipe string) string {
	t.Helper()
	root := cli.NewExamplesCmd(cli.AskDeps{})
	command, _, err := root.Find([]string{recipe})
	if err != nil {
		t.Fatal(err)
	}
	if command.Example == "" {
		t.Fatalf("recipe %q has no copyable shell example", recipe)
	}
	return command.Example
}

func TestBuiltinExamplesParseAsBash(t *testing.T) {
	for _, recipe := range []string{"noul", "choice", "score", "rate-sort", "rank-top-k", "validate-native", "ask-native", "debug-chain", "map-gate", "reduce-gate", "map-reduce-gate"} {
		t.Run(recipe, func(t *testing.T) {
			cmd := exec.Command("bash", "-n", "-c", builtinRecipeShell(t, recipe))
			if output, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("invalid Bash example: %v: %s", err, output)
			}
		})
	}
}

func runBuiltinRecipe(t *testing.T, recipe string, extra map[string]string) processResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "bash", "-c", builtinRecipeShell(t, recipe))
	cmd.Dir = repoRoot
	cmd.Env = envWith(map[string]string{"PATH": os.Getenv("PATH"), "REAL_JEQ": jeqBin, "FAKE_SIGNAL_A": "0.9", "FAKE_SIGNAL_B": "0.9"}, extra)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	_ = cmd.Run()
	if ctx.Err() != nil {
		t.Fatalf("recipe %s timed out: stdout=%q stderr=%q", recipe, stdout.String(), stderr.String())
	}
	return processResult{stdout: stdout.String(), stderr: stderr.String(), exit: processExit(cmd)}
}

func TestCopyableMapGateRecipePropagatesPolicyAndPipelineStatuses(t *testing.T) {
	stub := filepath.Join(t.TempDir(), "jeq")
	stubSource := `#!/usr/bin/env bash
if [[ ${1:-} != map ]]; then exec "$REAL_JEQ" "$@"; fi
index=0
while IFS= read -r record; do
  index=$((index + 1))
  case "$index" in
    1) id=a; value=$FAKE_SIGNAL_A ;;
    2) id=b; value=$FAKE_SIGNAL_B ;;
    *) exit 2 ;;
  esac
  if [[ ${FAKE_MALFORMED:-0} == 1 ]]; then
    printf 'not-json\n'
  else
    printf '{"id":"%s","change":"synthetic","_jeq":{"risk":{"answers":{"risk":{"noul":%s}}}}}\n' "$id" "$value"
  fi
done
exit "${FAKE_MAP_EXIT:-0}"
`
	if err := os.WriteFile(stub, []byte(stubSource), 0o700); err != nil {
		t.Fatal(err)
	}
	toolDir := t.TempDir()
	if err := os.Symlink(stub, filepath.Join(toolDir, "jeq")); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name, signalA, signalB string
		mapExit, wantExit      int
		malformed              bool
		decisions              []string
	}{
		{name: "inclusive pass boundary", signalA: "0.8", signalB: "0.9", wantExit: 0, decisions: []string{"pass", "pass"}},
		{name: "reject", signalA: "0.4", signalB: "0.9", wantExit: 10, decisions: []string{"reject", "pass"}},
		{name: "uncertain", signalA: "0.5", signalB: "0.79", wantExit: 11, decisions: []string{"uncertain", "uncertain"}},
		{name: "reject takes precedence", signalA: "0.4", signalB: "0.5", wantExit: 10, decisions: []string{"reject", "uncertain"}},
		{name: "malformed mapped input", signalA: "0.9", signalB: "0.9", wantExit: 2, malformed: true},
		{name: "upstream failure despite successful gate and jq", signalA: "0.9", signalB: "0.9", mapExit: 23, wantExit: 23, decisions: []string{"pass", "pass"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			extra := map[string]string{
				"PATH":          toolDir + string(os.PathListSeparator) + os.Getenv("PATH"),
				"FAKE_SIGNAL_A": tc.signalA, "FAKE_SIGNAL_B": tc.signalB,
				"FAKE_MAP_EXIT": fmt.Sprint(tc.mapExit),
			}
			if tc.malformed {
				extra["FAKE_MALFORMED"] = "1"
			}
			result := runBuiltinRecipe(t, "map-gate", extra)
			if result.exit != tc.wantExit {
				t.Fatalf("exit=%d want=%d stdout=%q stderr=%q", result.exit, tc.wantExit, result.stdout, result.stderr)
			}
			if len(tc.decisions) == 0 {
				if result.stdout != "" || result.stderr == "" {
					t.Fatalf("malformed input output: stdout=%q stderr=%q", result.stdout, result.stderr)
				}
				return
			}
			records := ndjson(t, result.stdout)
			if len(records) != len(tc.decisions) {
				t.Fatalf("emitted %d decisions, want %d: %s", len(records), len(tc.decisions), result.stdout)
			}
			for i, record := range records {
				policy, ok := record["_jeq"].(map[string]any)["policy"].(map[string]any)
				if !ok || policy["decision"] != tc.decisions[i] {
					t.Errorf("decision %d=%#v want %q", i, policy, tc.decisions[i])
				}
			}
		})
	}
}

func slicesEqual(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}
