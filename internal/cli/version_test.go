package cli_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cristianoliveira/jeq/internal/cli"
)

func TestVersionCommandPrintsPlainBuildInformation(t *testing.T) {
	// Given a fresh version command with an injected stream
	var out bytes.Buffer
	cmd := cli.NewVersionCmd()
	cmd.SetOut(&out)

	// When it runs
	require.NoError(t, cmd.Execute())

	// Then concise human-readable build information is printed.
	want := "jeq dev\nCommit: unknown\n"
	assert.Equal(t, want, out.String())
	assert.False(t, strings.HasPrefix(strings.TrimSpace(out.String()), "{"), "version output must not be JSON")
}
