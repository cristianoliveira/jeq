package cli_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/cristianoliveira/jeq/internal/cli"
	"github.com/cristianoliveira/jeq/internal/domain/contract"
	"github.com/cristianoliveira/jeq/internal/domain/jeq"
)

func TestRateUsesMapPipelineAndPreservesScoreEvidence(t *testing.T) {
	score := 1.7
	confidence := 0.8
	client := &fakeClient{resp: contract.Response{Model: "m", Answers: map[string]contract.Answer{"severity": {Type: contract.TypeScore, Score: &score, Confidence: &confidence, Legend: map[string]string{"0": "low"}, Probs: map[string]float64{"0": .2, "1": .8}, Extra: map[string]json.RawMessage{"trace": json.RawMessage(`"x"`)}}}}}
	var out, errOut bytes.Buffer
	deps := RankDeps(client, &out)
	deps.Stdin = strings.NewReader(`{"description":"broken"}
{"description":"minor"}
`)
	code := cli.RunWithDeps([]string{"rate", "--as", "severity", "--input", "ndjson", "--state-pointer", "/description", "--instruction", "How severe?", "--level", "Low", "--level", "High"}, &out, &errOut, RankRenderer{}, deps)
	assert.Equal(t, 0, code, "stderr=%q", errOut.String())
	assert.Empty(t, errOut.String())
	assert.Equal(t, 2, client.call)
	for _, field := range []string{`"score":1.7`, `"legend"`, `"probabilities"`, `"confidence"`, `"model":"m"`, `"input_tokens":0`, `"trace":"x"`} {
		assert.Contains(t, out.String(), field, "missing evidence field %s", field)
	}
}

func TestRateJSONObjectSuccess(t *testing.T) {
	score := 1.0
	client := &fakeClient{resp: contract.Response{Model: "m", Answers: map[string]contract.Answer{"severity": {Type: contract.TypeScore, Score: &score}}}}
	var out, errOut bytes.Buffer
	deps := RankDeps(client, &out)
	deps.Stdin = strings.NewReader(`{"description":"one"}`)
	code := cli.RunWithDeps([]string{"rate", "--as", "severity", "--state-pointer", "/description", "--instruction", "How?", "--level", "Low", "--level", "High"}, &out, &errOut, RankRenderer{}, deps)
	assert.Equal(t, 0, code, "stderr=%q", errOut.String())
	assert.Equal(t, 1, client.call)
	assert.Contains(t, out.String(), `"description":"one"`)
}

func TestRateRejectsCollisionPointerFramingAndAPIErrorBeforePartialOutput(t *testing.T) {
	cases := []struct {
		name, input string
		err         *jeq.Error
	}{
		{"collision", `{"description":"x","_jeq":{"severity":{}}}`, nil},
		{"invalid pointer", `{"description":"x"}`, nil},
		{"malformed framing", `{"description":"x"}\nnot-json`, nil},
		{"api failure", `{"description":"x"}`, jeq.NewError(jeq.CodeNetworkError, "failed")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client := &fakeClient{err: tc.err}
			var out, errOut bytes.Buffer
			deps := RankDeps(client, &out)
			deps.Stdin = strings.NewReader(tc.input)
			args := []string{"rate", "--as", "severity", "--input", "ndjson", "--state-pointer", "/description", "--instruction", "How?", "--level", "Low", "--level", "High"}
			if tc.name == "invalid pointer" {
				args[6] = "/missing"
			}
			code := cli.RunWithDeps(args, &out, &errOut, RankRenderer{}, deps)
			assert.NotEqual(t, 0, code)
			assert.Empty(t, out.String())
		})
	}
}

func TestRateRejectsBlankAndInsufficientFlagsBeforeNetwork(t *testing.T) {
	for _, args := range [][]string{{"--level", "one"}, {"--level", "one", "--level", "  "}, {"--instruction", "  ", "--level", "one", "--level", "two"}} {
		client := &fakeClient{}
		var out, errOut bytes.Buffer
		deps := RankDeps(client, &out)
		deps.Stdin = strings.NewReader(`{"description":"x"}`)
		base := []string{"rate", "--as", "severity", "--state-pointer", "/description"}
		base = append(base, args...)
		code := cli.RunWithDeps(base, &out, &errOut, RankRenderer{}, deps)
		assert.Equal(t, 2, code, "args=%v stderr=%q", args, errOut.String())
		assert.Zero(t, client.call)
		assert.Empty(t, out.String())
	}
}
