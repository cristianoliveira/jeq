package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/cristianoliveira/gev/internal/cli"
	"github.com/cristianoliveira/gev/internal/infra/render"
)

// runCLI drives the real composition: the real renderer, the real tree.
func runCLI(args ...string) (code int, stdout, stderr string) {
	var out, errOut bytes.Buffer
	code = cli.Run(args, &out, &errOut, render.JSON{})
	return code, out.String(), errOut.String()
}

func TestBinaryUsageDocuments(t *testing.T) {
	tests := []struct {
		name   string
		args   []string
		golden string
	}{
		{"unknown command", []string{"totally-fake"}, "usage_unknown_command.golden"},
		{"misspelled command suggests", []string{"versionn"}, "usage_suggestion.golden"},
		{"unknown flag", []string{"--nope"}, "usage_unknown_flag.golden"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, stdout, stderr := runCLI(tt.args...)
			if code != 2 {
				t.Fatalf("exit = %d, want 2", code)
			}
			if stdout != "" || !strings.HasPrefix(stderr, "Error: ") {
				t.Errorf("stdout=%q stderr=%q", stdout, stderr)
			}
		})
	}
}

func TestHelpExitsZero(t *testing.T) {
	for _, args := range [][]string{{"--help"}, {"help"}} {
		code, stdout, stderr := runCLI(args...)
		if code != 0 {
			t.Errorf("args %q: exit = %d, want 0", args, code)
		}
		if strings.TrimSpace(stdout) == "" {
			t.Errorf("args %q: help must print to stdout", args)
		}
		if strings.TrimSpace(stderr) != "" {
			t.Errorf("args %q: help must not write stderr, got %q", args, stderr)
		}
	}
}
