package cli_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cristianoliveira/jeq/internal/cli"
)

func TestRootVersionAliasesMatchSubcommand(t *testing.T) {
	oldVersion, oldCommit := cli.Version, cli.Commit
	cli.Version, cli.Commit = "v-test", "abc123"
	defer func() { cli.Version, cli.Commit = oldVersion, oldCommit }()
	outputs := make([]string, 0, 3)
	for _, args := range [][]string{{"version"}, {"--version"}, {"-v"}} {
		var out, stderr bytes.Buffer
		cmd := cli.NewRootCmd()
		cmd.SetOut(&out)
		cmd.SetErr(&stderr)
		cmd.SetArgs(args)
		require.NoError(t, cmd.Execute(), "args=%v", args)
		assert.Empty(t, stderr.String(), "args=%v", args)
		outputs = append(outputs, out.String())
	}
	require.Len(t, outputs, 3)
	assert.Equal(t, "jeq v-test\nCommit: abc123\n", outputs[0])
	assert.Equal(t, outputs[0], outputs[1])
	assert.Equal(t, outputs[0], outputs[2])
}
