package cli_test

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/cristianoliveira/jeq/internal/cli"
	"github.com/cristianoliveira/jeq/internal/domain/contract"
	"github.com/cristianoliveira/jeq/internal/domain/jeq"
)

type RankRenderer struct{}

func (RankRenderer) RenderSuccess(io.Writer, contract.Response) error { return nil }
func (RankRenderer) RenderRaw(w io.Writer, raw []byte) error {
	_, err := w.Write(append(raw, '\n'))
	return err
}

func TestRankSelectsOriginalCandidateAndAttachesCompleteEvidence(t *testing.T) {
	choice := "b"
	confidence := 0.91
	client := &fakeClient{resp: contract.Response{Model: "jev-latest", Answers: map[string]contract.Answer{"route": {Type: contract.TypeChoice, Choice: &choice, Confidence: &confidence, Probs: map[string]float64{"a": .09, "b": .91}}}, Usage: contract.Usage{InputTokens: 3}}}
	var out, errOut bytes.Buffer
	deps := RankDeps(client, &out)
	deps.Stdin = strings.NewReader(`{"name":"a","description":{"text":"first"}}
{"name":"b","description":"second"}
`)
	code := cli.RunWithDeps([]string{"rank", "--as", "route", "--input", "ndjson", "--state", "request", "--instruction", "Which?", "--id-pointer", "/name", "--criteria-pointer", "/description"}, &out, &errOut, RankRenderer{}, deps)
	if code != 0 || errOut.Len() != 0 {
		t.Fatalf("code=%d stderr=%q", code, errOut.String())
	}
	var result map[string]any
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	items := result["items"].([]any)
	if items[0].(map[string]any)["id"] != "b" || items[1].(map[string]any)["id"] != "a" {
		t.Fatalf("ranking=%#v", items)
	}
	evidence := result["_jeq"].(map[string]any)["route"].(map[string]any)
	if evidence["model"] != "jev-latest" || evidence["answers"].(map[string]any)["route"].(map[string]any)["choice"] != "b" {
		t.Fatalf("evidence=%#v", evidence)
	}
	if client.call != 1 {
		t.Fatalf("requests=%d", client.call)
	}
}

func TestRankRejectsInvalidCandidatesBeforeAuthOrNetwork(t *testing.T) {
	for _, tc := range []struct{ name, input string }{
		{"duplicate", `{"id":"a","criteria":"x"}
{"id":"a","criteria":"y"}`},
		{"missing id", `{"criteria":"x"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := &fakeClient{}
			var out, errOut bytes.Buffer
			deps := RankDeps(client, &out)
			deps.Stdin = strings.NewReader(tc.input)
			code := cli.RunWithDeps([]string{"rank", "--as", "route", "--input", "ndjson", "--state", "request", "--instruction", "Which?", "--id-pointer", "/id", "--criteria-pointer", "/criteria"}, &out, &errOut, RankRenderer{}, deps)
			if code != 2 || out.Len() != 0 || !strings.Contains(errOut.String(), "Error: JEQ_INPUT_INVALID") || client.call != 0 {
				t.Fatalf("code=%d out=%q err=%q calls=%d", code, out.String(), errOut.String(), client.call)
			}
		})
	}
}

func RankDeps(client *fakeClient, _ *bytes.Buffer) cli.AskDeps {
	return cli.AskDeps{
		Stdin: strings.NewReader(""), ReadStdin: func(r io.Reader, _ int64, _ bool) ([]byte, *jeq.Error) {
			b, err := io.ReadAll(r)
			if err != nil {
				return nil, jeq.NewError(jeq.CodeInputInvalid, err.Error())
			}
			return b, nil
		},
		ReadFile: func(string, int64) ([]byte, *jeq.Error) { return nil, nil },
		Getenv: func(key string) string {
			if key == "TYPESAFE_API_KEY" {
				return "secret"
			}
			return ""
		},
		NewClient: func(string, time.Duration, string, int, func(string)) cli.APIClient { return client },
	}
}
