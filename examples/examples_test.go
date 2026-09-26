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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
		require.NoError(t, os.Symlink(fake, filepath.Join(toolDir, "jeq")))
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
		require.FailNow(t, "script %s timed out: stdout=%q stderr=%q", script, stdout.String(), stderr.String())
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
	require.Less(t, index, len(a.bodies), "request %d was not received; count=%d", index+1, len(a.bodies))
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
	require.True(t, strings.HasSuffix(output, "\n") && strings.Count(output, "\n") == 1,
		"want one JSON document and newline, got %q", output)
	var document map[string]any
	require.NoError(t, json.Unmarshal([]byte(strings.TrimSuffix(output, "\n")), &document), "invalid JSON output %q", output)
	return document
}

func ndjson(t *testing.T, output string) []map[string]any {
	t.Helper()
	require.NotEmpty(t, output)
	require.True(t, strings.HasSuffix(output, "\n"), "want NDJSON with trailing newline, got %q", output)
	lines := strings.Split(strings.TrimSpace(output), "\n")
	result := make([]map[string]any, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var document map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &document), "invalid NDJSON line %q", line)
		result = append(result, document)
	}
	return result
}

func fixture(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repoRoot, path))
	require.NoError(t, err)
	return string(data)
}

func writeWrapper(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "jeq-wrapper.sh")
	require.NoError(t, os.WriteFile(path, []byte("#!/usr/bin/env bash\nset -euo pipefail\n"+body+"\n"), 0o700))
	return path
}

func assertQuestions(t *testing.T, body map[string]any, names ...string) {
	t.Helper()
	require.IsType(t, map[string]any{}, body["questions"])
	questions := body["questions"].(map[string]any)
	for _, name := range names {
		assert.Contains(t, questions, name, "question %q missing from request", name)
	}
}

func assertUsage(t *testing.T, document map[string]any) {
	t.Helper()
	require.IsType(t, map[string]any{}, document["usage"])
	usage := document["usage"].(map[string]any)
	assert.Equal(t, float64(10), usage["input_tokens"])
	assert.Equal(t, float64(2), usage["output_tokens"])
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
			require.Equal(t, 0, result.exit, "stderr=%q stdout=%q", result.stderr, result.stdout)
			assert.Empty(t, result.stderr)
			receipt := oneJSON(t, result.stdout)
			assert.Equal(t, tc.action, receipt["action"])
			assert.Equal(t, tc.route, receipt["route"])
			assertUsage(t, receipt)
			require.Equal(t, 1, api.count())
			body := api.body(t, 0)
			assertQuestions(t, body, "route", "urgent", "escalate")
			assert.Equal(t, fixture(t, "examples/support-routing/fixtures/ticket.txt"), body["state"])
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
			require.Equal(t, tc.exit, result.exit, "stderr=%q stdout=%q", result.stderr, result.stdout)
			assert.Empty(t, result.stderr)
			receipt := oneJSON(t, result.stdout)
			assert.Equal(t, tc.kind, receipt["status"])
			assert.Equal(t, tc.safe, receipt["safe_to_ship"])
			assertUsage(t, receipt)
			require.Equal(t, 1, api.count())
			assertQuestions(t, api.body(t, 0), "safe_to_ship")
		})
	}

	t.Run("jeq status 1 is unchanged", func(t *testing.T) {
		api := newFakeAPI(t, func(int) (int, []byte) { return http.StatusUnauthorized, []byte(`{"message":"denied"}`) })
		result := runScript(t, "examples/change-risk-gate/gate.sh", "diff", api.server.URL, nil)
		assert.Equal(t, 1, result.exit)
		assert.Empty(t, strings.TrimSpace(result.stdout))
		assert.Contains(t, result.stderr, "JEQ_AUTH_REJECTED")
	})
	t.Run("jeq status 2 is unchanged", func(t *testing.T) {
		wrapper := writeWrapper(t, `exec "$JEQ_REAL" "$@" --unknown-flag`)
		result := runScript(t, "examples/change-risk-gate/gate.sh", "diff", "", map[string]string{"TEST_FAKE_JEQ": wrapper, "JEQ_REAL": jeqBin})
		assert.Equal(t, 2, result.exit)
		assert.Empty(t, strings.TrimSpace(result.stdout))
		assert.True(t, strings.HasPrefix(result.stderr, "Error: "), "stderr=%q", result.stderr)
	})
	t.Run("jeq status 130 is unchanged", func(t *testing.T) {
		wrapper := writeWrapper(t, `printf '%s\n' '{"code":"JEQ_INTERRUPTED","message":"request interrupted","recovery":"rerun the command when ready"}'; exit 130`)
		result := runScript(t, "examples/change-risk-gate/gate.sh", "diff", "", map[string]string{"TEST_FAKE_JEQ": wrapper})
		assert.Equal(t, 130, result.exit)
		assert.Empty(t, result.stderr)
		doc := oneJSON(t, result.stdout)
		assert.Equal(t, "JEQ_INTERRUPTED", doc["code"])
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
	require.Equal(t, 0, result.exit, "stderr=%q stdout=%q", result.stderr, result.stdout)
	assert.Empty(t, result.stderr)
	lines := ndjson(t, result.stdout)
	require.Len(t, lines, 3)
	require.Equal(t, 3, api.count())
	wantIDs := []string{"ISSUE-101", "ISSUE-102", "ISSUE-103"}
	for i, line := range lines {
		assert.Equal(t, wantIDs[i], line["id"], "line %d id", i)
		assert.Equal(t, float64(i+1), line["order"], "line %d order", i)
		assert.Equal(t, float64(0.9), line["priority_confidence"], "line %d priority confidence", i)
		assert.Equal(t, float64(0.9), line["impact_confidence"], "line %d impact confidence", i)
		assertUsage(t, line)
		request := api.body(t, i)
		assertQuestions(t, request, "priority", "impact")
		assert.NotNil(t, request["state"], "ranking request %d must carry state", i)
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
		require.Equal(t, 1, failed.exit, "stderr=%q stdout=%q", failed.stderr, failed.stdout)
		require.Equal(t, 2, failAPI.count())
		assert.Contains(t, failed.stderr, "JEQ_SERVER_ERROR")
		lines := ndjson(t, failed.stdout)
		require.Len(t, lines, 1)
		assert.Equal(t, "ISSUE-101", lines[0]["id"])
	})
	t.Run("missing and invalid IDs are local input errors", func(t *testing.T) {
		cases := []string{`{"title":"missing id"}`, `{"id":42,"title":"non-string id"}`}
		for _, input := range cases {
			result := runScript(t, "examples/issue-ranking/rank.sh", input+"\n", "", nil)
			require.Equal(t, 2, result.exit, "stderr=%q stdout=%q", result.stderr, result.stdout)
			assert.Empty(t, result.stderr)
			receipt := oneJSON(t, result.stdout)
			assert.Equal(t, "input_invalid", receipt["status"])
			assert.NotNil(t, receipt["reason"])
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
		require.Equal(t, 11, result.exit, "stderr=%q stdout=%q", result.stderr, result.stdout)
		assert.Empty(t, result.stderr)
		assert.Equal(t, 1, missingScoreAPI.count())
		receipt := oneJSON(t, result.stdout)
		assert.Equal(t, "uncertain", receipt["status"])
		assert.Equal(t, "ISSUE-404", receipt["id"])
	})
	t.Run("out of range confidence is uncertain", func(t *testing.T) {
		confidenceAPI := newFakeAPI(t, func(int) (int, []byte) {
			return http.StatusOK, responseDocument(map[string]any{
				"priority": answerScoreWithConfidence(2, 1.01),
				"impact":   answerScoreWithConfidence(1, 0.9),
			})
		})
		result := runScript(t, "examples/issue-ranking/rank.sh", "{\"id\":\"ISSUE-405\"}\n", confidenceAPI.server.URL, nil)
		require.Equal(t, 11, result.exit, "stderr=%q stdout=%q", result.stderr, result.stdout)
		assert.Empty(t, result.stderr)
		assert.Equal(t, 1, confidenceAPI.count())
		receipt := oneJSON(t, result.stdout)
		assert.Equal(t, "uncertain", receipt["status"])
		assert.Equal(t, "ISSUE-405", receipt["id"])
	})
}

func TestReleaseReadinessScriptPassesExactReduceRequest(t *testing.T) {
	fake := filepath.Join(t.TempDir(), "fake-jeq")
	require.NoError(t, os.WriteFile(fake, []byte("#!/usr/bin/env bash\nset -euo pipefail\nprintf '1\\n' >>\"$FAKE_COUNT\"\nprintf '%s\\0' \"$@\" >\"$FAKE_ARGS\"\ncat >\"$FAKE_INPUT\"\nprintf '%s\\n' '{\"model\":\"fake\"}'\n"), 0o700))
	temp := t.TempDir()
	argsFile, inputFile, countFile := filepath.Join(temp, "args"), filepath.Join(temp, "input"), filepath.Join(temp, "count")
	result := runScript(t, "examples/release-readiness/review.sh", "", "", map[string]string{
		"TEST_FAKE_JEQ": fake, "FAKE_ARGS": argsFile, "FAKE_INPUT": inputFile, "FAKE_COUNT": countFile,
	})
	require.Equal(t, 0, result.exit, "stderr=%q", result.stderr)
	assert.Empty(t, result.stderr)
	argsData, err := os.ReadFile(argsFile)
	require.NoError(t, err)
	args := bytes.Split(bytes.TrimSuffix(argsData, []byte{0}), []byte{0})
	questions, err := os.ReadFile(filepath.Join(repoRoot, "examples/release-readiness/questions.json"))
	require.NoError(t, err)
	expectedArgs := []string{"reduce", "--as", "release_ready", "--input", "ndjson", "--questions-json", string(questions)}
	assert.Equal(t, expectedArgs, stringSlice(args))
	count, err := os.ReadFile(countFile)
	require.NoError(t, err)
	assert.Equal(t, "1\n", string(count))
	input, err := os.ReadFile(inputFile)
	require.NoError(t, err)
	expected, err := os.ReadFile(filepath.Join(repoRoot, "examples/release-readiness/findings.ndjson"))
	require.NoError(t, err)
	assert.Equal(t, string(expected), string(input))
}

func TestReleaseReadinessScriptPropagatesFailure(t *testing.T) {
	temp := t.TempDir()
	fake := filepath.Join(temp, "fake-jeq")
	countFile := filepath.Join(temp, "count")
	require.NoError(t, os.WriteFile(fake, []byte("#!/usr/bin/env bash\nprintf '1\\n' >>\"$FAKE_COUNT\"\nprintf 'kept stdout\\n'\nprintf 'failure\\n' >&2\nexit 23\n"), 0o700))
	result := runScript(t, "examples/release-readiness/review.sh", "", "", map[string]string{"TEST_FAKE_JEQ": fake, "FAKE_COUNT": countFile})
	count, err := os.ReadFile(countFile)
	require.NoError(t, err)
	assert.Equal(t, 23, result.exit)
	assert.Equal(t, "kept stdout\n", result.stdout)
	assert.Equal(t, "failure\n", result.stderr)
	assert.Equal(t, "1\n", string(count))
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
	require.NoError(t, err)
	require.NotNil(t, command)
	require.NotEmpty(t, command.Example, "recipe %q has no copyable shell example", recipe)
	return command.Example
}

func TestBuiltinExamplesParseAsBash(t *testing.T) {
	for _, recipe := range []string{"noul", "choice", "score", "rate-sort", "rank-top-k", "validate-native", "ask-native", "debug-chain", "map-gate", "reduce-gate", "map-reduce-gate"} {
		t.Run(recipe, func(t *testing.T) {
			cmd := exec.Command("bash", "-n", "-c", builtinRecipeShell(t, recipe))
			output, err := cmd.CombinedOutput()
			require.NoError(t, err, "invalid Bash example: %s", output)
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
		require.FailNow(t, "recipe %s timed out: stdout=%q stderr=%q", recipe, stdout.String(), stderr.String())
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
	require.NoError(t, os.WriteFile(stub, []byte(stubSource), 0o700))
	toolDir := t.TempDir()
	require.NoError(t, os.Symlink(stub, filepath.Join(toolDir, "jeq")))
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
			require.Equal(t, tc.wantExit, result.exit, "stdout=%q stderr=%q", result.stdout, result.stderr)
			if len(tc.decisions) == 0 {
				assert.Empty(t, result.stdout)
				assert.NotEmpty(t, result.stderr)
				return
			}
			records := ndjson(t, result.stdout)
			require.Len(t, records, len(tc.decisions), "stdout=%s", result.stdout)
			for i, record := range records {
				require.IsType(t, map[string]any{}, record["_jeq"])
				jeq := record["_jeq"].(map[string]any)
				require.IsType(t, map[string]any{}, jeq["policy"])
				policy := jeq["policy"].(map[string]any)
				assert.Equal(t, tc.decisions[i], policy["decision"], "decision %d policy=%#v", i, policy)
			}
		})
	}
}
