package cli_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/cristianoliveira/gev/internal/cli"
)

func TestVersionCommandPrintsSingleJSONLine(t *testing.T) {
	// Given a fresh version command with an injected stream
	var out bytes.Buffer
	cmd := cli.NewVersionCmd()
	cmd.SetOut(&out)

	// When it runs
	if err := cmd.Execute(); err != nil {
		t.Fatalf("version command returned error: %v", err)
	}

	// Then exactly one JSON line is printed
	if got := strings.Count(out.String(), "\n"); got != 1 {
		t.Fatalf("expected exactly one output line, got %d: %q", got, out.String())
	}

	var info map[string]string
	if err := json.Unmarshal([]byte(strings.TrimSpace(out.String())), &info); err != nil {
		t.Fatalf("output is not valid JSON: %v: %q", err, out.String())
	}

	// And it carries name, dev version, and a commit placeholder
	want := map[string]string{"name": "gev", "version": "dev", "commit": "unknown"}
	for key, expected := range want {
		if info[key] != expected {
			t.Errorf("%s = %q, want %q", key, info[key], expected)
		}
	}
	if len(info) != len(want) {
		t.Errorf("expected exactly %d fields, got %v", len(want), info)
	}
}
