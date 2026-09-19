package cli_test

import (
	"bytes"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/cristianoliveira/gev/internal/cli"
	"github.com/cristianoliveira/gev/internal/domain/contract"
	"github.com/cristianoliveira/gev/internal/domain/gev"
)

type valueRenderer struct {
	values []any
	errors []*gev.Error
}

func (r *valueRenderer) RenderSuccess(io.Writer, contract.Response) error { return nil }
func (r *valueRenderer) RenderError(_ io.Writer, e *gev.Error) error {
	r.errors = append(r.errors, e)
	return nil
}

func (r *valueRenderer) RenderValue(w io.Writer, value any) error {
	r.values = append(r.values, value)
	_, err := io.WriteString(w, "value\n")
	return err
}

func TestHomeIsOfflineAndReportsOnlyCredentialReadiness(t *testing.T) {
	r := &valueRenderer{}
	var envReads, clientCreates int
	deps := cli.AskDeps{
		Getenv: func(key string) string {
			envReads++
			if key == "TYPESAFE_API_KEY" {
				return "secret"
			}
			return "model-from-env"
		},
		Renderer:  r,
		NewClient: func(string, time.Duration, string, int, func(string)) cli.APIClient { clientCreates++; return nil },
	}
	var out, errOut bytes.Buffer
	if code := cli.RunWithDeps(nil, &out, &errOut, r, deps); code != 0 {
		t.Fatalf("exit=%d", code)
	}
	if clientCreates != 0 || errOut.Len() != 0 || len(r.values) != 0 {
		t.Fatalf("creates=%d stderr=%q renderer-values=%d", clientCreates, errOut.String(), len(r.values))
	}
	text := out.String()
	for _, want := range []string{"Credentials: ready", "Default model: model-from-env", "Next: gev examples"} {
		if !strings.Contains(text, want) {
			t.Fatalf("home output %q lacks %q; env reads=%d", text, want, envReads)
		}
	}
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
	if code := cli.RunWithDeps([]string{"version"}, &out, &errOut, r, deps); code != 0 || !strings.Contains(out.String(), "gev v-test\nCommit: abc123\n") || len(r.values) != 0 {
		t.Fatalf("version code=%d output=%q renderer-values=%d", code, out.String(), len(r.values))
	}
	if code := cli.RunWithDeps([]string{"models"}, &out, &errOut, r, deps); code != 0 || gotURL != "https://env-url" || client.call != 0 {
		t.Fatalf("models code=%d url=%q errors=%v out=%q", code, gotURL, r.errors, out.String())
	}
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
	if code := cli.RunWithDeps([]string{"models"}, &out, &errOut, r, deps); code != 1 {
		t.Fatalf("exit=%d errors=%v out=%q", code, r.errors, out.String())
	}
	if created || len(r.errors) != 0 || !strings.Contains(errOut.String(), "GEV_AUTH_MISSING") {
		t.Fatalf("created=%v errors=%v stderr=%q", created, r.errors, errOut.String())
	}
}

func TestValidateConflictsAndFailuresAreUsageErrors(t *testing.T) {
	r := &valueRenderer{}
	reads := 0
	deps := cli.AskDeps{
		Getenv:    func(string) string { t.Fatal("validate conflict read environment"); return "" },
		ReadFile:  func(string, int64) ([]byte, *gev.Error) { reads++; return nil, nil },
		ReadStdin: func(io.Reader, int64, bool) ([]byte, *gev.Error) { reads++; return nil, nil },
		Renderer:  r,
	}
	var out, errOut bytes.Buffer
	if code := cli.RunWithDeps([]string{"validate", "--request", "a", "--questions", "b"}, &out, &errOut, r, deps); code != 2 || reads != 0 {
		t.Fatalf("code=%d reads=%d errors=%v", code, reads, r.errors)
	}

	deps.Getenv = func(string) string { return "" }
	deps.ReadFile = func(string, int64) ([]byte, *gev.Error) {
		reads++
		return []byte(`{"state":"s","model":"m","questions":{}}`), nil
	}
	if code := cli.RunWithDeps([]string{"validate", "--request", "bad.json"}, &out, &errOut, r, deps); code != 2 || reads != 1 {
		t.Fatalf("invalid code=%d reads=%d errors=%v", code, reads, r.errors)
	}
}

func TestValidateNeverCreatesClientAndDoesNotExposeState(t *testing.T) {
	r := &valueRenderer{}
	created := false
	deps := cli.AskDeps{
		Getenv: func(key string) string {
			if key == "TYPESAFE_DEFAULT_MODEL" {
				return "env-model"
			}
			if key == "TYPESAFE_API_KEY" {
				t.Fatal("validate read credential")
			}
			return ""
		},
		ReadFile: func(string, int64) ([]byte, *gev.Error) {
			return []byte(`{"questions":{"q":{"type":"noul","instructions":"i"}}}`), nil
		},
		ReadStdin: func(io.Reader, int64, bool) ([]byte, *gev.Error) { t.Fatal("unexpected stdin"); return nil, nil },
		NewClient: func(string, time.Duration, string, int, func(string)) cli.APIClient {
			created = true
			return &fakeClient{}
		},
		Renderer: r,
		Stdin:    strings.NewReader(""),
	}
	var out, errOut bytes.Buffer
	if code := cli.RunWithDeps([]string{"validate", "--questions", "q.json", "--state", "SECRET STATE"}, &out, &errOut, r, deps); code != 0 || created || len(r.values) != 0 {
		t.Fatalf("code=%d created=%v renderer-values=%d", code, created, len(r.values))
	}
	if strings.Contains(out.String(), "SECRET STATE") {
		t.Fatal("validation output disclosed state")
	}
}
