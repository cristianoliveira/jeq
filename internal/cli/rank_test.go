package cli_test

import (
	"bytes"
	"encoding/json"
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
	require.Equal(t, 0, code, "stderr=%q", errOut.String())
	assert.Empty(t, errOut.String())
	var result map[string]any
	require.NoError(t, json.Unmarshal(out.Bytes(), &result))
	items, ok := result["items"].([]any)
	require.True(t, ok)
	require.Len(t, items, 2)
	first, ok := items[0].(map[string]any)
	require.True(t, ok)
	second, ok := items[1].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "b", first["id"])
	assert.Equal(t, "a", second["id"])
	evidence, ok := result["_jeq"].(map[string]any)
	require.True(t, ok)
	route, ok := evidence["route"].(map[string]any)
	require.True(t, ok)
	answers, ok := route["answers"].(map[string]any)
	require.True(t, ok)
	routeAnswer, ok := answers["route"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "jev-latest", route["model"])
	assert.Equal(t, "b", routeAnswer["choice"])
	assert.Equal(t, 1, client.call)
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
			if tc.wantError {
				assert.NotEqual(t, 0, code, "stderr=%q", errOut.String())
				assert.Empty(t, out.String())
				return
			}
			assert.Equal(t, 0, code, "stderr=%q", errOut.String())
			assert.NotEmpty(t, out.String())
		})
	}
}

func TestRankRejectsConflictingStateSourcesBeforeNetwork(t *testing.T) {
	client := &fakeClient{}
	var out, errOut bytes.Buffer
	deps := RankDeps(client, &out)
	deps.Stdin = strings.NewReader(`[{"id":"a","criteria":"x"}]`)
	code := cli.RunWithDeps([]string{"rank", "--as", "route", "--state-file", "-", "--instruction", "Which?", "--id-pointer", "/id", "--criteria-pointer", "/criteria"}, &out, &errOut, RankRenderer{}, deps)
	assert.Equal(t, 2, code)
	assert.Zero(t, client.call)
	assert.Empty(t, out.String())
	assert.Contains(t, errOut.String(), "stdin")
}

func TestRankRejectsTooManyCandidatesBeforeNetwork(t *testing.T) {
	var input strings.Builder
	for i := 0; i < cli.RankMaxOptions+1; i++ {
		input.WriteString(`{"id":"` + string(rune('a'+i%26)) + `-` + string(rune('0'+i/26)) + `","criteria":"x"}` + "\n")
	}
	client := &fakeClient{}
	var out, errOut bytes.Buffer
	deps := RankDeps(client, &out)
	deps.Stdin = strings.NewReader(input.String())
	code := cli.RunWithDeps([]string{"rank", "--as", "route", "--input", "ndjson", "--state", "request", "--instruction", "Which?", "--id-pointer", "/id", "--criteria-pointer", "/criteria"}, &out, &errOut, RankRenderer{}, deps)
	assert.Equal(t, 2, code)
	assert.Zero(t, client.call)
	assert.Empty(t, out.String())
	assert.Contains(t, errOut.String(), "255 Choice option limit")
}

func TestRankRejectsInvalidCandidatesBeforeAuthOrNetwork(t *testing.T) {
	for _, tc := range []struct{ name, input string }{
		{"duplicate", `{"id":"a","criteria":"x"}
{"id":"a","criteria":"y"}`},
		{"missing id", `{"criteria":"x"}`},
		{"non-string id", `{"id":7,"criteria":"x"}`},
		{"empty-string id", `{"id":"","criteria":"x"}`},
		{"missing criteria value", `{"id":"a"}`},
		{"evidence collision", `{"id":"a","criteria":"x","_jeq":{"route":{}}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := &fakeClient{}
			var out, errOut bytes.Buffer
			deps := RankDeps(client, &out)
			deps.Stdin = strings.NewReader(tc.input)
			code := cli.RunWithDeps([]string{"rank", "--as", "route", "--input", "ndjson", "--state", "request", "--instruction", "Which?", "--id-pointer", "/id", "--criteria-pointer", "/criteria"}, &out, &errOut, RankRenderer{}, deps)
			assert.Equal(t, 2, code)
			assert.Empty(t, out.String())
			assert.Contains(t, errOut.String(), "Error: JEQ_INPUT_INVALID")
			assert.Zero(t, client.call)
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
