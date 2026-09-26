package cli_test

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
	require.NoError(t, os.MkdirAll(filepath.Join(home, ".config", "jeq"), 0o700))
	questions := filepath.Join(root, "questions.json")
	require.NoError(t, os.WriteFile(questions, []byte(validationQuestions), 0o600))
	return home, questions
}

func TestValidateHelpExplainsModelResolutionAndProviderSupport(t *testing.T) {
	command := cli.NewValidateCmd(cli.AskDeps{})
	var help bytes.Buffer
	command.SetOut(&help)
	command.SetErr(&help)
	require.NoError(t, command.Help())
	for _, phrase := range []string{"--model, JEQ_DEFAULT_MODEL", "config.default_model", "TYPESAFE_DEFAULT_MODEL", "without authentication or network access", "Passing validation does not show", "provider supports the", "rather than passing --model"} {
		assert.Contains(t, help.String(), phrase, "validate help")
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
				require.NoError(t, os.WriteFile(configPath, []byte(tc.config), 0o600))
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
			require.Equal(t, 0, askCode, "stderr=%q", askStderr)
			require.Equal(t, 1, client.call)
			assert.Equal(t, tc.wantModel, client.request.Model, "response model=%q", client.resp.Model)

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
			credentialReads := []string{}
			validateDeps.Getenv = func(key string) string {
				if key == "TYPESAFE_API_KEY" || key == "ACME_API_KEY" || key == "AI_GATEWAY_API_KEY" || key == "VERCEL_OIDC_TOKEN" || key == "JEQ_API_KEY" {
					credentialReads = append(credentialReads, key)
				}
				return validateValues[key]
			}
			validateArgs := []string{"validate", "--questions", questions, "--state", "STATE_SECRET"}
			if tc.flagModel != "" {
				validateArgs = append(validateArgs, "--model", tc.flagModel)
			}
			code := cli.RunWithDeps(validateArgs, &out, &stderr, &askRenderer{}, validateDeps)
			assert.Equal(t, 0, code, "created=%v calls=%d stderr=%q", created, client.call, stderr.String())
			assert.False(t, created)
			assert.Zero(t, client.call)
			assert.Empty(t, credentialReads)
			assert.Contains(t, out.String(), "model: "+tc.wantModel+"\n")
			assert.Contains(t, out.String(), "model_source: "+tc.wantSource+"\n")
			for _, secret := range []string{"STATE_SECRET", "API_SECRET", "GATEWAY_SECRET", "ACME_API_KEY"} {
				assert.NotContains(t, out.String(), secret, "receipt disclosed a secret")
			}
			assert.Equal(t, tc.wantModel, client.request.Model, "ask request and validation receipt differ")
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
	require.NoError(t, err)
	return contents
}

func TestValidateUsesFreshEnvironmentAndConfigOnEachInvocation(t *testing.T) {
	homeA, questionsA := validationFixture(t)
	homeB, questionsB := validationFixture(t)
	configA := filepath.Join(homeA, ".config", "jeq", "config.json")
	configB := filepath.Join(homeB, ".config", "jeq", "config.json")
	for path, model := range map[string]string{configA: "config-a", configB: "config-b"} {
		require.NoError(t, os.WriteFile(path, []byte(`{"default_model":"`+model+`"}`), 0o600))
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
		assert.Equal(t, 0, code, "stderr=%q", stderr.String())
		assert.Contains(t, out.String(), "model: "+tc.wantModel+"\n")
		assert.Contains(t, out.String(), "model_source: "+tc.wantSource+"\n")
	}
	homeAfterA, homeAfterB := snapshotHome(t, homeA), snapshotHome(t, homeB)
	assert.Equal(t, homeBeforeA, homeAfterA, "validation changed home A")
	assert.Equal(t, homeBeforeB, homeAfterB, "validation changed home B")
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
				require.NoError(t, os.WriteFile(configPath, []byte(tc.contents), 0o600))
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
					assert.Equal(t, 2, code, "stderr=%q", stderr.String())
					assert.Contains(t, stderr.String(), "JEQ_INPUT_INVALID")
					assert.Zero(t, credentialReads)
					assert.Zero(t, clientCreates)
					assert.Zero(t, client.call)
				})
			}
		})
	}
}

func TestNativeRequestModelIsAuthoritativeAndExplicitModelConflictsBeforeIO(t *testing.T) {
	home, _ := validationFixture(t)
	request := filepath.Join(t.TempDir(), "native.json")
	require.NoError(t, os.WriteFile(request, []byte(`{"model":"native-model","state":{"s":"STATE_SECRET"},"questions":{"q":{"type":"noul","instructions":"i"}}}`), 0o600))
	client := &fakeClient{resp: contract.Response{Model: "response-model"}}
	askValues := map[string]string{"JEQ_DEFAULT_MODEL": "composed-model", "TYPESAFE_API_KEY": "API_SECRET"}
	deps := resolutionDeps(t, home, askValues, client)
	modelConfigReads := 0
	deps.Getenv = func(key string) string {
		if key == "JEQ_DEFAULT_MODEL" {
			modelConfigReads++
		}
		return askValues[key]
	}
	code, renderer, out, stderr := runAsk(t, []string{"ask", "--request", request}, deps)
	require.Equal(t, 0, code, "stderr=%q", stderr)
	assert.Equal(t, 1, client.call)
	assert.Equal(t, "native-model", client.request.Model)
	assert.True(t, renderer.response, "native ask did not evaluate")
	assert.Zero(t, modelConfigReads)

	var validateOut, validateErr bytes.Buffer
	validateDeps := resolutionDeps(t, home, map[string]string{"JEQ_DEFAULT_MODEL": "composed-model"}, client)
	validateEnvReads := 0
	validateDeps.Getenv = func(string) string {
		validateEnvReads++
		return ""
	}
	code = cli.RunWithDeps([]string{"validate", "--request", request}, &validateOut, &validateErr, &askRenderer{}, validateDeps)
	assert.Equal(t, 0, code, "stderr=%q", validateErr.String())
	assert.Contains(t, validateOut.String(), "model: native-model\n")
	assert.Contains(t, validateOut.String(), "model_source: native\n")
	assert.Zero(t, validateEnvReads)
	assert.NotContains(t, out, "STATE_SECRET")
	assert.NotContains(t, validateOut.String(), "STATE_SECRET")

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
			assert.Equal(t, 2, code)
			assert.Zero(t, reads)
			assert.Zero(t, envReads)
			assert.Zero(t, clients)
			assert.Contains(t, stderr.String(), "edit the model in the request document")
		})
	}
}

func TestValidateExamplesDiscoveryDoesNotResolveConfigOrCreateClient(t *testing.T) {
	home, _ := validationFixture(t)
	require.NoError(t, os.WriteFile(filepath.Join(home, ".config", "jeq", "config.json"), []byte(`{"default_model":"configured-model"}`), 0o600))
	before := snapshotHome(t, home)
	optionalReads := 0
	envReads, fileReads, stdinReads, clientCreates := 0, 0, 0, 0
	deps := cli.AskDeps{
		Getenv: func(string) string { envReads++; return "" },
		ReadFile: func(string, int64) ([]byte, *jeq.Error) {
			fileReads++
			return nil, nil
		},
		ReadStdin: func(io.Reader, int64, bool) ([]byte, *jeq.Error) {
			stdinReads++
			return nil, nil
		},
		ReadOptionalFile: func(string, int64) ([]byte, *jeq.Error, bool) {
			optionalReads++
			return nil, nil, false
		},
		NewClient: func(string, time.Duration, string, int, func(string)) cli.APIClient {
			clientCreates++
			return nil
		},
	}
	var stdout, stderr bytes.Buffer
	code := cli.RunWithDeps([]string{"examples", "validate"}, &stdout, &stderr, &askRenderer{}, deps)
	assert.Equal(t, 0, code)
	assert.Contains(t, stdout.String(), "jeq validate")
	assert.Empty(t, stderr.String())
	assert.Zero(t, envReads)
	assert.Zero(t, fileReads)
	assert.Zero(t, stdinReads)
	assert.Zero(t, optionalReads)
	assert.Zero(t, clientCreates)
	assert.Equal(t, before, snapshotHome(t, home))
}
