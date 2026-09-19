package cli_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/cristianoliveira/jeq/internal/cli"
	"github.com/cristianoliveira/jeq/internal/domain/contract"
)

func TestRateUsesMapPipelineAndPreservesScoreEvidence(t *testing.T) {
	score := 1.7
	client := &fakeClient{resp: contract.Response{Model: "m", Answers: map[string]contract.Answer{"severity": {Type: contract.TypeScore, Score: &score, Legend: map[string]string{"0": "low"}, Probs: map[string]float64{"0": .2, "1": .8}, Extra: map[string]json.RawMessage{"trace": json.RawMessage(`"x"`)}}}}}
	var out, errOut bytes.Buffer
	deps := RankDeps(client, &out)
	deps.Stdin = strings.NewReader(`{"description":"broken"}
{"description":"minor"}
`)
	code := cli.RunWithDeps([]string{"rate", "--as", "severity", "--input", "ndjson", "--state-pointer", "/description", "--instruction", "How severe?", "--level", "Low", "--level", "High"}, &out, &errOut, RankRenderer{}, deps)
	if code != 0 || errOut.Len() != 0 || client.call != 2 {
		t.Fatalf("code=%d err=%q calls=%d", code, errOut.String(), client.call)
	}
	if !strings.Contains(out.String(), `"trace":"x"`) {
		t.Fatalf("unknown evidence lost: %s", out.String())
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
		if code != 2 || client.call != 0 || out.Len() != 0 {
			t.Fatalf("args=%v code=%d err=%q", args, code, errOut.String())
		}
	}
}
