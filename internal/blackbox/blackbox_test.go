package blackbox_test

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
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	repoRoot string
	jeqBin   string
)

func TestMain(m *testing.M) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		fmt.Fprintln(os.Stderr, "black-box suite cannot locate itself")
		os.Exit(2)
	}
	repoRoot = filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	temp, err := os.MkdirTemp("", "jeq-blackbox-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	defer func() { _ = os.RemoveAll(temp) }()
	jeqBin = filepath.Join(temp, "jeq")
	build := exec.Command("go", "build", "-o", jeqBin, "./cmd/jeq")
	build.Dir = repoRoot
	build.Stdout = os.Stdout
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "building black-box binary:", err)
		os.Exit(2)
	}
	os.Exit(m.Run())
}

type processResult struct {
	stdout string
	stderr string
	exit   int
	err    error
}

func runBinary(t *testing.T, stdin string, env map[string]string, args ...string) processResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, jeqBin, args...)
	cmd.Dir = repoRoot
	cmd.Env = mergedEnv(env)
	cmd.Stdin = strings.NewReader(stdin)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if ctx.Err() != nil {
		require.FailNow(t, fmt.Sprintf("%v timed out: stdout=%q stderr=%q", strings.Join(args, " "), stdout.String(), stderr.String()))
	}
	return processResult{stdout: stdout.String(), stderr: stderr.String(), exit: exitCode(cmd, err), err: err}
}

func mergedEnv(overrides map[string]string) []string {
	env := make([]string, 0, len(os.Environ())+len(overrides)+3)
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if _, replaced := overrides[key]; !replaced {
			env = append(env, entry)
		}
	}
	env = append(env, "NO_COLOR=1", "TERM=dumb", "CI=1")
	for key, value := range overrides {
		env = append(env, key+"="+value)
	}
	return env
}

func exitCode(cmd *exec.Cmd, err error) int {
	if cmd.ProcessState == nil {
		return -1
	}
	if err == nil {
		return 0
	}
	if status, ok := cmd.ProcessState.Sys().(syscall.WaitStatus); ok && status.Signaled() {
		return 128 + int(status.Signal())
	}
	return cmd.ProcessState.ExitCode()
}

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repoRoot, "internal", "fixtures", "contract", name))
	require.NoError(t, err)
	return data
}

func assertJSON(t *testing.T, output string) map[string]any {
	t.Helper()
	require.True(t, strings.HasSuffix(output, "\n") && strings.Count(output, "\n") == 1,
		"want one JSON document and one trailing newline, got %q", output)
	var doc map[string]any
	require.NoError(t, json.Unmarshal([]byte(strings.TrimSuffix(output, "\n")), &doc), "stdout=%q", output)
	return doc
}

func assertCleanMachineOutput(t *testing.T, result processResult, wantExit int, secret string) map[string]any {
	t.Helper()
	require.Equal(t, wantExit, result.exit, "stdout=%q stderr=%q err=%v", result.stdout, result.stderr, result.err)
	if secret != "" {
		assert.NotContains(t, result.stdout, secret, "secret leaked to stdout")
		assert.NotContains(t, result.stderr, secret, "secret leaked to stderr")
	}
	if wantExit != 0 {
		require.Empty(t, result.stdout, "failed commands must not write stdout")
		assert.True(t, strings.HasPrefix(result.stderr, "Error: "), "want standard stderr error, got %q", result.stderr)
		return map[string]any{"error": strings.TrimSpace(result.stderr)}
	}
	if strings.HasPrefix(result.stdout, "{") {
		return assertJSON(t, result.stdout)
	}
	doc := map[string]any{}
	for _, line := range strings.Split(strings.TrimSpace(result.stdout), "\n") {
		pair := strings.SplitN(line, ": ", 2)
		if len(pair) == 2 {
			var value any
			switch pair[1] {
			case "true":
				value = true
			case "false":
				value = false
			default:
				value = pair[1]
			}
			doc[pair[0]] = value
		}
	}
	return doc
}

type fakeAPI struct {
	server    *httptest.Server
	requests  atomic.Int32
	firstReq  chan struct{}
	closeOnce sync.Once
	bodyMu    sync.Mutex
	lastBody  []byte
	bodies    [][]byte
}

func newFakeAPI(t *testing.T, handler func(http.ResponseWriter, *http.Request, int)) *fakeAPI {
	t.Helper()
	api := &fakeAPI{firstReq: make(chan struct{})}
	api.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := int(api.requests.Add(1))
		api.closeOnce.Do(func() { close(api.firstReq) })
		body, _ := io.ReadAll(r.Body)
		api.bodyMu.Lock()
		api.lastBody = append([]byte(nil), body...)
		api.bodies = append(api.bodies, append([]byte(nil), body...))
		api.bodyMu.Unlock()
		r.Header.Set("X-Test-Body", string(body))
		handler(w, r, count)
	}))
	t.Cleanup(api.server.Close)
	return api
}

func (a *fakeAPI) count() int { return int(a.requests.Load()) }

func (a *fakeAPI) bodyAt(index int) []byte {
	a.bodyMu.Lock()
	defer a.bodyMu.Unlock()
	if index < 0 || index >= len(a.bodies) {
		return nil
	}
	return append([]byte(nil), a.bodies[index]...)
}

func (a *fakeAPI) body() []byte {
	a.bodyMu.Lock()
	defer a.bodyMu.Unlock()
	return append([]byte(nil), a.lastBody...)
}

func TestBlackBoxDiscoveryAndProseExceptions(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		args  []string
		prose bool
	}{
		{name: "root help default", args: nil, prose: true},
		{name: "version default", args: []string{"version"}},
		{name: "help", args: []string{"--help"}, prose: true},
		{name: "completion", args: []string{"completion", "bash"}, prose: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := runBinary(t, "", map[string]string{"TYPESAFE_API_KEY": ""}, tc.args...)
			if tc.prose {
				require.Equal(t, 0, result.exit, "stdout=%q stderr=%q", result.stdout, result.stderr)
				assert.NotEmpty(t, result.stdout)
				assert.False(t, strings.HasSuffix(result.stdout, "\n\n"), "prose output must not end with a blank line")
				return
			}
			assertCleanMachineOutput(t, result, 0, "")
		})
	}
}

func TestBlackBoxValidateSourcesAndNoStateEcho(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	requestPath := filepath.Join(tmp, "request.json")
	questionsPath := filepath.Join(tmp, "questions.json")
	statePath := filepath.Join(tmp, "state.json")
	require.NoError(t, os.WriteFile(requestPath, fixture(t, "request_full.json"), 0o600))
	require.NoError(t, os.WriteFile(questionsPath, []byte(`{"questions":{"q":{"type":"noul","instructions":"Is this urgent?"}}}`), 0o600))
	require.NoError(t, os.WriteFile(statePath, []byte(`{"customer":"do not echo this state"}`), 0o600))
	cases := []struct {
		name  string
		args  []string
		input string
	}{
		{name: "native file", args: []string{"validate", "--request", requestPath}},
		{name: "native stdin", args: []string{"validate", "--request", "-"}, input: string(fixture(t, "request_full.json"))},
		{name: "composed files", args: []string{"validate", "--questions", questionsPath, "--state-json-file", statePath}},
		{name: "composed stdin", args: []string{"validate", "--questions", "-", "--state", "literal state"}, input: `{"questions":{"q":{"type":"noul","instructions":"Is this urgent?"}}}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := runBinary(t, tc.input, map[string]string{"TYPESAFE_API_KEY": "should-not-be-read"}, tc.args...)
			doc := assertCleanMachineOutput(t, result, 0, "should-not-be-read")
			assert.Equal(t, true, doc["valid"])
			assert.NotContains(t, result.stdout, "do not echo this state")
			assert.NotContains(t, result.stdout, "literal state")
		})
	}
}

func TestBlackBoxAskNativeComposedFileStdinAndJSONModes(t *testing.T) {
	t.Parallel()
	response := fixture(t, "response_200_full.json")
	tmp := t.TempDir()
	requestPath := filepath.Join(tmp, "request.json")
	questionsPath := filepath.Join(tmp, "questions.json")
	require.NoError(t, os.WriteFile(requestPath, fixture(t, "request_full.json"), 0o600))
	require.NoError(t, os.WriteFile(questionsPath, []byte(`{"questions":{"q":{"type":"noul","instructions":"Is this urgent?"}}}`), 0o600))
	cases := []struct {
		name  string
		args  func(string) []string
		input string
	}{
		{name: "native file default", args: func(_ string) []string { return []string{"ask", "--request", requestPath} }},
		{name: "native stdin explicit json", args: func(_ string) []string {
			return []string{"ask", "--request", "-"}
		}, input: string(fixture(t, "request_full.json"))},
		{name: "composed file default", args: func(_ string) []string {
			return []string{"ask", "--questions", questionsPath, "--state", "a user needs help"}
		}},
		{name: "composed stdin explicit json", args: func(_ string) []string {
			return []string{"ask", "--questions", "-", "--state", "a user needs help"}
		}, input: `{"questions":{"q":{"type":"noul","instructions":"Is this urgent?"}}}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			api := newFakeAPI(t, func(w http.ResponseWriter, r *http.Request, _ int) {
				if r.Method != http.MethodPost || r.URL.Path != "/v1/systemone" || r.Header.Get("Authorization") != "Bearer blackbox-secret" {
					http.Error(w, "bad request", http.StatusBadRequest)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write(response)
			})
			result := runBinary(t, tc.input, map[string]string{"TYPESAFE_API_KEY": "blackbox-secret", "TYPESAFE_BASE_URL": api.server.URL}, tc.args(api.server.URL)...)
			assertCleanMachineOutput(t, result, 0, "blackbox-secret")
			assert.Equal(t, 1, api.count())
			var request map[string]any
			require.NoError(t, json.Unmarshal(api.body(), &request), "server request=%q", api.body())
			assert.NotNil(t, request["model"])
			assert.NotNil(t, request["questions"])
		})
	}
}

func TestBlackBoxInfrastructureFlagsAreRemoved(t *testing.T) {
	t.Parallel()
	api := newFakeAPI(t, func(w http.ResponseWriter, _ *http.Request, _ int) {
		http.Error(w, "must not call", http.StatusInternalServerError)
	})
	cases := [][]string{{"ask", "--base-url"}, {"map", "--base-url"}, {"reduce", "--base-url"}, {"rank", "--base-url"}, {"rate", "--base-url"}, {"models", "--base-url"}, {"map", "--config"}, {"reduce", "--config"}, {"rank", "--config"}, {"rate", "--config"}}
	for _, args := range cases {
		result := runBinary(t, "", map[string]string{"TYPESAFE_API_KEY": "must-not-read", "TYPESAFE_BASE_URL": api.server.URL}, append(args, "value")...)
		require.Equal(t, 2, result.exit, "args=%v result=%#v", args, result)
		assert.Empty(t, result.stdout)
		assert.Contains(t, result.stderr, "unknown flag")
	}
	assert.Zero(t, api.count())
}

func TestBlackBoxVersionAliases(t *testing.T) {
	t.Parallel()
	var outputs []string
	for _, args := range [][]string{{"version"}, {"--version"}, {"-v"}} {
		result := runBinary(t, "stdin must not be read", map[string]string{"TYPESAFE_API_KEY": ""}, args...)
		require.Equal(t, 0, result.exit, "args=%v", args)
		assert.Empty(t, result.stderr, "args=%v", args)
		outputs = append(outputs, result.stdout)
	}
	require.Len(t, outputs, 3)
	assert.NotEmpty(t, outputs[0])
	assert.Equal(t, outputs[0], outputs[1])
	assert.Equal(t, outputs[0], outputs[2])
}

func TestBlackBoxRankUsesOneRequestAndReturnsAllCandidates(t *testing.T) {
	t.Parallel()
	api := newFakeAPI(t, func(w http.ResponseWriter, r *http.Request, _ int) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/systemone" {
			http.Error(w, "wrong endpoint", http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte(`{"model":"jev-latest","answers":{"route":{"type":"choice","choice":"b","probabilities":{"a":0.2,"b":0.8},"confidence":0.8}},"usage":{"input_tokens":1,"output_tokens":1},"trace_id":"trace-1"}`))
	})
	input := "{\"name\":\"a\",\"description\":\"first\"}\n{\"name\":\"b\",\"description\":\"second\"}\n"
	result := runBinary(t, input, map[string]string{"TYPESAFE_BASE_URL": api.server.URL, "TYPESAFE_API_KEY": "rank-secret"}, "rank", "--as", "route", "--input", "ndjson", "--state", "request", "--instruction", "Which?", "--id-pointer", "/name", "--criteria-pointer", "/description")
	require.Equal(t, 0, result.exit, "stdout=%q stderr=%q", result.stdout, result.stderr)
	assert.Empty(t, result.stderr)
	assert.Equal(t, 1, api.count())
	var doc map[string]any
	require.NoError(t, json.Unmarshal([]byte(result.stdout), &doc))
	require.IsType(t, []any{}, doc["items"])
	items := doc["items"].([]any)
	require.Len(t, items, 2)
	require.IsType(t, map[string]any{}, items[0])
	assert.Equal(t, "b", items[0].(map[string]any)["id"])
	require.IsType(t, map[string]any{}, doc["_jeq"])
	jeqEvidence := doc["_jeq"].(map[string]any)
	require.IsType(t, map[string]any{}, jeqEvidence["route"])
	assert.Equal(t, "trace-1", jeqEvidence["route"].(map[string]any)["trace_id"])
	jq := exec.Command("jq", "-c", ".items[:1] | map(.candidate)")
	jq.Stdin = strings.NewReader(result.stdout)
	topK, err := jq.Output()
	require.NoError(t, err, "jq output=%q", topK)
	assert.Contains(t, string(topK), `"name":"b"`)
}

func TestBlackBoxRateUsesScoreAndComposesWithJQ(t *testing.T) {
	t.Parallel()
	api := newFakeAPI(t, func(w http.ResponseWriter, r *http.Request, _ int) {
		if r.URL.Path != "/v1/systemone" {
			http.Error(w, "wrong endpoint", http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte(`{"model":"m","answers":{"severity":{"type":"score","score":2,"legend":{"0":"low","1":"high"},"probabilities":{"0":0.1,"1":0.9},"confidence":0.9}},"usage":{"input_tokens":1,"output_tokens":1}}`))
	})
	input := "{\"description\":\"minor\"}\n{\"description\":\"outage\"}\n"
	result := runBinary(t, input, map[string]string{"TYPESAFE_BASE_URL": api.server.URL, "TYPESAFE_API_KEY": "rate-secret"}, "rate", "--as", "severity", "--input", "ndjson", "--state-pointer", "/description", "--instruction", "How severe?", "--level", "Low", "--level", "High")
	require.Equal(t, 0, result.exit, "stderr=%q", result.stderr)
	assert.Empty(t, result.stderr)
	assert.Equal(t, 2, api.count())
	for i, wantState := range []string{"minor", "outage"} {
		var request map[string]any
		require.NoError(t, json.Unmarshal(api.bodyAt(i), &request))
		assert.Equal(t, wantState, request["state"], "request %d state", i)
		require.IsType(t, map[string]any{}, request["questions"])
		questions := request["questions"].(map[string]any)
		require.IsType(t, map[string]any{}, questions["severity"])
		question := questions["severity"].(map[string]any)
		assert.Equal(t, "score", question["type"], "request %d", i)
		assert.Equal(t, "How severe?", question["instructions"], "request %d", i)
		require.IsType(t, []any{}, question["criteria"])
		assert.Equal(t, []any{"Low", "High"}, question["criteria"], "request %d criteria", i)
	}
	jq := exec.Command("jq", "-s", "sort_by(._jeq.severity.answers.severity.score) | reverse | length")
	jq.Stdin = strings.NewReader(result.stdout)
	top, err := jq.Output()
	require.NoError(t, err, "jq output=%q", top)
	assert.Equal(t, "2", strings.TrimSpace(string(top)))
}

func TestBlackBoxModelsAuthAndStatuses(t *testing.T) {
	t.Parallel()
	t.Run("models success", func(t *testing.T) {
		modelsBody := fixture(t, "models.json")
		api := newFakeAPI(t, func(w http.ResponseWriter, r *http.Request, _ int) {
			if r.Method != http.MethodGet || r.URL.Path != "/v1/models" {
				http.Error(w, "wrong endpoint", http.StatusBadRequest)
				return
			}
			_, _ = w.Write(modelsBody)
		})
		result := runBinary(t, "", map[string]string{"TYPESAFE_BASE_URL": api.server.URL, "TYPESAFE_API_KEY": "models-secret"}, "models")
		assertCleanMachineOutput(t, result, 0, "models-secret")
		assert.Equal(t, 1, api.count())
	})
	t.Run("missing key is pre-network", func(t *testing.T) {
		api := newFakeAPI(t, func(w http.ResponseWriter, _ *http.Request, _ int) {
			http.Error(w, "must not call", http.StatusInternalServerError)
		})
		result := runBinary(t, "", map[string]string{"TYPESAFE_BASE_URL": api.server.URL, "TYPESAFE_API_KEY": ""}, "models")
		doc := assertCleanMachineOutput(t, result, 1, "")
		assert.NotEmpty(t, doc["error"])
		assert.Zero(t, api.count())
	})
	t.Run("auth rejection", func(t *testing.T) {
		api := newFakeAPI(t, func(w http.ResponseWriter, _ *http.Request, _ int) {
			http.Error(w, `{"message":"secret should not appear"}`, http.StatusUnauthorized)
		})
		result := runBinary(t, "", map[string]string{"TYPESAFE_BASE_URL": api.server.URL, "TYPESAFE_API_KEY": "auth-secret"}, "models")
		doc := assertCleanMachineOutput(t, result, 1, "auth-secret")
		assert.NotEmpty(t, doc["error"])
		assert.Equal(t, 1, api.count())
	})
}

func TestBlackBoxRetriesMalformedTimeoutLowConfidenceAndDrop(t *testing.T) {
	t.Parallel()
	response := fixture(t, "response_200_full.json")
	tmp := t.TempDir()
	requestPath := filepath.Join(tmp, "request.json")
	require.NoError(t, os.WriteFile(requestPath, fixture(t, "request_full.json"), 0o600))
	t.Run("retry", func(t *testing.T) {
		api := newFakeAPI(t, func(w http.ResponseWriter, _ *http.Request, count int) {
			if count == 1 {
				w.Header().Set("Retry-After", "0")
				w.WriteHeader(http.StatusTooManyRequests)
				return
			}
			_, _ = w.Write(response)
		})
		result := runBinary(t, "", map[string]string{"TYPESAFE_BASE_URL": api.server.URL, "TYPESAFE_API_KEY": "retry-secret", "JEQ_TRACE_ID": "retry-test"}, "--verbose", "ask", "--request", requestPath)
		assertCleanMachineOutput(t, result, 0, "retry-secret")
		assert.Contains(t, result.stderr, `"event":"request.attempted"`)
		assert.Contains(t, result.stderr, `"http_status":429`)
		assert.Contains(t, result.stderr, `"event":"request.retrying"`)
		assert.NotContains(t, result.stderr, "retry-secret")
		assert.Equal(t, 2, api.count())
		assert.Contains(t, result.stderr, "retry 1/2")
	})
	t.Run("server status failure", func(t *testing.T) {
		api := newFakeAPI(t, func(w http.ResponseWriter, _ *http.Request, _ int) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`<html>dependency stack should stay contained</html>`))
		})
		result := runBinary(t, "", map[string]string{"TYPESAFE_BASE_URL": api.server.URL, "TYPESAFE_API_KEY": "status-secret"}, "ask", "--request", requestPath)
		doc := assertCleanMachineOutput(t, result, 1, "status-secret")
		assert.NotEmpty(t, doc["error"])
		assert.Equal(t, 1, api.count())
		assert.NotContains(t, result.stdout, "dependency stack")
	})
	t.Run("malformed response", func(t *testing.T) {
		api := newFakeAPI(t, func(w http.ResponseWriter, _ *http.Request, _ int) { _, _ = w.Write([]byte("not-json")) })
		result := runBinary(t, "", map[string]string{"TYPESAFE_BASE_URL": api.server.URL, "TYPESAFE_API_KEY": "malformed-secret"}, "ask", "--request", requestPath)
		doc := assertCleanMachineOutput(t, result, 1, "malformed-secret")
		assert.NotEmpty(t, doc["error"])
		assert.Equal(t, 1, api.count())
	})
	t.Run("timeout", func(t *testing.T) {
		api := newFakeAPI(t, func(_ http.ResponseWriter, r *http.Request, _ int) { <-r.Context().Done() })
		result := runBinary(t, "", map[string]string{"TYPESAFE_BASE_URL": api.server.URL, "TYPESAFE_API_KEY": "timeout-secret"}, "ask", "--request", requestPath, "--timeout", "50ms")
		doc := assertCleanMachineOutput(t, result, 1, "timeout-secret")
		assert.NotEmpty(t, doc["error"])
		assert.Equal(t, 1, api.count())
	})
	t.Run("low confidence is success", func(t *testing.T) {
		low := bytes.Replace(response, []byte(`"confidence": 1.0`), []byte(`"confidence": 0.01`), 1)
		api := newFakeAPI(t, func(w http.ResponseWriter, _ *http.Request, _ int) { _, _ = w.Write(low) })
		result := runBinary(t, "", map[string]string{"TYPESAFE_BASE_URL": api.server.URL, "TYPESAFE_API_KEY": "confidence-secret"}, "ask", "--request", requestPath)
		assertCleanMachineOutput(t, result, 0, "confidence-secret")
	})
	t.Run("connection drop is never replayed", func(t *testing.T) {
		hijackResult := make(chan error, 1)
		api := newFakeAPI(t, func(w http.ResponseWriter, _ *http.Request, _ int) {
			hijacker, ok := w.(http.Hijacker)
			if !ok {
				hijackResult <- fmt.Errorf("server does not support hijacking")
				return
			}
			conn, _, err := hijacker.Hijack()
			if err != nil {
				hijackResult <- err
				return
			}
			_ = conn.Close()
			hijackResult <- nil
		})
		result := runBinary(t, "", map[string]string{"TYPESAFE_BASE_URL": api.server.URL, "TYPESAFE_API_KEY": "drop-secret"}, "ask", "--request", requestPath)
		require.Equal(t, 1, api.count())
		require.NoError(t, <-hijackResult)
		doc := assertCleanMachineOutput(t, result, 1, "drop-secret")
		assert.NotEmpty(t, doc["error"])
		assert.NotContains(t, result.stderr, "EOF")
		assert.NotContains(t, result.stderr, "connection reset")
	})
}

func TestBlackBoxUsageAndFailureSeparation(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		args []string
	}{
		{name: "unknown command", args: []string{"badsubcommand"}},
		{name: "unknown flag", args: []string{"--nope"}},
		{name: "misspelled command", args: []string{"versio"}},
		{name: "unsupported output", args: []string{"version", "--output", "toon"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := runBinary(t, "", map[string]string{"TYPESAFE_API_KEY": ""}, tc.args...)
			doc := assertCleanMachineOutput(t, result, 2, "")
			assert.True(t, strings.HasPrefix(result.stderr, "Error: "), "standard usage error missing: %q", result.stderr)
			assert.NotEmpty(t, doc["error"])
		})
	}
}

func TestBlackBoxInterruptBlockedAsk(t *testing.T) {
	api := newFakeAPI(t, func(_ http.ResponseWriter, r *http.Request, _ int) { <-r.Context().Done() })
	tmp := t.TempDir()
	requestPath := filepath.Join(tmp, "request.json")
	require.NoError(t, os.WriteFile(requestPath, fixture(t, "request_full.json"), 0o600))
	cmd := exec.Command(jeqBin, "--verbose", "ask", "--request", requestPath, "--timeout", "10s")
	cmd.Dir = repoRoot
	cmd.Env = mergedEnv(map[string]string{"TYPESAFE_API_KEY": "interrupt-secret", "TYPESAFE_BASE_URL": api.server.URL})
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	require.NoError(t, cmd.Start())
	select {
	case <-api.firstReq:
	case <-time.After(5 * time.Second):
		_ = cmd.Process.Kill()
		require.FailNow(t, "blocked ask did not reach fake server")
	}
	require.NoError(t, cmd.Process.Signal(os.Interrupt))
	wait := make(chan error, 1)
	go func() { wait <- cmd.Wait() }()
	select {
	case err := <-wait:
		require.Equal(t, 130, exitCode(cmd, err), "stdout=%q stderr=%q err=%v", stdout.String(), stderr.String(), err)
		assert.Empty(t, stdout.String())
		assert.Contains(t, stderr.String(), "JEQ_INTERRUPTED")
		assert.NotContains(t, stderr.String(), "interrupt-secret")
		assert.Contains(t, stderr.String(), `"phase":"transport"`)
		assert.Contains(t, stderr.String(), `"event":"run.failed"`)
	case <-time.After(5 * time.Second):
		_ = cmd.Process.Kill()
		require.FailNow(t, "interrupted ask did not exit by deadline")
	}
}
