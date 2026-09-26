package cli_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cristianoliveira/jeq/internal/cli"
	"github.com/cristianoliveira/jeq/internal/domain/contract"
	"github.com/cristianoliveira/jeq/internal/domain/jeq"
)

type askRenderer struct {
	response   bool
	err        *jeq.Error
	failWriter bool
}

func (r *askRenderer) RenderSuccess(w io.Writer, resp contract.Response) error {
	r.response = true
	if r.failWriter {
		return errors.New("injected writer failure")
	}
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
			return "http://127.0.0.1:18080"
		}
		return ""
	})
	gotPath := ""
	deps.ReadFile = func(path string, _ int64) ([]byte, *jeq.Error) {
		*reads++
		gotPath = path
		return []byte(`{"state":"hello","model":"native","questions":{"q":{"type":"noul","instructions":"i"}},"x_unknown":{"a":1}}`), nil
	}

	code, renderer, stdout, stderr := runAsk(t, []string{"ask", "--request", "native.json"}, deps)
	require.Equal(t, 0, code, "stderr=%q", stderr)
	assert.True(t, renderer.response)
	assert.Nil(t, renderer.err)
	assert.Equal(t, "native.json", gotPath)
	assert.Empty(t, stderr)
	assert.Equal(t, 1, strings.Count(stdout, "\n"))
	assert.Equal(t, 1, client.call)
	assert.Equal(t, "native", client.request.Model)
	assert.NotNil(t, client.request.Extra["x_unknown"], "request not passed through")
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
	require.Equal(t, 0, code)
	assert.Equal(t, "flag-model", client.request.Model)
	assert.Equal(t, `"state"`, string(client.request.State))
}

func TestAskConflictHappensBeforeReadersAndEnvironment(t *testing.T) {
	client := &fakeClient{}
	envReads := 0
	deps, _, reads := testDeps(t, client, func(string) string { envReads++; return "" })
	code, renderer, _, stderr := runAsk(t, []string{"ask", "--request", "a", "--questions", "b"}, deps)
	assert.Equal(t, 2, code)
	assert.Nil(t, renderer.err)
	assert.Zero(t, *reads)
	assert.Zero(t, envReads)
	assert.Zero(t, client.call)
	assert.Contains(t, stderr, "JEQ_SOURCE_CONFLICT")
}

func TestAskRejectsDualStdinBeforeRead(t *testing.T) {
	client := &fakeClient{}
	envReads := 0
	deps, _, reads := testDeps(t, client, func(string) string { envReads++; return "" })
	code, renderer, _, stderr := runAsk(t, []string{"ask", "--questions", "-", "--state-json-file", "-"}, deps)
	assert.Equal(t, 2, code)
	assert.Nil(t, renderer.err)
	assert.Contains(t, stderr, "JEQ_SOURCE_CONFLICT")
	assert.Zero(t, *reads)
	assert.Zero(t, envReads)
}

func TestAskMissingKeyDoesNotCreateClient(t *testing.T) {
	client := &fakeClient{}
	deps, _, _ := testDeps(t, client, func(string) string { return "" })
	created := false
	deps.NewClient = func(string, time.Duration, string, int, func(string)) cli.APIClient { created = true; return client }
	code, renderer, _, stderr := runAsk(t, []string{"ask", "--questions", "q", "--state", "s"}, deps)
	assert.Equal(t, 1, code)
	assert.Nil(t, renderer.err)
	assert.Contains(t, stderr, "JEQ_AUTH_MISSING")
	assert.False(t, created)
}

func TestAskMaxRetriesAndInputConfigErrors(t *testing.T) {
	for _, args := range [][]string{{"--max-retries", "6"}, {"--max-retries", "-1"}, {"--timeout", "nope"}} {
		t.Run(strings.Join(args, "-"), func(t *testing.T) {
			client := &fakeClient{}
			deps, _, _ := testDeps(t, client, func(string) string { return "secret" })
			code, renderer, _, stderr := runAsk(t, append([]string{"ask", "--questions", "q", "--state", "s"}, args...), deps)
			assert.Equal(t, 2, code)
			assert.Nil(t, renderer.err)
			assert.Contains(t, stderr, "Error:")
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
			return "http://127.0.0.1:18081"
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
	code, _, _, _ := runAsk(t, []string{"ask", "--questions", "q", "--state", "s", "--timeout", "3s", "--max-retries", "5"}, deps)
	require.Equal(t, 0, code)
	assert.Equal(t, "http://127.0.0.1:18081", gotURL)
	assert.Equal(t, 3*time.Second, gotTimeout)
	assert.Equal(t, 5, gotRetries)
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
	require.Equal(t, 0, code)
	assert.Nil(t, renderer.err)
	assert.Equal(t, 1, *reads)
	assert.Equal(t, 1, client.call)
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
	assert.Equal(t, 1, code)
	assert.Nil(t, r.err)
	assert.Contains(t, errOut.String(), "JEQ_RATE_LIMITED")
	assert.NotContains(t, errOut.String(), "wrapped dependency secret")
	assert.Empty(t, out.String())
}
