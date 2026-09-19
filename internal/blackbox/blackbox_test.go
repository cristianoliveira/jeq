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
		t.Fatalf("%v timed out: stdout=%q stderr=%q", strings.Join(args, " "), stdout.String(), stderr.String())
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
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func assertJSON(t *testing.T, output string) map[string]any {
	t.Helper()
	if !strings.HasSuffix(output, "\n") || strings.Count(output, "\n") != 1 {
		t.Fatalf("want one JSON document and one trailing newline, got %q", output)
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSuffix(output, "\n")), &doc); err != nil {
		t.Fatalf("stdout is not JSON: %v (%q)", err, output)
	}
	return doc
}

func assertCleanMachineOutput(t *testing.T, result processResult, wantExit int, secret string) map[string]any {
	t.Helper()
	if result.exit != wantExit {
		t.Fatalf("exit=%d want=%d stdout=%q stderr=%q err=%v", result.exit, wantExit, result.stdout, result.stderr, result.err)
	}
	if secret != "" && (strings.Contains(result.stdout, secret) || strings.Contains(result.stderr, secret)) {
		t.Fatalf("secret leaked in stdout/stderr: stdout=%q stderr=%q", result.stdout, result.stderr)
	}
	if wantExit != 0 {
		if result.stdout != "" || !strings.HasPrefix(result.stderr, "Error: ") {
			t.Fatalf("want standard stderr error: stdout=%q stderr=%q", result.stdout, result.stderr)
		}
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
		api.bodyMu.Unlock()
		handler(w, r, count)
	}))
	t.Cleanup(api.server.Close)
	return api
}

func (a *fakeAPI) count() int { return int(a.requests.Load()) }

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
				if result.exit != 0 || result.stdout == "" || strings.HasSuffix(result.stdout, "\n\n") {
					t.Fatalf("prose command exit=%d stdout=%q stderr=%q", result.exit, result.stdout, result.stderr)
				}
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
	if err := os.WriteFile(requestPath, fixture(t, "request_full.json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(questionsPath, []byte(`{"questions":{"q":{"type":"noul","instructions":"Is this urgent?"}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(statePath, []byte(`{"customer":"do not echo this state"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name  string
		args  []string
		input string
	}{
		{name: "native file", args: []string{"validate", "--request", requestPath}},
		{name: "native stdin", args: []string{"validate", "--request", "-"}, input: string(fixture(t, "request_full.json"))},
		{name: "composed files", args: []string{"validate", "--questions", questionsPath, "--state-json", statePath}},
		{name: "composed stdin", args: []string{"validate", "--questions", "-", "--state", "literal state"}, input: `{"questions":{"q":{"type":"noul","instructions":"Is this urgent?"}}}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := runBinary(t, tc.input, map[string]string{"TYPESAFE_API_KEY": "should-not-be-read"}, tc.args...)
			doc := assertCleanMachineOutput(t, result, 0, "should-not-be-read")
			if doc["valid"] != true || strings.Contains(result.stdout, "do not echo this state") || strings.Contains(result.stdout, "literal state") {
				t.Fatalf("unexpected validation document: %q", result.stdout)
			}
		})
	}
}

func TestBlackBoxAskNativeComposedFileStdinAndJSONModes(t *testing.T) {
	t.Parallel()
	response := fixture(t, "response_200_full.json")
	tmp := t.TempDir()
	requestPath := filepath.Join(tmp, "request.json")
	questionsPath := filepath.Join(tmp, "questions.json")
	if err := os.WriteFile(requestPath, fixture(t, "request_full.json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(questionsPath, []byte(`{"questions":{"q":{"type":"noul","instructions":"Is this urgent?"}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name  string
		args  func(string) []string
		input string
	}{
		{name: "native file default", args: func(url string) []string { return []string{"ask", "--request", requestPath, "--base-url", url} }},
		{name: "native stdin explicit json", args: func(url string) []string {
			return []string{"ask", "--request", "-", "--base-url", url}
		}, input: string(fixture(t, "request_full.json"))},
		{name: "composed file default", args: func(url string) []string {
			return []string{"ask", "--questions", questionsPath, "--state", "a user needs help", "--base-url", url}
		}},
		{name: "composed stdin explicit json", args: func(url string) []string {
			return []string{"ask", "--questions", "-", "--state", "a user needs help", "--base-url", url}
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
			result := runBinary(t, tc.input, map[string]string{"TYPESAFE_API_KEY": "blackbox-secret"}, tc.args(api.server.URL)...)
			assertCleanMachineOutput(t, result, 0, "blackbox-secret")
			if api.count() != 1 {
				t.Fatalf("request count=%d want=1", api.count())
			}
			var request map[string]any
			if err := json.Unmarshal(api.body(), &request); err != nil || request["model"] == nil || request["questions"] == nil {
				t.Fatalf("server received invalid request %q: %v", api.body(), err)
			}
		})
	}
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
	result := runBinary(t, input, map[string]string{"TYPESAFE_API_KEY": "rank-secret"}, "rank", "--as", "route", "--input", "ndjson", "--state", "request", "--instruction", "Which?", "--id-pointer", "/name", "--criteria-pointer", "/description", "--base-url", api.server.URL)
	if result.exit != 0 || result.stderr != "" || api.count() != 1 {
		t.Fatalf("exit=%d stderr=%q requests=%d stdout=%q", result.exit, result.stderr, api.count(), result.stdout)
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(result.stdout), &doc); err != nil {
		t.Fatal(err)
	}
	items := doc["items"].([]any)
	if items[0].(map[string]any)["id"] != "b" || len(items) != 2 {
		t.Fatalf("items=%#v", items)
	}
	if doc["_jeq"].(map[string]any)["route"].(map[string]any)["trace_id"] != "trace-1" {
		t.Fatalf("unknown response field was not preserved: %#v", doc)
	}
	jq := exec.Command("jq", "-c", ".items[:1] | map(.candidate)")
	jq.Stdin = strings.NewReader(result.stdout)
	topK, err := jq.Output()
	if err != nil || !strings.Contains(string(topK), `"name":"b"`) {
		t.Fatalf("jq top-k=%q err=%v", topK, err)
	}
}

func TestBlackBoxModelsAuthAndStatuses(t *testing.T) {
	t.Parallel()
	t.Run("models success", func(t *testing.T) {
		api := newFakeAPI(t, func(w http.ResponseWriter, r *http.Request, _ int) {
			if r.Method != http.MethodGet || r.URL.Path != "/v1/models" {
				http.Error(w, "wrong endpoint", http.StatusBadRequest)
				return
			}
			_, _ = w.Write(fixture(t, "models.json"))
		})
		result := runBinary(t, "", map[string]string{"TYPESAFE_API_KEY": "models-secret"}, "models", "--base-url", api.server.URL)
		assertCleanMachineOutput(t, result, 0, "models-secret")
		if api.count() != 1 {
			t.Fatalf("request count=%d", api.count())
		}
	})
	t.Run("missing key is pre-network", func(t *testing.T) {
		api := newFakeAPI(t, func(w http.ResponseWriter, _ *http.Request, _ int) {
			http.Error(w, "must not call", http.StatusInternalServerError)
		})
		result := runBinary(t, "", map[string]string{"TYPESAFE_API_KEY": ""}, "models", "--base-url", api.server.URL)
		doc := assertCleanMachineOutput(t, result, 1, "")
		if doc["error"] == "" || api.count() != 0 {
			t.Fatalf("doc=%v requests=%d", doc, api.count())
		}
	})
	t.Run("auth rejection", func(t *testing.T) {
		api := newFakeAPI(t, func(w http.ResponseWriter, _ *http.Request, _ int) {
			http.Error(w, `{"message":"secret should not appear"}`, http.StatusUnauthorized)
		})
		result := runBinary(t, "", map[string]string{"TYPESAFE_API_KEY": "auth-secret"}, "models", "--base-url", api.server.URL)
		doc := assertCleanMachineOutput(t, result, 1, "auth-secret")
		if doc["error"] == "" || api.count() != 1 {
			t.Fatalf("doc=%v requests=%d", doc, api.count())
		}
	})
}

func TestBlackBoxRetriesMalformedTimeoutLowConfidenceAndDrop(t *testing.T) {
	t.Parallel()
	response := fixture(t, "response_200_full.json")
	tmp := t.TempDir()
	requestPath := filepath.Join(tmp, "request.json")
	if err := os.WriteFile(requestPath, fixture(t, "request_full.json"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Run("retry", func(t *testing.T) {
		api := newFakeAPI(t, func(w http.ResponseWriter, _ *http.Request, count int) {
			if count == 1 {
				w.Header().Set("Retry-After", "0")
				w.WriteHeader(http.StatusTooManyRequests)
				return
			}
			_, _ = w.Write(response)
		})
		result := runBinary(t, "", map[string]string{"TYPESAFE_API_KEY": "retry-secret"}, "ask", "--request", requestPath, "--base-url", api.server.URL)
		assertCleanMachineOutput(t, result, 0, "retry-secret")
		if api.count() != 2 || !strings.Contains(result.stderr, "retry 1/2") {
			t.Fatalf("requests=%d stderr=%q", api.count(), result.stderr)
		}
	})
	t.Run("server status failure", func(t *testing.T) {
		api := newFakeAPI(t, func(w http.ResponseWriter, _ *http.Request, _ int) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`<html>dependency stack should stay contained</html>`))
		})
		result := runBinary(t, "", map[string]string{"TYPESAFE_API_KEY": "status-secret"}, "ask", "--request", requestPath, "--base-url", api.server.URL)
		doc := assertCleanMachineOutput(t, result, 1, "status-secret")
		if doc["error"] == "" || api.count() != 1 || strings.Contains(result.stdout, "dependency stack") {
			t.Fatalf("doc=%v requests=%d stdout=%q", doc, api.count(), result.stdout)
		}
	})
	t.Run("malformed response", func(t *testing.T) {
		api := newFakeAPI(t, func(w http.ResponseWriter, _ *http.Request, _ int) { _, _ = w.Write([]byte("not-json")) })
		result := runBinary(t, "", map[string]string{"TYPESAFE_API_KEY": "malformed-secret"}, "ask", "--request", requestPath, "--base-url", api.server.URL)
		doc := assertCleanMachineOutput(t, result, 1, "malformed-secret")
		if doc["error"] == "" || api.count() != 1 {
			t.Fatalf("doc=%v requests=%d", doc, api.count())
		}
	})
	t.Run("timeout", func(t *testing.T) {
		api := newFakeAPI(t, func(_ http.ResponseWriter, r *http.Request, _ int) { <-r.Context().Done() })
		result := runBinary(t, "", map[string]string{"TYPESAFE_API_KEY": "timeout-secret"}, "ask", "--request", requestPath, "--base-url", api.server.URL, "--timeout", "50ms")
		doc := assertCleanMachineOutput(t, result, 1, "timeout-secret")
		if doc["error"] == "" || api.count() != 1 {
			t.Fatalf("doc=%v requests=%d", doc, api.count())
		}
	})
	t.Run("low confidence is success", func(t *testing.T) {
		low := bytes.Replace(response, []byte(`"confidence": 1.0`), []byte(`"confidence": 0.01`), 1)
		api := newFakeAPI(t, func(w http.ResponseWriter, _ *http.Request, _ int) { _, _ = w.Write(low) })
		result := runBinary(t, "", map[string]string{"TYPESAFE_API_KEY": "confidence-secret"}, "ask", "--request", requestPath, "--base-url", api.server.URL)
		assertCleanMachineOutput(t, result, 0, "confidence-secret")
	})
	t.Run("connection drop is never replayed", func(t *testing.T) {
		api := newFakeAPI(t, func(w http.ResponseWriter, _ *http.Request, _ int) {
			hijacker, ok := w.(http.Hijacker)
			if !ok {
				t.Fatal("server does not support hijacking")
			}
			conn, _, err := hijacker.Hijack()
			if err != nil {
				t.Fatal(err)
			}
			_ = conn.Close()
		})
		result := runBinary(t, "", map[string]string{"TYPESAFE_API_KEY": "drop-secret"}, "ask", "--request", requestPath, "--base-url", api.server.URL)
		doc := assertCleanMachineOutput(t, result, 1, "drop-secret")
		if doc["error"] == "" || api.count() != 1 {
			t.Fatalf("doc=%v requests=%d", doc, api.count())
		}
		if strings.Contains(result.stderr, "EOF") || strings.Contains(result.stderr, "connection reset") {
			t.Fatalf("raw transport detail leaked: stdout=%q stderr=%q", result.stdout, result.stderr)
		}
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
			if !strings.HasPrefix(result.stderr, "Error: ") || doc["error"] == "" {
				t.Fatalf("standard usage error missing: %q", result.stderr)
			}
		})
	}
}

func TestBlackBoxInterruptBlockedAsk(t *testing.T) {
	api := newFakeAPI(t, func(_ http.ResponseWriter, r *http.Request, _ int) { <-r.Context().Done() })
	tmp := t.TempDir()
	requestPath := filepath.Join(tmp, "request.json")
	if err := os.WriteFile(requestPath, fixture(t, "request_full.json"), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(jeqBin, "ask", "--request", requestPath, "--base-url", api.server.URL, "--timeout", "10s")
	cmd.Dir = repoRoot
	cmd.Env = mergedEnv(map[string]string{"TYPESAFE_API_KEY": "interrupt-secret"})
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-api.firstReq:
	case <-time.After(2 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatal("blocked ask did not reach fake server")
	}
	if err := cmd.Process.Signal(os.Interrupt); err != nil {
		t.Fatal(err)
	}
	wait := make(chan error, 1)
	go func() { wait <- cmd.Wait() }()
	select {
	case err := <-wait:
		if code := exitCode(cmd, err); code != 130 {
			t.Fatalf("interrupt exit=%d want=130 stdout=%q stderr=%q err=%v", code, stdout.String(), stderr.String(), err)
		}
		if stdout.Len() != 0 || !strings.Contains(stderr.String(), "JEQ_INTERRUPTED") || strings.Contains(stderr.String(), "interrupt-secret") {
			t.Fatalf("interrupt output stdout=%q stderr=%q", stdout.String(), stderr.String())
		}
	case <-time.After(2 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatal("interrupted ask did not exit by deadline")
	}
}
