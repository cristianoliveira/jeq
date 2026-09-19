package cli_test

import (
	"bytes"
	"encoding/json"
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
	catalog := asMap(t, r.values[len(r.values)-1])
	items := catalog["examples"].([]any)
	want := []string{"validate-native", "ask-native", "map-gate", "reduce-gate", "map-reduce-gate"}
	if len(items) != len(want) {
		t.Fatalf("items=%#v", items)
	}
	for i, id := range want {
		item := items[i].(map[string]any)
		if item["id"] != id || item["purpose"] == "" || item["network_calls"] == "" {
			t.Fatalf("item[%d]=%#v", i, items[i])
		}
		if code := cli.RunWithDeps([]string{"examples", id}, &out, &errOut, r, cli.AskDeps{}); code != 0 {
			t.Fatalf("detail %s code=%d", id, code)
		}
		detail := asMap(t, r.values[len(r.values)-1])
		for _, key := range []string{"purpose", "covers", "requirements", "network_calls", "shell", "input_shape", "output_shape", "privacy", "next_step"} {
			if detail[key] == nil || detail[key] == "" {
				t.Fatalf("detail %s missing %s", id, key)
			}
		}
		shell := detail["shell"].(string)
		if !strings.Contains(shell, "GEV_BIN=${GEV_BIN:-gev}") || !strings.Contains(shell, "set -euo pipefail") || strings.Contains(shell, "examples/") || strings.Contains(shell, "eval ") {
			t.Fatalf("not self-contained: %s", id)
		}
		if strings.Contains(id, "gate") && !strings.Contains(shell, "jq -c 'del(") {
			t.Fatalf("gate recipe lacks safe final projection: %s", id)
		}
		if id == "map-gate" && !strings.Contains(shell, "gate --as policy --input ndjson") {
			t.Fatalf("multi-record gate lacks NDJSON framing: %s", shell)
		}
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

func TestHomeAndCommandHelpPointToExamples(t *testing.T) {
	r := &valueRenderer{}
	var out, errOut bytes.Buffer
	if code := cli.RunWithDeps(nil, &out, &errOut, r, cli.AskDeps{}); code != 0 {
		t.Fatal(code)
	}
	home := asMap(t, r.values[0])
	commands := fmtStrings(home["commands"])
	if !contains(commands, "examples") || home["next_step"] != "run gev examples" {
		t.Fatalf("home=%#v", home)
	}
	for _, command := range []*cobra.Command{cli.NewExamplesCmd(cli.AskDeps{}), cli.NewAskCmd(cli.AskDeps{}), cli.NewValidateCmd(cli.AskDeps{}), cli.NewMapCmd(cli.AskDeps{}), cli.NewReduceCmd(cli.AskDeps{}), cli.NewGateCmd(cli.AskDeps{})} {
		if command.Example == "" {
			t.Fatalf("%s has no example", command.Use)
		}
	}
}

func asMap(t *testing.T, value any) map[string]any {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func fmtStrings(value any) []string {
	values, _ := value.([]any)
	out := make([]string, 0, len(values))
	for _, value := range values {
		out = append(out, value.(string))
	}
	return out
}

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}
