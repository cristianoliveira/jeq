package cli_test

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/cristianoliveira/jeq/internal/cli"
	"github.com/cristianoliveira/jeq/internal/domain/contract"
	"github.com/cristianoliveira/jeq/internal/domain/jeq"
)

const validationQuestions = `{"questions":{"q":{"type":"noul","instructions":"Is this valid?"}}}`

func resolutionDeps(t *testing.T, home string, values map[string]string, client *fakeClient) cli.AskDeps {
	t.Helper()
	getenv := func(key string) string { return values[key] }
	readFile := func(path string, limit int64) ([]byte, *jeq.Error) {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, jeq.NewError(jeq.CodeInputInvalid, "cannot read test input")
		}
		if int64(len(data)) > limit {
			return nil, jeq.NewError(jeq.CodeInputInvalid, "test input too large")
		}
		return data, nil
	}
	readOptional := func(path string, limit int64) ([]byte, *jeq.Error, bool) {
		data, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			return nil, nil, false
		}
		if err != nil {
			return nil, jeq.NewError(jeq.CodeInputInvalid, "cannot read test config"), true
		}
		if int64(len(data)) > limit {
			return nil, jeq.NewError(jeq.CodeInputInvalid, "test config too large"), true
		}
		return data, nil, true
	}
	deps := cli.AskDeps{
		ReadFile:         readFile,
		ReadOptionalFile: readOptional,
		ReadStdin: func(io.Reader, int64, bool) ([]byte, *jeq.Error) {
			return nil, jeq.NewError(jeq.CodeInputInvalid, "unexpected stdin")
		},
		Getenv: getenv,
		Stdin:  strings.NewReader(""),
		NewClient: func(string, time.Duration, string, int, func(string)) cli.APIClient {
			return client
		},
	}
	values["HOME"] = home
	return deps
}

func validationFixture(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	home := filepath.Join(root, "home")
	if err := os.MkdirAll(filepath.Join(home, ".config", "jeq"), 0o700); err != nil {
		t.Fatal(err)
	}
	questions := filepath.Join(root, "questions.json")
	if err := os.WriteFile(questions, []byte(validationQuestions), 0o600); err != nil {
		t.Fatal(err)
	}
	return home, questions
}

func TestValidateHelpExplainsModelResolutionAndProviderSupport(t *testing.T) {
	command := cli.NewValidateCmd(cli.AskDeps{})
	var help bytes.Buffer
	command.SetOut(&help)
	command.SetErr(&help)
	if err := command.Help(); err != nil {
		t.Fatal(err)
	}
	for _, phrase := range []string{"--model, JEQ_DEFAULT_MODEL", "config.default_model", "TYPESAFE_DEFAULT_MODEL", "without authentication or network access", "Passing validation does not show", "provider supports the", "rather than passing --model"} {
		if !strings.Contains(help.String(), phrase) {
			t.Errorf("validate help missing %q: %s", phrase, help.String())
		}
	}
}

func TestValidateResolvesTheSameOutgoingModelAsAsk(t *testing.T) {
	cases := []struct {
		name, flagModel, config, wantModel, wantSource string
		env                                            map[string]string
	}{
		{
			name:      "explicit flag",
			flagModel: "flag-model", config: `{"default_model":"config-model"}`,
			env: map[string]string{"JEQ_DEFAULT_MODEL": "env-model"}, wantModel: "flag-model", wantSource: "flag",
		},
		{
			name:   "JEQ_DEFAULT_MODEL",
			config: `{"default_model":"config-model"}`,
			env:    map[string]string{"JEQ_DEFAULT_MODEL": "env-model"}, wantModel: "env-model", wantSource: "environment",
		},
		{
			name:   "named provider default",
			config: `{"providers":{"acme":{"base_url":"http://127.0.0.1:8080","default_model":"acme-model","auth":"none"}}}`,
			env:    map[string]string{"JEQ_PROVIDER": "acme"}, wantModel: "acme-model", wantSource: "provider",
		},
		{
			name:      "built-in provider default",
			env:       map[string]string{"JEQ_PROVIDER": "vercel", "AI_GATEWAY_API_KEY": "GATEWAY_SECRET"},
			wantModel: "typesafe-ai/jev", wantSource: "provider",
		},
		{
			name:      "legacy config default",
			config:    `{"default_model":"legacy-config-model"}`,
			wantModel: "legacy-config-model", wantSource: "config",
		},
		{
			name: "legacy environment default",
			env:  map[string]string{"TYPESAFE_DEFAULT_MODEL": "legacy-env-model"}, wantModel: "legacy-env-model", wantSource: "environment",
		},
		{
			name:      "fallback",
			wantModel: cli.DefaultModel, wantSource: "default",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			home, questions := validationFixture(t)
			configPath := filepath.Join(home, ".config", "jeq", "config.json")
			if tc.config != "" {
				if err := os.WriteFile(configPath, []byte(tc.config), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			values := map[string]string{}
			for key, value := range tc.env {
				values[key] = value
			}
			values["TYPESAFE_API_KEY"] = "API_SECRET"
			client := &fakeClient{resp: contract.Response{Model: "response-version-not-request"}}
			deps := resolutionDeps(t, home, values, client)

			askArgs := []string{"ask", "--questions", questions, "--state", "STATE_SECRET"}
			if tc.flagModel != "" {
				askArgs = append(askArgs, "--model", tc.flagModel)
			}
			askCode, _, _, askStderr := runAsk(t, askArgs, deps)
			if askCode != 0 || client.call != 1 {
				t.Fatalf("ask code=%d calls=%d stderr=%q", askCode, client.call, askStderr)
			}
			if client.request.Model != tc.wantModel {
				t.Fatalf("ask request model=%q want=%q (response model=%q)", client.request.Model, tc.wantModel, client.resp.Model)
			}

			client.call = 0
			var out, stderr bytes.Buffer
			validateValues := map[string]string{}
			for key, value := range values {
				if key != "TYPESAFE_API_KEY" && key != "AI_GATEWAY_API_KEY" {
					validateValues[key] = value
				}
			}
			validateValues["HOME"] = home
			validateDeps := resolutionDeps(t, home, validateValues, client)
			created := false
			validateDeps.NewClient = func(string, time.Duration, string, int, func(string)) cli.APIClient {
				created = true
				return client
			}
			validateDeps.Getenv = func(key string) string {
				if key == "TYPESAFE_API_KEY" || key == "ACME_API_KEY" || key == "AI_GATEWAY_API_KEY" || key == "VERCEL_OIDC_TOKEN" || key == "JEQ_API_KEY" {
					t.Fatalf("validate read credential %s", key)
				}
				return validateValues[key]
			}
			validateArgs := []string{"validate", "--questions", questions, "--state", "STATE_SECRET"}
			if tc.flagModel != "" {
				validateArgs = append(validateArgs, "--model", tc.flagModel)
			}
			code := cli.RunWithDeps(validateArgs, &out, &stderr, &askRenderer{}, validateDeps)
			if code != 0 || created || client.call != 0 {
				t.Fatalf("validate code=%d created=%v calls=%d stderr=%q", code, created, client.call, stderr.String())
			}
			if !strings.Contains(out.String(), "model: "+tc.wantModel+"\n") || !strings.Contains(out.String(), "model_source: "+tc.wantSource+"\n") {
				t.Fatalf("receipt=%q, want model=%q source=%q", out.String(), tc.wantModel, tc.wantSource)
			}
			for _, secret := range []string{"STATE_SECRET", "API_SECRET", "GATEWAY_SECRET", "ACME_API_KEY"} {
				if strings.Contains(out.String(), secret) {
					t.Errorf("receipt disclosed %q: %q", secret, out.String())
				}
			}
			if client.request.Model != tc.wantModel {
				t.Errorf("ask request model=%q differs from validation receipt %q", client.request.Model, tc.wantModel)
			}
		})
	}
}

func snapshotHome(t *testing.T, home string) map[string]string {
	t.Helper()
	contents := map[string]string{}
	err := filepath.WalkDir(home, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(home, path)
		if err != nil {
			return err
		}
		if entry.IsDir() {
			contents[relative] = "directory"
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		contents[relative] = string(data)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return contents
}

func TestValidateUsesFreshEnvironmentAndConfigOnEachInvocation(t *testing.T) {
	homeA, questionsA := validationFixture(t)
	homeB, questionsB := validationFixture(t)
	configA := filepath.Join(homeA, ".config", "jeq", "config.json")
	configB := filepath.Join(homeB, ".config", "jeq", "config.json")
	for path, model := range map[string]string{configA: "config-a", configB: "config-b"} {
		if err := os.WriteFile(path, []byte(`{"default_model":"`+model+`"}`), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	homeBeforeA := snapshotHome(t, homeA)
	homeBeforeB := snapshotHome(t, homeB)

	for _, tc := range []struct {
		home, questions, envModel, wantModel, wantSource string
	}{
		{homeA, questionsA, "env-a", "env-a", "environment"},
		{homeB, questionsB, "", "config-b", "config"},
	} {
		values := map[string]string{"JEQ_DEFAULT_MODEL": tc.envModel}
		deps := resolutionDeps(t, tc.home, values, &fakeClient{})
		var out, stderr bytes.Buffer
		code := cli.RunWithDeps([]string{"validate", "--questions", tc.questions, "--state", "state"}, &out, &stderr, &askRenderer{}, deps)
		if code != 0 || !strings.Contains(out.String(), "model: "+tc.wantModel+"\n") || !strings.Contains(out.String(), "model_source: "+tc.wantSource+"\n") {
			t.Fatalf("code=%d out=%q stderr=%q", code, out.String(), stderr.String())
		}
	}
	homeAfterA, homeAfterB := snapshotHome(t, homeA), snapshotHome(t, homeB)
	if !reflect.DeepEqual(homeBeforeA, homeAfterA) || !reflect.DeepEqual(homeBeforeB, homeAfterB) {
		t.Fatalf("validation changed HOME contents: before A=%v B=%v; after A=%v B=%v", homeBeforeA, homeBeforeB, homeAfterA, homeAfterB)
	}
}

func TestMalformedOrMissingExplicitConfigFailsBeforeAuthentication(t *testing.T) {
	for _, tc := range []struct {
		name, contents string
		missing        bool
	}{
		{name: "missing explicit file", missing: true},
		{name: "malformed document", contents: `{"default_model":`},
		{name: "invalid provider profile", contents: `{"default_provider":"acme","providers":{"acme":{"base_url":"http://example.test","default_model":"m","auth":"none"}}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home, questions := validationFixture(t)
			configPath := filepath.Join(home, "explicit.json")
			if !tc.missing {
				if err := os.WriteFile(configPath, []byte(tc.contents), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			for _, command := range []string{"ask", "validate"} {
				t.Run(command, func(t *testing.T) {
					values := map[string]string{"JEQ_CONFIG": configPath, "TYPESAFE_API_KEY": "never-read"}
					client := &fakeClient{}
					deps := resolutionDeps(t, home, values, client)
					credentialReads, clientCreates := 0, 0
					deps.Getenv = func(key string) string {
						if key == "TYPESAFE_API_KEY" {
							credentialReads++
							t.Fatal("credential read before invalid config rejection")
						}
						return values[key]
					}
					deps.NewClient = func(string, time.Duration, string, int, func(string)) cli.APIClient {
						clientCreates++
						return client
					}
					args := []string{command, "--questions", questions, "--state", "state", "--model", "override"}
					var stdout, stderr bytes.Buffer
					code := cli.RunWithDeps(args, &stdout, &stderr, &askRenderer{}, deps)
					if code != 2 || !strings.Contains(stderr.String(), "JEQ_INPUT_INVALID") || credentialReads != 0 || clientCreates != 0 || client.call != 0 {
						t.Fatalf("code=%d credentials=%d clients=%d evals=%d stderr=%q", code, credentialReads, clientCreates, client.call, stderr.String())
					}
				})
			}
		})
	}
}

func TestNativeRequestModelIsAuthoritativeAndExplicitModelConflictsBeforeIO(t *testing.T) {
	home, _ := validationFixture(t)
	request := filepath.Join(t.TempDir(), "native.json")
	if err := os.WriteFile(request, []byte(`{"model":"native-model","state":{"s":"STATE_SECRET"},"questions":{"q":{"type":"noul","instructions":"i"}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	client := &fakeClient{resp: contract.Response{Model: "response-model"}}
	askValues := map[string]string{"JEQ_DEFAULT_MODEL": "composed-model", "TYPESAFE_API_KEY": "API_SECRET"}
	deps := resolutionDeps(t, home, askValues, client)
	deps.Getenv = func(key string) string {
		if key == "JEQ_DEFAULT_MODEL" {
			t.Fatal("native ask consulted composed-model configuration")
		}
		return askValues[key]
	}
	code, renderer, out, stderr := runAsk(t, []string{"ask", "--request", request}, deps)
	if code != 0 || client.call != 1 || client.request.Model != "native-model" {
		t.Fatalf("ask code=%d calls=%d model=%q stderr=%q", code, client.call, client.request.Model, stderr)
	}
	if !renderer.response {
		t.Fatal("native ask did not evaluate")
	}

	var validateOut, validateErr bytes.Buffer
	validateDeps := resolutionDeps(t, home, map[string]string{"JEQ_DEFAULT_MODEL": "composed-model"}, client)
	validateDeps.Getenv = func(string) string {
		t.Fatal("native validation consulted composed-model configuration")
		return ""
	}
	code = cli.RunWithDeps([]string{"validate", "--request", request}, &validateOut, &validateErr, &askRenderer{}, validateDeps)
	if code != 0 || !strings.Contains(validateOut.String(), "model: native-model\n") || !strings.Contains(validateOut.String(), "model_source: native\n") {
		t.Fatalf("native validation code=%d output=%q stderr=%q", code, validateOut.String(), validateErr.String())
	}
	if strings.Contains(out, "STATE_SECRET") || strings.Contains(validateOut.String(), "STATE_SECRET") {
		t.Fatalf("receipt disclosed raw state: ask=%q validate=%q", out, validateOut.String())
	}

	for _, command := range []string{"ask", "validate"} {
		t.Run(command+" conflict", func(t *testing.T) {
			reads, envReads, clients := 0, 0, 0
			conflictDeps := cli.AskDeps{
				ReadFile:  func(string, int64) ([]byte, *jeq.Error) { reads++; return nil, nil },
				ReadStdin: func(io.Reader, int64, bool) ([]byte, *jeq.Error) { reads++; return nil, nil },
				Getenv:    func(string) string { envReads++; return "" },
				NewClient: func(string, time.Duration, string, int, func(string)) cli.APIClient { clients++; return client },
			}
			args := []string{command, "--request", request, "--model", "override"}
			var stdout, stderr bytes.Buffer
			code := cli.RunWithDeps(args, &stdout, &stderr, &askRenderer{}, conflictDeps)
			if code != 2 || reads != 0 || envReads != 0 || clients != 0 || !strings.Contains(stderr.String(), "edit the model in the request document") {
				t.Fatalf("code=%d reads=%d env=%d clients=%d stderr=%q", code, reads, envReads, clients, stderr.String())
			}
		})
	}
}

func TestValidateExamplesDiscoveryDoesNotResolveConfigOrCreateClient(t *testing.T) {
	home, _ := validationFixture(t)
	if err := os.WriteFile(filepath.Join(home, ".config", "jeq", "config.json"), []byte(`{"default_model":"configured-model"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	before := snapshotHome(t, home)
	optionalReads := 0
	deps := cli.AskDeps{
		Getenv:   func(string) string { t.Fatal("example discovery read environment"); return "" },
		ReadFile: func(string, int64) ([]byte, *jeq.Error) { t.Fatal("example discovery read a file"); return nil, nil },
		ReadStdin: func(io.Reader, int64, bool) ([]byte, *jeq.Error) {
			t.Fatal("example discovery read stdin")
			return nil, nil
		},
		ReadOptionalFile: func(string, int64) ([]byte, *jeq.Error, bool) {
			optionalReads++
			return nil, nil, false
		},
		NewClient: func(string, time.Duration, string, int, func(string)) cli.APIClient {
			t.Fatal("example discovery created a client")
			return nil
		},
	}
	var stdout, stderr bytes.Buffer
	code := cli.RunWithDeps([]string{"examples", "validate"}, &stdout, &stderr, &askRenderer{}, deps)
	if code != 0 || !strings.Contains(stdout.String(), "jeq validate") || stderr.Len() != 0 {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if optionalReads != 0 || !reflect.DeepEqual(before, snapshotHome(t, home)) {
		t.Fatalf("discovery read config %d times or changed HOME", optionalReads)
	}
}
