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

func TestRankRejectsInvalidProbabilityResponses(t *testing.T) {
	cases := []struct {
		name      string
		probs     map[string]float64
		choice    string
		wantError bool
	}{
		{"negative", map[string]float64{"a": -.1, "b": 1.1}, "b", true},
		{"over one", map[string]float64{"a": 1.2, "b": -.2}, "a", true},
		{"wrong sum", map[string]float64{"a": .2, "b": .2}, "a", true},
		{"missing", map[string]float64{"b": 1}, "b", true},
		{"extra", map[string]float64{"a": .2, "b": .7, "c": .1}, "b", true},
		{"unique max mismatch", map[string]float64{"a": .8, "b": .2}, "b", true},
		{"tie selected first", map[string]float64{"a": .5, "b": .5}, "b", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client := &fakeClient{resp: contract.Response{Model: "m", Answers: map[string]contract.Answer{"route": {Type: contract.TypeChoice, Choice: &tc.choice, Probs: tc.probs}}}}
			var out, errOut bytes.Buffer
			deps := RankDeps(client, &out)
			deps.Stdin = strings.NewReader("[{\"id\":\"a\",\"criteria\":null},{\"id\":\"b\",\"criteria\":\"B\"}]")
			code := cli.RunWithDeps([]string{"rank", "--as", "route", "--state", "request", "--instruction", "Which?", "--id-pointer", "/id", "--criteria-pointer", "/criteria"}, &out, &errOut, RankRenderer{}, deps)
			if tc.wantError && (code != 1 || out.Len() != 0) {
				t.Fatalf("code=%d out=%q err=%q", code, out.String(), errOut.String())
			}
			if !tc.wantError && (code != 0 || out.Len() == 0) {
				t.Fatalf("code=%d out=%q err=%q", code, out.String(), errOut.String())
			}
		})
	}
}

func TestRankRejectsConflictingStateSourcesBeforeNetwork(t *testing.T) {
	client := &fakeClient{}
	var out, errOut bytes.Buffer
	deps := RankDeps(client, &out)
	deps.Stdin = strings.NewReader(`[{"id":"a","criteria":"x"}]`)
	code := cli.RunWithDeps([]string{"rank", "--as", "route", "--state", "request", "--state-file", "request.txt", "--instruction", "Which?", "--id-pointer", "/id", "--criteria-pointer", "/criteria"}, &out, &errOut, RankRenderer{}, deps)
	if code != 2 || client.call != 0 || out.Len() != 0 {
		t.Fatalf("code=%d calls=%d out=%q err=%q", code, client.call, out.String(), errOut.String())
	}
}

func TestRankRejectsTooManyCandidatesBeforeNetwork(t *testing.T) {
	var input strings.Builder
	for i := 0; i < cli.RankMaxOptions+1; i++ {
		input.WriteString(`{"id":"` + string(rune('a'+i%26)) + `-` + string(rune('0'+i/26)) + `","criteria":"x"}` + "\\n")
	}
	client := &fakeClient{}
	var out, errOut bytes.Buffer
	deps := RankDeps(client, &out)
	deps.Stdin = strings.NewReader(input.String())
	code := cli.RunWithDeps([]string{"rank", "--as", "route", "--input", "ndjson", "--state", "request", "--instruction", "Which?", "--id-pointer", "/id", "--criteria-pointer", "/criteria"}, &out, &errOut, RankRenderer{}, deps)
	if code != 2 || client.call != 0 || out.Len() != 0 {
		t.Fatalf("code=%d calls=%d out=%q err=%q", code, client.call, out.String(), errOut.String())
	}
}

func TestRankRejectsInvalidCandidatesBeforeAuthOrNetwork(t *testing.T) {
	for _, tc := range []struct{ name, input string }{
		{"duplicate", `{"id":"a","criteria":"x"}
{"id":"a","criteria":"y"}`},
		{"missing id", `{"criteria":"x"}`},
		{"non-string id", `{"id":7,"criteria":"x"}`},
		{"evidence collision", `{"id":"a","criteria":"x","_jeq":{"route":{}}}`},
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
