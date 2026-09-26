package cli_test

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cristianoliveira/jeq/internal/cli"
	"github.com/cristianoliveira/jeq/internal/domain/jeq"
	"github.com/spf13/cobra"
)

func TestExamplesUseNativeCobraHelp(t *testing.T) {
	ids := []string{"noul", "choice", "score", "rate-sort", "rank-top-k", "validate-native", "ask-native", "map-gate", "reduce-gate", "map-reduce-gate", "debug-chain"}
	var parent, parentHelp, stderr bytes.Buffer
	require.Equal(t, 0, cli.RunWithDeps([]string{"examples"}, &parent, &stderr, nil, cli.AskDeps{}), "stderr=%q", stderr.String())
	stderr.Reset()
	require.Equal(t, 0, cli.RunWithDeps([]string{"examples", "--help"}, &parentHelp, &stderr, nil, cli.AskDeps{}), "stderr=%q", stderr.String())
	assert.Equal(t, parent.String(), parentHelp.String())
	for _, phrase := range []string{"typed decisions and probabilities", "jq and the shell own", "does not write replies"} {
		assert.Contains(t, parent.String(), phrase, "parent role contract")
	}
	for _, id := range ids {
		var out, help, errOut bytes.Buffer
		require.Equal(t, 0, cli.RunWithDeps([]string{"examples", id}, &out, &errOut, nil, cli.AskDeps{}), "%s stderr=%q", id, errOut.String())
		require.Equal(t, 0, cli.RunWithDeps([]string{"examples", id, "--help"}, &help, &errOut, nil, cli.AskDeps{}), "%s help stderr=%q", id, errOut.String())
		assert.Equal(t, out.String(), help.String(), "%s help", id)
		for _, field := range []string{"Commands:", "Requirements:", "Network calls:", "Input:", "Output:", "Privacy:"} {
			assert.Contains(t, out.String(), field, "%s example field", id)
		}
		switch id {
		case "noul", "choice", "score":
			assert.Contains(t, out.String(), `"type":"`+id+`"`, "%s request shape", id)
			assert.Contains(t, out.String(), "jeq validate --request -", "%s offline validation", id)
			assert.Contains(t, out.String(), `"questions"`, "%s questions", id)
		case "validate-native":
			assert.Contains(t, out.String(), "does not call Jev", "%s offline role note", id)
			assert.Contains(t, out.String(), "validates the caller-defined request shape", "%s request shape note", id)
		default:
			assert.Contains(t, out.String(), "Jev supplies typed semantic evidence", "%s role note", id)
			assert.Contains(t, out.String(), "jq and the shell own", "%s shell role note", id)
		}
		assert.NotContains(t, out.String(), "Next:", "%s obsolete navigation", id)
		if id == "debug-chain" {
			assert.Contains(t, out.String(), "reduce --as aggregate --input ndjson", "%s shell example", id)
		}
	}
}

func TestValidateQuestionTypesOffline(t *testing.T) {
	cases := []struct {
		name      string
		valid     string
		invalid   string
		wantError string
	}{
		{name: "noul", valid: `{"model":"jev-latest","state":"file","questions":{"q":{"type":"noul","instructions":"Is this urgent?","criteria":{"true":"Act now","false":"Can wait"}}}}`, invalid: `{"model":"jev-latest","state":"file","questions":{"q":{"type":"noul","instructions":"Is this urgent?","criteria":{"true":1}}}}`, wantError: "questions.q.criteria: noul criteria must be an object"},
		{name: "choice", valid: `{"model":"jev-latest","state":"file","questions":{"q":{"type":"choice","instructions":"Which category?","criteria":{"invoice":"Financial","report":"Analysis"}}}}`, invalid: `{"model":"jev-latest","state":"file","questions":{"q":{"type":"choice","instructions":"Which category?","criteria":{}}}}`, wantError: "questions.q.criteria: choice needs a non-empty map of options"},
		{name: "score", valid: `{"model":"jev-latest","state":"file","questions":{"q":{"type":"score","instructions":"How severe?","criteria":["Low","Medium","High"]}}}`, invalid: `{"model":"jev-latest","state":"file","questions":{"q":{"type":"score","instructions":"How severe?","criteria":["Low"]}}}`, wantError: "questions.q.criteria: score needs at least two level descriptions"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, input := range []struct {
				name, raw, want string
				code            int
			}{
				{name: "valid", raw: tc.valid, want: "valid: true", code: 0},
				{name: "invalid criteria", raw: tc.invalid, want: tc.wantError, code: 2},
			} {
				t.Run(input.name, func(t *testing.T) {
					envReads := 0
					deps := cli.AskDeps{
						ReadFile: func(string, int64) ([]byte, *jeq.Error) { return nil, nil },
						Stdin:    strings.NewReader(input.raw),
						ReadStdin: func(stdin io.Reader, limit int64, _ bool) ([]byte, *jeq.Error) {
							data, err := io.ReadAll(io.LimitReader(stdin, limit))
							if err != nil {
								return nil, jeq.WrapError(jeq.CodeInputInvalid, err, "reading test stdin")
							}
							return data, nil
						},
						Getenv: func(string) string { envReads++; return "" },
					}
					var out, errOut bytes.Buffer
					code := cli.RunWithDeps([]string{"validate", "--request", "-"}, &out, &errOut, nil, deps)
					assert.Equal(t, input.code, code)
					assert.Contains(t, out.String()+errOut.String(), input.want)
					assert.Zero(t, envReads)
				})
			}
		})
	}
}

func TestOfflineQuestionExamplesMatchTheirRequirements(t *testing.T) {
	for _, id := range []string{"noul", "score"} {
		var out, errOut bytes.Buffer
		require.Equal(t, 0, cli.RunWithDeps([]string{"examples", id}, &out, &errOut, nil, cli.AskDeps{}), "%s stderr=%q", id, errOut.String())
		for _, expected := range []string{"Commands: validate", "Requirements: installed jeq, bash", "Network calls: 0 API requests"} {
			assert.Contains(t, out.String(), expected, "%s offline requirement", id)
		}
		assert.NotContains(t, out.String(), "TYPESAFE_API_KEY", "%s unused API requirement", id)
		assert.NotContains(t, out.String(), "map:", "%s unused map requirement", id)
	}
}

func TestMapHelpDiscoversEveryQuestionType(t *testing.T) {
	var out bytes.Buffer
	cmd := cli.NewMapCmd(cli.AskDeps{})
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	require.NoError(t, cmd.Help())
	for _, phrase := range []string{"Noul estimates yes/no", "Choice selects among named options", "Score rates against ordered levels", "non-empty `instructions`", "Noul is optional with true/false string descriptions", "Choice requires a non-empty options object", "Score requires an ordered array of at least two levels", "jeq examples noul", "jeq examples choice", "jeq examples score"} {
		assert.Contains(t, out.String(), phrase, "map help")
	}
}

func TestRankExampleUsesExplicitNDJSONAndSyntheticInputs(t *testing.T) {
	cmd := cli.NewRankCmd(cli.AskDeps{})
	var help bytes.Buffer
	cmd.SetOut(&help)
	cmd.SetErr(&help)
	require.NoError(t, cmd.Help())
	for _, phrase := range []string{"--input ndjson", `"name":"billing"`, "--state 'A customer asks about a refund'"} {
		assert.Contains(t, help.String(), phrase, "rank help")
	}
}

func TestJudgmentHelpStatesJevRole(t *testing.T) {
	for _, command := range []*cobra.Command{cli.NewAskCmd(cli.AskDeps{}), cli.NewMapCmd(cli.AskDeps{}), cli.NewRateCmd(cli.AskDeps{}), cli.NewRankCmd(cli.AskDeps{}), cli.NewReduceCmd(cli.AskDeps{})} {
		var out bytes.Buffer
		command.SetOut(&out)
		command.SetErr(&out)
		require.NoError(t, command.Help())
		assert.Contains(t, out.String(), "typed semantic", "%s help", command.Name())
		assert.Contains(t, out.String(), "caller", "%s help", command.Name())
	}
}

func TestExamplesResolveCommandNamesOffline(t *testing.T) {
	for _, name := range []string{"ask", "map", "rate", "rank", "reduce", "gate", "validate"} {
		var out, errOut bytes.Buffer
		code := cli.RunWithDeps([]string{"examples", name}, &out, &errOut, nil, cli.AskDeps{})
		require.Equal(t, 0, code, "%s stderr=%q", name, errOut.String())
		assert.Contains(t, out.String(), "Canonical offline recipe", "%s", name)
		if name == "map" {
			assert.Contains(t, out.String(), "map-gate")
		}
		if name == "reduce" {
			assert.Contains(t, out.String(), "reduce-gate")
		}
	}
}

func TestExamplesUnknownAndExtraArgsUseNativeCobraErrors(t *testing.T) {
	for _, args := range [][]string{{"examples", "missing"}, {"examples", "ask-native", "extra"}} {
		r := &valueRenderer{}
		var out, errOut bytes.Buffer
		code := cli.RunWithDeps(args, &out, &errOut, r, cli.AskDeps{})
		assert.Equal(t, 2, code)
		assert.Empty(t, out.String())
		assert.True(t, strings.HasPrefix(errOut.String(), "Error: "))
		assert.Empty(t, r.errors)
	}
}

func TestRootAndCommandHelpPointToExamples(t *testing.T) {
	r := &valueRenderer{}
	var out, errOut bytes.Buffer
	require.Equal(t, 0, cli.RunWithDeps(nil, &out, &errOut, r, cli.AskDeps{}))
	assert.Contains(t, out.String(), "Available Commands:")
	assert.Contains(t, out.String(), "examples")
	assert.Contains(t, out.String(), "jeq examples map-reduce-gate")
	assert.Contains(t, out.String(), "classifier on steroids")
	assert.Contains(t, out.String(), "does not write replies")
	assert.Contains(t, out.String(), "typed decisions and probabilities")
}
