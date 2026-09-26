package cli_test

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

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
	assert.NotEqual(t, 0, code)
	assert.Empty(t, out.String())
	assert.Contains(t, errOut.String(), `"phase":"output_write"`)
	assert.NotContains(t, errOut.String(), "private-candidate")
	assert.NotContains(t, errOut.String(), "private-criteria")
}

func TestReduceOutputFailureHasTypedVerbosePhase(t *testing.T) {
	noul := 0.8
	client := &fakeClient{resp: contract.Response{Model: "m", Answers: map[string]contract.Answer{"aggregate": {Type: contract.TypeNoul, Noul: &noul}}}}
	var out, errOut bytes.Buffer
	deps, _, _ := testDeps(t, client, func(k string) string {
		if k == "TYPESAFE_API_KEY" {
			return "secret"
		}
		return ""
	})
	deps.Stdin = strings.NewReader(`[{"private-state":"private-candidate"}]`)
	deps.ReadStdin = func(_ io.Reader, _ int64, _ bool) ([]byte, *jeq.Error) {
		return []byte(`[{"private-state":"private-candidate"}]`), nil
	}
	code := cli.RunWithDeps([]string{"--verbose", "reduce", "--as", "aggregate", "--input", "json", "--questions-json", `{"questions":{"aggregate":{"type":"noul","instructions":"private-prompt"}}}`}, &out, &errOut, failingOutputRenderer{}, deps)
	assert.NotEqual(t, 0, code)
	assert.Empty(t, out.String())
	assert.Contains(t, errOut.String(), `"phase":"output_write"`)
	assert.NotContains(t, errOut.String(), "private-candidate")
	assert.NotContains(t, errOut.String(), "private-prompt")
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
	assert.NotEqual(t, 0, code)
	assert.Empty(t, out.String())
	assert.Contains(t, errOut.String(), `"phase":"output_write"`)
	assert.NotContains(t, errOut.String(), "private-candidate")
	assert.NotContains(t, errOut.String(), "private-criteria")
}
