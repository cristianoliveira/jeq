package cli

import "io"

// Run builds a fresh command tree, executes args against injected streams,
// and returns the process exit code (ADR 0001 § Errors).
func Run(args []string, stdout, stderr io.Writer) int {
	root := NewRootCmd()
	root.SetOut(stdout)
	root.SetErr(stderr)
	root.SetArgs(args)

	// Cobra rejects unknown commands during Find, before any flag parsing;
	// classify that rejection as a usage failure.
	if _, _, err := root.Find(args); err != nil {
		return ExitCode(NewUsageError(err))
	}

	if err := root.Execute(); err != nil {
		return ExitCode(err)
	}
	return 0
}
