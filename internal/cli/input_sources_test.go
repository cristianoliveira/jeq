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

const (
	sourceQuestions = `{"questions":{"q":{"type":"noul","instructions":"Is this urgent?"}}}`
	sourceState     = `{"ticket":{"id":7}}`
)

func inputSourceDeps(client *fakeClient, files map[string][]byte, stdin string, reads *int, getenv func(string) string) cli.AskDeps {
	return cli.AskDeps{
		ReadFile: func(path string, _ int64) ([]byte, *jeq.Error) {
			(*reads)++
			data, ok := files[path]
			if !ok {
				return nil, jeq.NewError(jeq.CodeInputInvalid, "unexpected file read")
			}
			return data, nil
		},
		ReadStdin: func(r io.Reader, limit int64, _ bool) ([]byte, *jeq.Error) {
			data, err := io.ReadAll(io.LimitReader(r, limit))
			if err != nil {
				return nil, jeq.WrapError(jeq.CodeInputInvalid, err, "reading test stdin")
			}
			return data, nil
		},
		NewClient: func(string, time.Duration, string, int, func(string)) cli.APIClient { return client },
		Getenv:    getenv,
		Stdin:     strings.NewReader(stdin),
	}
}

func TestAskEquivalentQuestionAndStateSources(t *testing.T) {
	score := 0.8
	cases := []struct {
		name  string
		args  []string
		files map[string][]byte
		stdin string
	}{
		{name: "inline", args: []string{"--questions-json", sourceQuestions, "--state-json", sourceState}},
		{name: "files", args: []string{"--questions", "questions.json", "--state-json-file", "state.json"}, files: map[string][]byte{"questions.json": []byte(sourceQuestions), "state.json": []byte(sourceState)}},
		{name: "state from stdin", args: []string{"--questions-json", sourceQuestions, "--state-json-file", "-"}, stdin: sourceState},
	}
	var expected contract.Request
	for i, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			client := &fakeClient{resp: contract.Response{Model: "m", Answers: map[string]contract.Answer{"q": {Type: contract.TypeNoul, Noul: &score}}}}
			reads := 0
			deps := inputSourceDeps(client, tc.files, tc.stdin, &reads, func(name string) string {
				if name == "TYPESAFE_API_KEY" {
					return "test-key"
				}
				return ""
			})
			args := append([]string{"ask", "--model", "m"}, tc.args...)
			code, renderer, _, stderr := runAsk(t, args, deps)
			require.Equal(t, 0, code, "stderr=%q", stderr)
			assert.Nil(t, renderer.err)
			assert.Equal(t, 1, client.call)
			if i == 0 {
				expected = client.request
				return
			}
			assert.Equal(t, expected.State, client.request.State, "request differs from inline source request")
			assert.Equal(t, expected.Model, client.request.Model)
			assert.Len(t, client.request.Questions, len(expected.Questions))
			assert.Equal(t, expected.Questions["q"].Type, client.request.Questions["q"].Type)
			assert.Equal(t, string(expected.Questions["q"].Instructions), string(client.request.Questions["q"].Instructions))
		})
	}
}

func TestValidateAcceptsInlineAndFileSourceContract(t *testing.T) {
	files := map[string][]byte{"questions.json": []byte(sourceQuestions), "state.json": []byte(sourceState)}
	cases := []struct {
		name string
		args []string
	}{
		{name: "inline", args: []string{"--questions-json", sourceQuestions, "--state-json", sourceState}},
		{name: "files", args: []string{"--questions", "questions.json", "--state-json-file", "state.json"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stdin := strings.NewReader("")
			unexpectedEnv := []string{}
			deps := cli.AskDeps{
				ReadFile: func(path string, _ int64) ([]byte, *jeq.Error) { return files[path], nil },
				ReadStdin: func(r io.Reader, limit int64, _ bool) ([]byte, *jeq.Error) {
					data, err := io.ReadAll(io.LimitReader(r, limit))
					if err != nil {
						return nil, jeq.WrapError(jeq.CodeInputInvalid, err, "reading test stdin")
					}
					return data, nil
				},
				Getenv: func(key string) string {
					switch key {
					case "JEQ_CONFIG", "JEQ_PROVIDER", "TYPESAFE_BASE_URL":
						return ""
					default:
						unexpectedEnv = append(unexpectedEnv, key)
						return ""
					}
				},
				Stdin: stdin,
			}
			args := append([]string{"validate"}, tc.args...)
			args = append(args, "--model", "m")
			var out, stderr bytes.Buffer
			code := cli.RunWithDeps(args, &out, &stderr, nil, deps)
			assert.Equal(t, 0, code, "stdout=%q stderr=%q", out.String(), stderr.String())
			assert.Contains(t, out.String(), "valid: true")
			assert.Empty(t, unexpectedEnv)
		})
	}
}

func TestOversizedInlineQuestionsFailBeforeReadingState(t *testing.T) {
	client := &fakeClient{}
	reads := 0
	envReads := 0
	deps := inputSourceDeps(client, nil, "", &reads, func(string) string { envReads++; return "" })
	large := strings.Repeat("x", cli.SourceLimit+1)
	code, renderer, _, stderr := runAsk(t, []string{"ask", "--questions-json", large, "--state", "state"}, deps)
	assert.Equal(t, 2, code)
	assert.Nil(t, renderer.err)
	assert.Zero(t, reads)
	assert.Zero(t, envReads)
	assert.Zero(t, client.call)
	assert.Contains(t, stderr, "exceeds the")
	assert.NotContains(t, stderr, large[:32])
}

func TestQuestionSourceConflictDoesNotOpenFiles(t *testing.T) {
	client := &fakeClient{}
	reads := 0
	envReads := 0
	deps := inputSourceDeps(client, nil, "", &reads, func(string) string { envReads++; return "" })
	code, renderer, _, stderr := runAsk(t, []string{"ask", "--questions", "private.json", "--questions-json", sourceQuestions, "--state", "state"}, deps)
	assert.Equal(t, 2, code)
	assert.Nil(t, renderer.err)
	assert.Zero(t, reads)
	assert.Zero(t, envReads)
	assert.Zero(t, client.call)
	assert.NotContains(t, stderr, "private.json")
}

func TestMalformedInlineQuestionsDoNotOpenOtherSources(t *testing.T) {
	client := &fakeClient{}
	reads := 0
	envReads := 0
	deps := inputSourceDeps(client, nil, "", &reads, func(string) string { envReads++; return "" })
	inline := `{"questions":{"private-secret":{"type":"noul","instructions":"x"},"private-secret":{"type":"noul","instructions":"x"}}}`
	code, renderer, _, stderr := runAsk(t, []string{"ask", "--questions-json", inline, "--state-file", "private-state.txt"}, deps)
	assert.Equal(t, 2, code)
	assert.Nil(t, renderer.err)
	assert.Zero(t, reads)
	assert.Zero(t, envReads)
	assert.Zero(t, client.call)
	assert.NotContains(t, stderr, "private-state")
	assert.NotContains(t, stderr, "private-secret")
}

func TestMalformedStateJSONDoesNotOpenQuestionFile(t *testing.T) {
	for _, tc := range []struct {
		name, value, want string
	}{
		{name: "filename migration", value: "state.json", want: "--state-json-file PATH"},
		{name: "duplicate keys", value: `{"private-key":1,"private-key":2}`, want: "state JSON must be a valid"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := &fakeClient{}
			reads, envReads := 0, 0
			deps := inputSourceDeps(client, nil, "", &reads, func(string) string { envReads++; return "" })
			code, _, _, stderr := runAsk(t, []string{"ask", "--questions", "questions.json", "--state-json", tc.value}, deps)
			assert.Equal(t, 2, code)
			assert.Zero(t, reads)
			assert.Zero(t, envReads)
			assert.Zero(t, client.call)
			assert.Contains(t, stderr, tc.want)
			assert.NotContains(t, stderr, "private-key")
		})
	}
}
