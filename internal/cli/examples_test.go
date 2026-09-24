package cli_test

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/cristianoliveira/jeq/internal/cli"
	"github.com/cristianoliveira/jeq/internal/domain/jeq"
	"github.com/spf13/cobra"
)

func TestExamplesUseNativeCobraHelp(t *testing.T) {
	ids := []string{"noul", "choice", "score", "rate-sort", "rank-top-k", "validate-native", "ask-native", "map-gate", "reduce-gate", "map-reduce-gate", "debug-chain"}
	var parent, parentHelp, stderr bytes.Buffer
	if code := cli.RunWithDeps([]string{"examples"}, &parent, &stderr, nil, cli.AskDeps{}); code != 0 {
		t.Fatalf("parent code=%d stderr=%q", code, stderr.String())
	}
	stderr.Reset()
	if code := cli.RunWithDeps([]string{"examples", "--help"}, &parentHelp, &stderr, nil, cli.AskDeps{}); code != 0 || parent.String() != parentHelp.String() {
		t.Fatalf("parent/help code=%d stderr=%q\nparent=%q\nhelp=%q", code, stderr.String(), parent.String(), parentHelp.String())
	}
	for _, phrase := range []string{"typed decisions and probabilities", "jq and the shell own", "does not write replies"} {
		if !strings.Contains(parent.String(), phrase) {
			t.Fatalf("parent missing role contract %q: %s", phrase, parent.String())
		}
	}
	for _, id := range ids {
		var out, help, errOut bytes.Buffer
		if code := cli.RunWithDeps([]string{"examples", id}, &out, &errOut, nil, cli.AskDeps{}); code != 0 {
			t.Fatalf("%s code=%d stderr=%q", id, code, errOut.String())
		}
		if code := cli.RunWithDeps([]string{"examples", id, "--help"}, &help, &errOut, nil, cli.AskDeps{}); code != 0 || out.String() != help.String() {
			t.Fatalf("%s/help mismatch: %q != %q", id, out.String(), help.String())
		}
		for _, field := range []string{"Commands:", "Requirements:", "Network calls:", "Input:", "Output:", "Privacy:"} {
			if !strings.Contains(out.String(), field) {
				t.Fatalf("%s missing %q", id, field)
			}
		}
		if id == "noul" || id == "choice" || id == "score" {
			if !strings.Contains(out.String(), `"type":"`+id+`"`) || !strings.Contains(out.String(), "jeq validate --request -") || !strings.Contains(out.String(), `"questions"`) {
				t.Fatalf("%s missing discoverable request shape and offline validation: %q", id, out.String())
			}
		} else if id == "validate-native" {
			if !strings.Contains(out.String(), "does not call Jev") || !strings.Contains(out.String(), "validates the caller-defined request shape") {
				t.Fatalf("%s missing offline role note: %q", id, out.String())
			}
		} else if !strings.Contains(out.String(), "Jev supplies typed semantic evidence") || !strings.Contains(out.String(), "jq and the shell own") {
			t.Fatalf("%s missing role note: %q", id, out.String())
		}
		if strings.Contains(out.String(), "Next:") {
			t.Fatalf("%s shell/navigation output invalid: %q", id, out.String())
		}
		if id == "debug-chain" && !strings.Contains(out.String(), "reduce --as aggregate --input ndjson") {
			t.Fatalf("%s shell/navigation output invalid: %q", id, out.String())
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
						Getenv: func(string) string { t.Fatal("offline validation must not read environment"); return "" },
					}
					var out, errOut bytes.Buffer
					code := cli.RunWithDeps([]string{"validate", "--request", "-"}, &out, &errOut, nil, deps)
					if code != input.code || !strings.Contains(out.String()+errOut.String(), input.want) {
						t.Fatalf("code=%d stdout=%q stderr=%q", code, out.String(), errOut.String())
					}
				})
			}
		})
	}
}

func TestMapHelpDiscoversEveryQuestionType(t *testing.T) {
	var out bytes.Buffer
	cmd := cli.NewMapCmd(cli.AskDeps{})
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Help(); err != nil {
		t.Fatal(err)
	}
	for _, phrase := range []string{"Noul estimates yes/no", "Choice selects among named options", "Score rates against ordered levels", "non-empty `instructions`", "Noul is optional with true/false string descriptions", "Choice requires a non-empty options object", "Score requires an ordered array of at least two levels", "jeq examples noul", "jeq examples choice", "jeq examples score"} {
		if !strings.Contains(out.String(), phrase) {
			t.Errorf("map help missing %q", phrase)
		}
	}
}

func TestJudgmentHelpStatesJevRole(t *testing.T) {
	for _, command := range []*cobra.Command{cli.NewAskCmd(cli.AskDeps{}), cli.NewMapCmd(cli.AskDeps{}), cli.NewRateCmd(cli.AskDeps{}), cli.NewRankCmd(cli.AskDeps{}), cli.NewReduceCmd(cli.AskDeps{})} {
		var out bytes.Buffer
		command.SetOut(&out)
		command.SetErr(&out)
		if err := command.Help(); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out.String(), "typed semantic") || !strings.Contains(out.String(), "caller") {
			t.Fatalf("%s help=%q", command.Name(), out.String())
		}
	}
}

func TestExamplesResolveCommandNamesOffline(t *testing.T) {
	for _, name := range []string{"ask", "map", "rate", "rank", "reduce", "gate", "validate"} {
		var out, errOut bytes.Buffer
		if code := cli.RunWithDeps([]string{"examples", name}, &out, &errOut, nil, cli.AskDeps{}); code != 0 || !strings.Contains(out.String(), "Canonical offline recipe") {
			t.Fatalf("%s code=%d out=%q err=%q", name, code, out.String(), errOut.String())
		}
		if name == "map" && !strings.Contains(out.String(), "map-gate") {
			t.Fatalf("map canonical=%q", out.String())
		}
		if name == "reduce" && !strings.Contains(out.String(), "reduce-gate") {
			t.Fatalf("reduce canonical=%q", out.String())
		}
	}
}

func TestExamplesUnknownAndExtraArgsUseNativeCobraErrors(t *testing.T) {
	for _, args := range [][]string{{"examples", "missing"}, {"examples", "ask-native", "extra"}} {
		r := &valueRenderer{}
		var out, errOut bytes.Buffer
		if code := cli.RunWithDeps(args, &out, &errOut, r, cli.AskDeps{}); code != 2 || out.Len() != 0 || !strings.HasPrefix(errOut.String(), "Error: ") || len(r.errors) != 0 {
			t.Fatalf("args=%v code=%d stderr=%q errors=%v", args, code, errOut.String(), r.errors)
		}
	}
}

func TestRootAndCommandHelpPointToExamples(t *testing.T) {
	r := &valueRenderer{}
	var out, errOut bytes.Buffer
	if code := cli.RunWithDeps(nil, &out, &errOut, r, cli.AskDeps{}); code != 0 {
		t.Fatal(code)
	}
	if !strings.Contains(out.String(), "Available Commands:") || !strings.Contains(out.String(), "examples") || !strings.Contains(out.String(), "jeq examples map-reduce-gate") || !strings.Contains(out.String(), "classifier on steroids") || !strings.Contains(out.String(), "does not write replies") || !strings.Contains(out.String(), "typed decisions and probabilities") {
		t.Fatalf("root help=%q", out.String())
	}
}
