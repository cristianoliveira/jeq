package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cristianoliveira/jeq/internal/cli"
	"github.com/cristianoliveira/jeq/internal/infra/render"
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
			require.Equal(t, 2, code)
			assert.Empty(t, stdout)
			assert.True(t, strings.HasPrefix(stderr, "Error: "), "stderr=%q", stderr)
		})
	}
}

func TestHelpExitsZero(t *testing.T) {
	for _, args := range [][]string{{"--help"}, {"help"}} {
		code, stdout, stderr := runCLI(args...)
		assert.Equal(t, 0, code, "args %q", args)
		assert.NotEmpty(t, strings.TrimSpace(stdout), "args %q: help must print to stdout", args)
		assert.Empty(t, strings.TrimSpace(stderr), "args %q: help must not write stderr", args)
	}
}
