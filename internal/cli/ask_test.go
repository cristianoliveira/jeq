package cli_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/cristianoliveira/jeq/internal/cli"
	"github.com/cristianoliveira/jeq/internal/domain/contract"
	"github.com/cristianoliveira/jeq/internal/domain/jeq"
)

type askRenderer struct {
	response bool
	err      *jeq.Error
}

func (r *askRenderer) RenderSuccess(w io.Writer, resp contract.Response) error {
	r.response = true
	b, _ := resp.Encode()
	_, err := w.Write(append(b, '\n'))
	return err
}
func (r *askRenderer) RenderError(_ io.Writer, e *jeq.Error) error { r.err = e; return nil }

type fakeClient struct {
	request contract.Request
	resp    contract.Response
	err     *jeq.Error
	call    int
}

func (c *fakeClient) Evaluate(_ context.Context, req contract.Request) (contract.Response, *jeq.Error) {
	c.call++
	c.request = req
	return c.resp, c.err
}

func (c *fakeClient) Models(context.Context) (contract.Models, *jeq.Error) {
	return contract.Models{Models: []contract.ModelInfo{{Name: "jev-latest"}, {Name: "jev-preview"}}}, c.err
}

func testDeps(t *testing.T, client *fakeClient, getenv func(string) string) (cli.AskDeps, *bytes.Buffer, *int) {
	t.Helper()
	var stdin bytes.Buffer
	var reads int
	deps := cli.AskDeps{
		ReadFile: func(_ string, _ int64) ([]byte, *jeq.Error) {
			reads++
			return []byte(`{"questions":{"q":{"type":"noul","instructions":"i"}}}`), nil
		},
		ReadStdin: func(_ io.Reader, _ int64, _ bool) ([]byte, *jeq.Error) {
			reads++
			return nil, jeq.NewError(jeq.CodeInputInvalid, "unexpected stdin")
		},
		NewClient: func(_ string, _ time.Duration, _ string, _ int, diagnostic func(string)) cli.APIClient {
			_ = diagnostic
			return client
		},
		Getenv: getenv,
		Stdin:  &stdin,
	}
	return deps, &stdin, &reads
}

func runAsk(t *testing.T, args []string, deps cli.AskDeps) (int, *askRenderer, string, string) {
	t.Helper()
	var out, stderr bytes.Buffer
	r := &askRenderer{}
	code := cli.RunWithDeps(args, &out, &stderr, r, deps)
	return code, r, out.String(), stderr.String()
}

func TestAskNativeHappyPath(t *testing.T) {
	client := &fakeClient{resp: contract.Response{Model: "jev-1.13.0", Answers: map[string]contract.Answer{}, Usage: contract.Usage{}}}
	deps, _, reads := testDeps(t, client, func(key string) string {
		if key == "TYPESAFE_API_KEY" {
			return "secret"
		}
		if key == "TYPESAFE_BASE_URL" {
			return "http://fixture"
		}
		return ""
	})
	deps.ReadFile = func(path string, _ int64) ([]byte, *jeq.Error) {
		*reads++
		if path != "native.json" {
			t.Errorf("path = %q", path)
		}
		return []byte(`{"state":"hello","model":"native","questions":{"q":{"type":"noul","instructions":"i"}},"x_unknown":{"a":1}}`), nil
	}

	code, renderer, stdout, stderr := runAsk(t, []string{"ask", "--request", "native.json"}, deps)
	if code != 0 || !renderer.response || renderer.err != nil {
		t.Fatalf("code=%d success=%v err=%v", code, renderer.response, renderer.err)
	}
	if stderr != "" || strings.Count(stdout, "\n") != 1 {
		t.Errorf("stdout=%q stderr=%q", stdout, stderr)
	}
	if client.call != 1 || client.request.Model != "native" || client.request.Extra["x_unknown"] == nil {
		t.Errorf("request not passed through: %+v calls=%d", client.request, client.call)
	}
}

func TestAskComposedHappyPathAndPrecedence(t *testing.T) {
	client := &fakeClient{resp: contract.Response{Model: "m", Answers: map[string]contract.Answer{}, Usage: contract.Usage{}}}
	deps, _, _ := testDeps(t, client, func(key string) string {
		switch key {
		case "TYPESAFE_API_KEY":
			return "secret"
		case "TYPESAFE_DEFAULT_MODEL":
			return "env-model"
		}
		return ""
	})
	code, _, _, _ := runAsk(t, []string{"ask", "--questions", "q.json", "--state", "state", "--model", "flag-model"}, deps)
	if code != 0 {
		t.Fatalf("exit = %d", code)
	}
	if client.request.Model != "flag-model" || string(client.request.State) != `"state"` {
		t.Errorf("request = %+v", client.request)
	}
}

func TestAskConflictHappensBeforeReadersAndEnvironment(t *testing.T) {
	client := &fakeClient{}
	deps, _, reads := testDeps(t, client, func(string) string { t.Fatal("environment read before conflict"); return "" })
	code, renderer, _, stderr := runAsk(t, []string{"ask", "--request", "a", "--questions", "b"}, deps)
	if code != 2 || renderer.err != nil || *reads != 0 || client.call != 0 || !strings.Contains(stderr, "JEQ_SOURCE_CONFLICT") {
		t.Fatalf("code=%d err=%v reads=%d calls=%d stderr=%q", code, renderer.err, *reads, client.call, stderr)
	}
}

func TestAskRejectsDualStdinBeforeRead(t *testing.T) {
	client := &fakeClient{}
	deps, _, reads := testDeps(t, client, func(string) string { t.Fatal("environment read before conflict"); return "" })
	code, renderer, _, stderr := runAsk(t, []string{"ask", "--questions", "-", "--state-json", "-"}, deps)
	if code != 2 || renderer.err != nil || !strings.Contains(stderr, "JEQ_SOURCE_CONFLICT") || *reads != 0 {
		t.Fatalf("code=%d err=%v reads=%d", code, renderer.err, *reads)
	}
}

func TestAskMissingKeyDoesNotCreateClient(t *testing.T) {
	client := &fakeClient{}
	deps, _, _ := testDeps(t, client, func(string) string { return "" })
	created := false
	deps.NewClient = func(string, time.Duration, string, int, func(string)) cli.APIClient { created = true; return client }
	code, renderer, _, stderr := runAsk(t, []string{"ask", "--questions", "q", "--state", "s"}, deps)
	if code != 1 || renderer.err != nil || !strings.Contains(stderr, "JEQ_AUTH_MISSING") || created {
		t.Fatalf("code=%d err=%v created=%v", code, renderer.err, created)
	}
}

func TestAskMaxRetriesAndInputConfigErrors(t *testing.T) {
	for _, args := range [][]string{{"--max-retries", "6"}, {"--max-retries", "-1"}, {"--timeout", "nope"}} {
		t.Run(strings.Join(args, "-"), func(t *testing.T) {
			client := &fakeClient{}
			deps, _, _ := testDeps(t, client, func(string) string { return "secret" })
			code, renderer, _, stderr := runAsk(t, append([]string{"ask", "--questions", "q", "--state", "s"}, args...), deps)
			if code != 2 || renderer.err != nil || !strings.Contains(stderr, "Error:") {
				t.Fatalf("code=%d err=%v", code, renderer.err)
			}
		})
	}
}

func TestAskPassesBaseURLTimeoutAndRetryConfiguration(t *testing.T) {
	client := &fakeClient{resp: contract.Response{Model: "m", Answers: map[string]contract.Answer{}, Usage: contract.Usage{}}}
	deps, _, _ := testDeps(t, client, func(key string) string {
		if key == "TYPESAFE_API_KEY" {
			return "secret"
		}
		if key == "TYPESAFE_BASE_URL" {
			return "env-url"
		}
		return ""
	})
	var gotURL string
	var gotTimeout time.Duration
	var gotRetries int
	deps.NewClient = func(baseURL string, timeout time.Duration, _ string, retries int, _ func(string)) cli.APIClient {
		gotURL, gotTimeout, gotRetries = baseURL, timeout, retries
		return client
	}
	code, _, _, _ := runAsk(t, []string{"ask", "--questions", "q", "--state", "s", "--base-url", "flag-url", "--timeout", "3s", "--max-retries", "5"}, deps)
	if code != 0 || gotURL != "flag-url" || gotTimeout != 3*time.Second || gotRetries != 5 {
		t.Fatalf("code=%d url=%q timeout=%s retries=%d", code, gotURL, gotTimeout, gotRetries)
	}
}

func TestAskReadsExplicitStdinOnce(t *testing.T) {
	client := &fakeClient{resp: contract.Response{Model: "m", Answers: map[string]contract.Answer{}, Usage: contract.Usage{}}}
	deps, _, reads := testDeps(t, client, func(key string) string {
		if key == "TYPESAFE_API_KEY" {
			return "secret"
		}
		return ""
	})
	deps.ReadStdin = func(_ io.Reader, _ int64, _ bool) ([]byte, *jeq.Error) {
		*reads++
		return []byte(`{"state":"s","model":"m","questions":{"q":{"type":"noul","instructions":"i"}}}`), nil
	}
	code, renderer, _, _ := runAsk(t, []string{"ask", "--request", "-"}, deps)
	if code != 0 || renderer.err != nil || *reads != 1 || client.call != 1 {
		t.Fatalf("code=%d err=%v reads=%d calls=%d", code, renderer.err, *reads, client.call)
	}
}

func TestAskFailureRendersStableDocumentAndRetryDiagnostics(t *testing.T) {
	client := &fakeClient{err: jeq.WrapError(jeq.CodeRateLimited, errors.New("wrapped dependency secret"), "slow")}
	deps, _, _ := testDeps(t, client, func(key string) string {
		if key == "TYPESAFE_API_KEY" {
			return "secret"
		}
		return ""
	})
	var out, errOut bytes.Buffer
	r := &askRenderer{}
	code := cli.RunWithDeps([]string{"ask", "--questions", "q", "--state", "s"}, &out, &errOut, r, deps)
	if code != 1 || r.err != nil || !strings.Contains(errOut.String(), "JEQ_RATE_LIMITED") || strings.Contains(errOut.String(), "wrapped dependency secret") || out.Len() != 0 {
		t.Fatalf("code=%d err=%v stdout=%q", code, r.err, out.String())
	}
}
