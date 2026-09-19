package cli_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/cristianoliveira/gev/internal/cli"
	"github.com/spf13/cobra"
)

func TestExamplesCatalogAndDetailsAreDeterministicAndOffline(t *testing.T) {
	r := &valueRenderer{}
	var out, errOut bytes.Buffer
	if code := cli.RunWithDeps([]string{"examples"}, &out, &errOut, r, cli.AskDeps{}); code != 0 {
		t.Fatalf("code=%d errors=%v", code, r.errors)
	}
	if out.Len() >= 2048 || errOut.Len() != 0 {
		t.Fatalf("catalog bytes=%d stderr=%q", out.Len(), errOut.String())
	}
	want := []string{"validate-native", "ask-native", "map-gate", "reduce-gate", "map-reduce-gate"}
	catalogText := out.String()
	previous := -1
	for _, id := range want {
		position := strings.Index(catalogText, "- "+id+":")
		if position <= previous {
			t.Fatalf("catalog order/output=%q", catalogText)
		}
		previous = position

		out.Reset()
		if code := cli.RunWithDeps([]string{"examples", id}, &out, &errOut, r, cli.AskDeps{}); code != 0 {
			t.Fatalf("detail %s code=%d", id, code)
		}
		detail := out.String()
		for _, field := range []string{id + "\n", "Commands:", "Requirements:", "Network calls:", "Input:", "Output:", "Privacy:", "Shell:", "Next:"} {
			if !strings.Contains(detail, field) {
				t.Fatalf("detail %s missing %q: %q", id, field, detail)
			}
		}
		if !strings.Contains(detail, "GEV_BIN=${GEV_BIN:-gev}") || !strings.Contains(detail, "set -euo pipefail") || strings.Contains(detail, "examples/") || strings.Contains(detail, "eval ") {
			t.Fatalf("not self-contained: %s", id)
		}
		if strings.Contains(id, "gate") && !strings.Contains(detail, "jq -c 'del(") {
			t.Fatalf("gate recipe lacks safe final projection: %s", id)
		}
		if id == "map-gate" && !strings.Contains(detail, "gate --as policy --input ndjson") {
			t.Fatalf("multi-record gate lacks NDJSON framing: %s", detail)
		}
	}
	if len(r.values) != 0 {
		t.Fatalf("plain examples unexpectedly used JSON renderer: %#v", r.values)
	}
}

func TestExamplesUnknownAndExtraArgsAreStructuredInputErrors(t *testing.T) {
	for _, args := range [][]string{{"examples", "missing"}, {"examples", "ask-native", "extra"}} {
		r := &valueRenderer{}
		var out, errOut bytes.Buffer
		if code := cli.RunWithDeps(args, &out, &errOut, r, cli.AskDeps{}); code != 2 || out.Len() != 0 || !strings.Contains(errOut.String(), "Error: GEV_INPUT_INVALID") || len(r.errors) != 0 {
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
	for _, command := range []*cobra.Command{cli.NewExamplesCmd(cli.AskDeps{}), cli.NewAskCmd(cli.AskDeps{}), cli.NewValidateCmd(cli.AskDeps{}), cli.NewMapCmd(cli.AskDeps{}), cli.NewReduceCmd(cli.AskDeps{}), cli.NewGateCmd(cli.AskDeps{})} {
		if command.Example == "" {
			t.Fatalf("%s has no example", command.Use)
		}
	}
}
