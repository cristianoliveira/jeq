package cli_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/cristianoliveira/gev/internal/cli"
)

func TestExamplesUseNativeCobraHelp(t *testing.T) {
	ids := []string{"validate-native", "ask-native", "map-gate", "reduce-gate", "map-reduce-gate"}
	var parent, parentHelp, stderr bytes.Buffer
	if code := cli.RunWithDeps([]string{"examples"}, &parent, &stderr, nil, cli.AskDeps{}); code != 0 {
		t.Fatalf("parent code=%d stderr=%q", code, stderr.String())
	}
	stderr.Reset()
	if code := cli.RunWithDeps([]string{"examples", "--help"}, &parentHelp, &stderr, nil, cli.AskDeps{}); code != 0 || parent.String() != parentHelp.String() {
		t.Fatalf("parent/help code=%d stderr=%q\nparent=%q\nhelp=%q", code, stderr.String(), parent.String(), parentHelp.String())
	}
	for _, id := range ids {
		var out, help, errOut bytes.Buffer
		if code := cli.RunWithDeps([]string{"examples", id}, &out, &errOut, nil, cli.AskDeps{}); code != 0 {
			t.Fatalf("%s code=%d stderr=%q", id, code, errOut.String())
		}
		if code := cli.RunWithDeps([]string{"examples", id, "--help"}, &help, &errOut, nil, cli.AskDeps{}); code != 0 || out.String() != help.String() {
			t.Fatalf("%s/help mismatch: %q != %q", id, out.String(), help.String())
		}
		for _, field := range []string{"Commands:", "Requirements:", "Network calls:", "Input:", "Output:", "Privacy:", "Exits:"} {
			if !strings.Contains(out.String(), field) {
				t.Fatalf("%s missing %q", id, field)
			}
		}
		if !strings.Contains(out.String(), "GEV_BIN=${GEV_BIN:-gev}") || strings.Contains(out.String(), "Next:") {
			t.Fatalf("%s shell/navigation output invalid: %q", id, out.String())
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
	if !strings.Contains(out.String(), "Available Commands:") || !strings.Contains(out.String(), "examples") || !strings.Contains(out.String(), "gev examples map-reduce-gate") {
		t.Fatalf("root help=%q", out.String())
	}
}
