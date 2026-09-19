package cli_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/cristianoliveira/gev/internal/cli"
)

func TestVersionCommandPrintsPlainBuildInformation(t *testing.T) {
	// Given a fresh version command with an injected stream
	var out bytes.Buffer
	cmd := cli.NewVersionCmd()
	cmd.SetOut(&out)

	// When it runs
	if err := cmd.Execute(); err != nil {
		t.Fatalf("version command returned error: %v", err)
	}

	// Then concise human-readable build information is printed.
	want := "gev dev\nCommit: unknown\n"
	if out.String() != want {
		t.Fatalf("output = %q, want %q", out.String(), want)
	}
	if strings.HasPrefix(strings.TrimSpace(out.String()), "{") {
		t.Fatalf("version output must not be JSON: %q", out.String())
	}
}
