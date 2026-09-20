package cli_test

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/cristianoliveira/jeq/internal/cli"
	"github.com/cristianoliveira/jeq/internal/domain/contract"
	"github.com/cristianoliveira/jeq/internal/domain/jeq"
)

type failingOutputRenderer struct{}

func (failingOutputRenderer) RenderSuccess(io.Writer, contract.Response) error {
	return errors.New("writer-secret")
}
func (failingOutputRenderer) RenderError(io.Writer, *jeq.Error) error { return nil }
func (failingOutputRenderer) RenderRaw(io.Writer, []byte) error {
	return errors.New("writer-secret")
}

func (failingOutputRenderer) RenderValue(io.Writer, any) error { return errors.New("writer-secret") }

func TestRankOutputFailureHasTypedVerbosePhase(t *testing.T) {
	choice := "private-candidate"
	confidence := 1.0
	client := &fakeClient{resp: contract.Response{Model: "m", Answers: map[string]contract.Answer{"route": {Type: contract.TypeChoice, Choice: &choice, Confidence: &confidence, Probs: map[string]float64{"private-candidate": 1}}}}}
	var out, errOut bytes.Buffer
	deps := RankDeps(client, &out)
	deps.Stdin = strings.NewReader(`{"id":"private-candidate","criteria":"private-criteria"}` + "\n")
	code := cli.RunWithDeps([]string{"--verbose", "rank", "--as", "route", "--input", "ndjson", "--state", "private-state", "--instruction", "private-prompt", "--id-pointer", "/id", "--criteria-pointer", "/criteria"}, &out, &errOut, failingOutputRenderer{}, deps)
	if code == 0 || out.Len() != 0 || !strings.Contains(errOut.String(), `"phase":"output_write"`) || strings.Contains(errOut.String(), "private-candidate") || strings.Contains(errOut.String(), "private-criteria") {
		t.Fatalf("code=%d stderr=%q", code, errOut.String())
	}
}

func TestAskOutputFailureHasTypedVerbosePhase(t *testing.T) {
	client := &fakeClient{resp: contract.Response{Model: "m", Answers: map[string]contract.Answer{}}}
	deps, _, _ := testDeps(t, client, func(k string) string {
		if k == "TYPESAFE_API_KEY" {
			return "secret"
		}
		return ""
	})
	var out, errOut bytes.Buffer
	code := cli.RunWithDeps([]string{"--verbose", "ask", "--state", "private-state", "--questions", "questions.json"}, &out, &errOut, failingOutputRenderer{}, deps)
	if code == 0 || out.Len() != 0 || !strings.Contains(errOut.String(), `"phase":"output_write"`) || strings.Contains(errOut.String(), "private-candidate") || strings.Contains(errOut.String(), "private-criteria") {
		t.Fatalf("code=%d stderr=%q", code, errOut.String())
	}
}
