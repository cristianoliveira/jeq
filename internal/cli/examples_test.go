package cli_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/cristianoliveira/jeq/internal/cli"
	"github.com/spf13/cobra"
)

func TestExamplesUseNativeCobraHelp(t *testing.T) {
	ids := []string{"rate-sort", "rank-top-k", "validate-native", "ask-native", "map-gate", "reduce-gate", "map-reduce-gate", "debug-chain"}
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
		if id == "validate-native" {
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
