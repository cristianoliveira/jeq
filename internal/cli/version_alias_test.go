package cli_test

import (
	"bytes"
	"testing"

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
		if err := cmd.Execute(); err != nil {
			t.Fatalf("args=%v err=%v", args, err)
		}
		if stderr.Len() != 0 {
			t.Fatalf("args=%v stderr=%q", args, stderr.String())
		}
		outputs = append(outputs, out.String())
	}
	if outputs[0] != "jeq v-test\nCommit: abc123\n" || outputs[1] != outputs[0] || outputs[2] != outputs[0] {
		t.Fatalf("outputs=%q", outputs)
	}
}
