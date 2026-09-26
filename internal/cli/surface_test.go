package cli_test

import (
	"bytes"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cristianoliveira/jeq/internal/cli"
	"github.com/cristianoliveira/jeq/internal/domain/jeq"
)

func TestBareActionsEqualNativeHelpWithoutDependencyWork(t *testing.T) {
	commands := []string{"ask", "validate", "map", "reduce", "gate"}
	for _, command := range commands {
		t.Run(command, func(t *testing.T) {
			var envReads, fileReads, clientCreates, stdinReads int
			deps := cli.AskDeps{
				Getenv:    func(string) string { envReads++; return "secret" },
				ReadFile:  func(string, int64) ([]byte, *jeq.Error) { fileReads++; return nil, nil },
				ReadStdin: func(io.Reader, int64, bool) ([]byte, *jeq.Error) { stdinReads++; return nil, nil },
				NewClient: func(string, time.Duration, string, int, func(string)) cli.APIClient { clientCreates++; return nil },
			}
			var bare, help, stderr bytes.Buffer
			assert.Equal(t, 0, cli.RunWithDeps([]string{command}, &bare, &stderr, nil, deps), "bare stderr=%q", stderr.String())
			stderr.Reset()
			assert.Equal(t, 0, cli.RunWithDeps([]string{command, "--help"}, &help, &stderr, nil, deps), "help stderr=%q", stderr.String())
			assert.Equal(t, bare.String(), help.String())
			assert.NotEmpty(t, bare.String())
			assert.Empty(t, stderr.String())
			assert.Zero(t, envReads)
			assert.Zero(t, fileReads)
			assert.Zero(t, clientCreates)
			assert.Zero(t, stdinReads)
		})
	}
}

func TestCommandsRejectUnexpectedArgumentsBeforeWork(t *testing.T) {
	commands := []string{"ask", "validate", "map", "reduce", "gate", "version", "models"}
	for _, command := range commands {
		t.Run(command, func(t *testing.T) {
			var out, errOut bytes.Buffer
			var envReads, clientCreates int
			deps := cli.AskDeps{
				Getenv:    func(string) string { envReads++; return "secret" },
				NewClient: func(string, time.Duration, string, int, func(string)) cli.APIClient { clientCreates++; return nil },
			}
			code := cli.RunWithDeps([]string{command, "junk"}, &out, &errOut, nil, deps)
			assert.Equal(t, 2, code)
			assert.Empty(t, out.String())
			assert.True(t, strings.HasPrefix(errOut.String(), "Error: "))
			assert.Zero(t, envReads)
			assert.Zero(t, clientCreates)
		})
	}
}

func TestIncompleteRecipeInvocationUsesNativeHelp(t *testing.T) {
	var bare, help, stderr bytes.Buffer
	for _, id := range []string{"validate-native", "ask-native", "map-gate", "reduce-gate", "map-reduce-gate"} {
		bare.Reset()
		help.Reset()
		stderr.Reset()
		require.Equal(t, 0, cli.Run([]string{"examples", id}, &bare, &stderr, nil), "%s bare stderr=%q", id, stderr.String())
		require.Equal(t, 0, cli.Run([]string{"examples", id, "--help"}, &help, &stderr, nil), "%s help stderr=%q", id, stderr.String())
		assert.Equal(t, bare.String(), help.String(), "%s help", id)
		assert.NotContains(t, bare.String(), "Next:", "%s obsolete navigation", id)
	}
}
