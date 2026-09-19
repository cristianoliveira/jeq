package cli

import (
	"errors"
	"fmt"
	"io"
)

// usageHint is appended to usage failures so a non-zero exit is never
// silent and always points at the valid alternatives (ADR 0001).
const usageHint = "Run 'gev --help' for usage.\n"

// Run builds a fresh command tree, executes args against injected streams,
// and returns the process exit code (ADR 0001 § Errors).
func Run(args []string, stdout, stderr io.Writer) int {
	root := NewRootCmd()
	root.SetOut(stdout)
	root.SetErr(stderr)
	root.SetArgs(args)

	// Cobra rejects unknown commands during Find, before any flag parsing;
	// classify that rejection as a usage failure and keep it self-correcting.
	if _, _, err := root.Find(args); err != nil {
		fmt.Fprintf(stderr, "Error: %v\n%s", err, usageHint)
		return ExitCode(NewUsageError(err))
	}

	if err := root.Execute(); err != nil {
		var usage *UsageError
		if errors.As(err, &usage) {
			fmt.Fprint(stderr, usageHint)
		}
		return ExitCode(err)
	}
	return 0
}
