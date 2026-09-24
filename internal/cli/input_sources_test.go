package cli_test

import (
	"bytes"
	"io"
	"strings"
	"testing"
	"time"

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
			if code != 0 || renderer.err != nil || client.call != 1 {
				t.Fatalf("code=%d err=%v calls=%d stderr=%q", code, renderer.err, client.call, stderr)
			}
			if i == 0 {
				expected = client.request
				return
			}
			if !bytes.Equal(client.request.State, expected.State) || client.request.Model != expected.Model || len(client.request.Questions) != len(expected.Questions) || client.request.Questions["q"].Type != expected.Questions["q"].Type || string(client.request.Questions["q"].Instructions) != string(expected.Questions["q"].Instructions) {
				t.Fatalf("request differs from inline source request: got=%s want=%s", mustEncodeRequest(t, client.request), mustEncodeRequest(t, expected))
			}
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
			deps := cli.AskDeps{
				ReadFile: func(path string, _ int64) ([]byte, *jeq.Error) { return files[path], nil },
				ReadStdin: func(r io.Reader, limit int64, _ bool) ([]byte, *jeq.Error) {
					data, err := io.ReadAll(io.LimitReader(r, limit))
					if err != nil {
						return nil, jeq.WrapError(jeq.CodeInputInvalid, err, "reading test stdin")
					}
					return data, nil
				},
				Getenv: func(string) string { t.Fatal("explicit model must not consult environment"); return "" },
				Stdin:  stdin,
			}
			args := append([]string{"validate"}, tc.args...)
			args = append(args, "--model", "m")
			var out, stderr bytes.Buffer
			if code := cli.RunWithDeps(args, &out, &stderr, nil, deps); code != 0 || !strings.Contains(out.String(), "valid: true") {
				t.Fatalf("code=%d stdout=%q stderr=%q", code, out.String(), stderr.String())
			}
		})
	}
}

func TestQuestionSourceConflictDoesNotOpenFiles(t *testing.T) {
	client := &fakeClient{}
	reads := 0
	deps := inputSourceDeps(client, nil, "", &reads, func(string) string { t.Fatal("environment read before conflict"); return "" })
	code, renderer, _, stderr := runAsk(t, []string{"ask", "--questions", "private.json", "--questions-json", sourceQuestions, "--state", "state"}, deps)
	if code != 2 || renderer.err != nil || reads != 0 || client.call != 0 || strings.Contains(stderr, "private.json") {
		t.Fatalf("code=%d err=%v reads=%d calls=%d stderr=%q", code, renderer.err, reads, client.call, stderr)
	}
}

func TestMalformedInlineQuestionsDoNotOpenOtherSources(t *testing.T) {
	client := &fakeClient{}
	reads := 0
	deps := inputSourceDeps(client, nil, "", &reads, func(string) string { t.Fatal("environment read for malformed inline source"); return "" })
	inline := `{"questions":{"private-secret":{"type":"noul","instructions":"x"},"private-secret":{"type":"noul","instructions":"x"}}}`
	code, renderer, _, stderr := runAsk(t, []string{"ask", "--questions-json", inline, "--state-file", "private-state.txt"}, deps)
	if code != 2 || renderer.err != nil || reads != 0 || client.call != 0 || strings.Contains(stderr, "private-state") || strings.Contains(stderr, "private-secret") {
		t.Fatalf("code=%d err=%v reads=%d calls=%d stderr=%q", code, renderer.err, reads, client.call, stderr)
	}
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
			reads := 0
			deps := inputSourceDeps(client, nil, "", &reads, func(string) string { t.Fatal("environment read for malformed inline JSON"); return "" })
			code, _, _, stderr := runAsk(t, []string{"ask", "--questions", "questions.json", "--state-json", tc.value}, deps)
			if code != 2 || reads != 0 || client.call != 0 || !strings.Contains(stderr, tc.want) || strings.Contains(stderr, "private-key") {
				t.Fatalf("code=%d reads=%d calls=%d stderr=%q", code, reads, client.call, stderr)
			}
		})
	}
}

func mustEncodeRequest(t *testing.T, request contract.Request) []byte {
	t.Helper()
	data, err := request.Encode()
	if err != nil {
		t.Fatal(err)
	}
	return data
}
