package cli_test

import (
	"bytes"
	"math"
	"strings"
	"testing"

	"github.com/cristianoliveira/jeq/internal/cli"
	"github.com/cristianoliveira/jeq/internal/domain/contract"
)

func TestRankAcceptanceInputFailures(t *testing.T) {
	cases := []struct {
		name, input string
		args        []string
	}{
		{"missing state", `[{"id":"a","criteria":"x"}]`, []string{"--instruction", "Which?", "--criteria-pointer", "/criteria"}},
		{"multiple state", `[{"id":"a","criteria":"x"}]`, []string{"--state", "one", "--state-json-file", "state.json", "--instruction", "Which?", "--criteria-pointer", "/criteria"}},
		{"missing criteria pointer", `[{"id":"a"}]`, []string{"--state", "one", "--instruction", "Which?"}},
		{"invalid criteria pointer", `[{"id":"a","criteria":"x"}]`, []string{"--state", "one", "--instruction", "Which?", "--criteria-pointer", "not-a-pointer"}},
		{"empty input", "", []string{"--state", "one", "--instruction", "Which?", "--criteria-pointer", "/criteria"}},
		{"invalid JSON", "[{", []string{"--state", "one", "--instruction", "Which?", "--criteria-pointer", "/criteria"}},
		{"invalid NDJSON", "{\"id\":\"a\"}\nnot-json\n", []string{"--input", "ndjson", "--state", "one", "--criteria-pointer", "/criteria"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client := &fakeClient{}
			var out, stderr bytes.Buffer
			deps := RankDeps(client, &out)
			deps.Stdin = strings.NewReader(tc.input)
			args := []string{"rank", "--as", "route", "--id-pointer", "/id"}
			args = append(args, tc.args...)
			code := cli.RunWithDeps(args, &out, &stderr, RankRenderer{}, deps)
			if code != 2 || out.Len() != 0 || client.call != 0 {
				t.Fatalf("code=%d out=%q stderr=%q calls=%d", code, out.String(), stderr.String(), client.call)
			}
		})
	}
}

func TestRankAcceptsInlineJSONState(t *testing.T) {
	choice := "a"
	client := &fakeClient{resp: contract.Response{Model: "m", Answers: map[string]contract.Answer{"route": {Type: contract.TypeChoice, Choice: &choice, Probs: map[string]float64{"a": 1}}}}}
	var out, stderr bytes.Buffer
	deps := RankDeps(client, &out)
	deps.Stdin = strings.NewReader(`[{"id":"a","criteria":"A"}]`)
	args := []string{"rank", "--as", "route", "--state-json", `{"query":"route"}`, "--instruction", "Which?", "--id-pointer", "/id", "--criteria-pointer", "/criteria"}
	if code := cli.RunWithDeps(args, &out, &stderr, RankRenderer{}, deps); code != 0 || !strings.Contains(out.String(), `"id":"a"`) || string(client.request.State) != `{"query":"route"}` {
		t.Fatalf("code=%d output=%q stderr=%q request-state=%q", code, out.String(), stderr.String(), client.request.State)
	}
}

func TestRankAcceptanceJSONArrayAndFullTieOrder(t *testing.T) {
	choice := "b"
	client := &fakeClient{resp: contract.Response{Model: "m", Answers: map[string]contract.Answer{"route": {Type: contract.TypeChoice, Choice: &choice, Probs: map[string]float64{"a": 1.0 / 3, "b": 1.0 / 3, "c": 1.0 / 3}}}}}
	var out, stderr bytes.Buffer
	deps := RankDeps(client, &out)
	deps.Stdin = strings.NewReader(`[{"id":"a","criteria":null},{"id":"b","criteria":"B"},{"id":"c","criteria":"C"}]`)
	code := cli.RunWithDeps([]string{"rank", "--as", "route", "--state", "one", "--instruction", "Which?", "--id-pointer", "/id", "--criteria-pointer", "/criteria"}, &out, &stderr, RankRenderer{}, deps)
	if code != 0 || stderr.Len() != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
	for _, id := range []string{"b", "a", "c"} {
		if !strings.Contains(out.String(), `"id":"`+id+`"`) {
			t.Fatalf("missing %s in %s", id, out.String())
		}
	}
	if strings.Index(out.String(), `"id":"b"`) > strings.Index(out.String(), `"id":"a"`) || strings.Index(out.String(), `"id":"a"`) > strings.Index(out.String(), `"id":"c"`) {
		t.Fatalf("tie order=%s", out.String())
	}
}

func TestRankAcceptanceRejectsNonFiniteProbabilities(t *testing.T) {
	for _, tc := range []struct {
		name        string
		probability float64
	}{{"NaN", math.NaN()}, {"positive infinity", math.Inf(1)}, {"negative infinity", math.Inf(-1)}} {
		t.Run(tc.name, func(t *testing.T) {
			choice := "a"
			client := &fakeClient{resp: contract.Response{Answers: map[string]contract.Answer{"route": {Type: contract.TypeChoice, Choice: &choice, Probs: map[string]float64{"a": tc.probability}}}}}
			var out, stderr bytes.Buffer
			deps := RankDeps(client, &out)
			deps.Stdin = strings.NewReader(`[{"id":"a","criteria":"A"}]`)
			code := cli.RunWithDeps([]string{"rank", "--as", "route", "--state", "one", "--instruction", "Which?", "--id-pointer", "/id", "--criteria-pointer", "/criteria"}, &out, &stderr, RankRenderer{}, deps)
			if code != 1 || out.Len() != 0 {
				t.Fatalf("code=%d out=%q stderr=%q", code, out.String(), stderr.String())
			}
		})
	}
}

func TestRankAcceptanceUnknownSelectedChoice(t *testing.T) {
	choice := "unknown"
	client := &fakeClient{resp: contract.Response{Answers: map[string]contract.Answer{"route": {Type: contract.TypeChoice, Choice: &choice, Probs: map[string]float64{"a": 1}}}}}
	var out, stderr bytes.Buffer
	deps := RankDeps(client, &out)
	deps.Stdin = strings.NewReader(`[{"id":"a","criteria":"A"}]`)
	code := cli.RunWithDeps([]string{"rank", "--as", "route", "--state", "one", "--instruction", "Which?", "--id-pointer", "/id", "--criteria-pointer", "/criteria"}, &out, &stderr, RankRenderer{}, deps)
	if code != 1 || out.Len() != 0 {
		t.Fatalf("code=%d out=%q stderr=%q", code, out.String(), stderr.String())
	}
}
