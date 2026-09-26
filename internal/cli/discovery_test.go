package cli_test

import (
	"bytes"
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

type valueRenderer struct {
	values []any
	errors []*jeq.Error
}

func (r *valueRenderer) RenderSuccess(io.Writer, contract.Response) error { return nil }
func (r *valueRenderer) RenderError(_ io.Writer, e *jeq.Error) error {
	r.errors = append(r.errors, e)
	return nil
}

func (r *valueRenderer) RenderValue(w io.Writer, value any) error {
	r.values = append(r.values, value)
	_, err := io.WriteString(w, "value\n")
	return err
}

func TestBareRootUsesCobraHelpWithoutDependencyWork(t *testing.T) {
	r := &valueRenderer{}
	var envReads, clientCreates int
	deps := cli.AskDeps{
		Getenv:   func(string) string { envReads++; return "secret" },
		Renderer: r,
		NewClient: func(string, time.Duration, string, int, func(string)) cli.APIClient {
			clientCreates++
			return nil
		},
	}
	var bareOut, helpOut, errOut bytes.Buffer
	require.Equal(t, 0, cli.RunWithDeps(nil, &bareOut, &errOut, r, deps), "bare stderr=%q", errOut.String())
	require.Equal(t, 0, cli.RunWithDeps([]string{"--help"}, &helpOut, &errOut, r, deps), "help stderr=%q", errOut.String())
	assert.Equal(t, bareOut.String(), helpOut.String())
	assert.Zero(t, envReads)
	assert.Zero(t, clientCreates)
	assert.Empty(t, errOut.String())
	assert.Empty(t, r.values)
	assert.Contains(t, bareOut.String(), "Usage:")
	assert.Contains(t, bareOut.String(), "Available Commands:")
	assert.Contains(t, bareOut.String(), "examples")
	assert.NotContains(t, bareOut.String(), "Credentials:")
}

func TestVersionUsesInjectedBuildValuesAndModelsUseAuth(t *testing.T) {
	oldVersion, oldCommit := cli.Version, cli.Commit
	defer func() { cli.Version, cli.Commit = oldVersion, oldCommit }()
	cli.Version, cli.Commit = "v-test", "abc123"
	r := &valueRenderer{}
	client := &fakeClient{}
	var gotURL string
	deps := cli.AskDeps{
		Getenv: func(key string) string {
			switch key {
			case "TYPESAFE_API_KEY":
				return "secret"
			case "TYPESAFE_BASE_URL":
				return "https://env-url"
			}
			return ""
		},
		NewClient: func(url string, _ time.Duration, _ string, _ int, _ func(string)) cli.APIClient {
			gotURL = url
			return client
		},
		Renderer: r,
	}
	var out, errOut bytes.Buffer
	versionCode := cli.RunWithDeps([]string{"version"}, &out, &errOut, r, deps)
	assert.Equal(t, 0, versionCode)
	assert.Contains(t, out.String(), "jeq v-test\nCommit: abc123\n")
	assert.Empty(t, r.values)
	modelsCode := cli.RunWithDeps([]string{"models"}, &out, &errOut, r, deps)
	assert.Equal(t, 0, modelsCode)
	assert.Equal(t, "https://env-url", gotURL)
	assert.Zero(t, client.call)
}

func TestModelsMissingCredentialDoesNotCreateClient(t *testing.T) {
	r := &valueRenderer{}
	created := false
	deps := cli.AskDeps{
		Getenv: func(string) string { return "" },
		NewClient: func(string, time.Duration, string, int, func(string)) cli.APIClient {
			created = true
			return &fakeClient{}
		},
		Renderer: r,
	}
	var out, errOut bytes.Buffer
	code := cli.RunWithDeps([]string{"models"}, &out, &errOut, r, deps)
	assert.Equal(t, 1, code)
	assert.False(t, created)
	assert.Empty(t, r.errors)
	assert.Contains(t, errOut.String(), "JEQ_AUTH_MISSING")
}

func TestValidateConflictsAndFailuresAreUsageErrors(t *testing.T) {
	r := &valueRenderer{}
	reads := 0
	envReads := 0
	deps := cli.AskDeps{
		Getenv:    func(string) string { envReads++; return "" },
		ReadFile:  func(string, int64) ([]byte, *jeq.Error) { reads++; return nil, nil },
		ReadStdin: func(io.Reader, int64, bool) ([]byte, *jeq.Error) { reads++; return nil, nil },
		Renderer:  r,
	}
	var out, errOut bytes.Buffer
	code := cli.RunWithDeps([]string{"validate", "--request", "a", "--questions", "b"}, &out, &errOut, r, deps)
	assert.Equal(t, 2, code)
	assert.Zero(t, reads)
	assert.Zero(t, envReads)

	deps.Getenv = func(string) string { return "" }
	deps.ReadFile = func(string, int64) ([]byte, *jeq.Error) {
		reads++
		return []byte(`{"state":"s","model":"m","questions":{}}`), nil
	}
	code = cli.RunWithDeps([]string{"validate", "--request", "bad.json"}, &out, &errOut, r, deps)
	assert.Equal(t, 2, code)
	assert.Equal(t, 1, reads)
}

func TestValidateNeverCreatesClientAndDoesNotExposeState(t *testing.T) {
	r := &valueRenderer{}
	created := false
	apiKeyReads := 0
	stdinReads := 0
	deps := cli.AskDeps{
		Getenv: func(key string) string {
			if key == "TYPESAFE_DEFAULT_MODEL" {
				return "env-model"
			}
			if key == "TYPESAFE_API_KEY" {
				apiKeyReads++
			}
			return ""
		},
		ReadFile: func(string, int64) ([]byte, *jeq.Error) {
			return []byte(`{"questions":{"q":{"type":"noul","instructions":"i"}}}`), nil
		},
		ReadStdin: func(io.Reader, int64, bool) ([]byte, *jeq.Error) { stdinReads++; return nil, nil },
		NewClient: func(string, time.Duration, string, int, func(string)) cli.APIClient {
			created = true
			return &fakeClient{}
		},
		Renderer: r,
		Stdin:    strings.NewReader(""),
	}
	var out, errOut bytes.Buffer
	code := cli.RunWithDeps([]string{"validate", "--questions", "q.json", "--state", "SECRET STATE"}, &out, &errOut, r, deps)
	assert.Equal(t, 0, code)
	assert.False(t, created)
	assert.Empty(t, r.values)
	assert.Zero(t, apiKeyReads)
	assert.Zero(t, stdinReads)
	assert.NotContains(t, out.String(), "SECRET STATE")
}
