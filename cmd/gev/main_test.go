package main

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianoliveira/gev/internal/cli"
	"github.com/cristianoliveira/gev/internal/infra/render"
)

// Regenerate goldens with: nix develop -c go test ./cmd/gev -update
var updateGolden = flag.Bool("update", false, "regenerate golden fixtures")

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
			if strings.TrimSpace(stderr) != "" {
				t.Errorf("stderr must be empty, got %q", stderr)
			}
			assertGolden(t, tt.golden, []byte(stdout))
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

func assertGolden(t *testing.T, name string, got []byte) {
	t.Helper()
	path := filepath.Join("..", "..", "internal", "fixtures", "render", name)
	if *updateGolden {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading golden %s (run with -update): %v", name, err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("stdout drifted from golden %s\nwant:\n%s\ngot:\n%s", name, want, got)
	}
}
