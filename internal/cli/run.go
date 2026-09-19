package cli

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/cristianoliveira/gev/internal/domain/gev"
	"github.com/spf13/cobra"
)

// Run builds a fresh command tree, executes args against injected streams,
// and returns the process exit code (ADR 0001 § Errors). Usage failures emit
// exactly one structured error document on stdout through the injected
// renderer; stderr stays empty so nothing duplicates the machine document.
func Run(args []string, stdout, stderr io.Writer, renderer Renderer) int {
	root := NewRootCmd()
	root.SetOut(stdout)
	root.SetErr(stderr)
	root.SetArgs(args)
	// Register built-ins so `gev help` and `gev completion` resolve before
	// the Find pre-check below.
	root.InitDefaultHelpCmd()
	root.InitDefaultCompletionCmd()

	// Cobra rejects unknown commands during Find, before flag parsing. Build
	// the structured document ourselves: never echo raw Cobra prose.
	if _, _, err := root.Find(args); err != nil {
		return renderUsageError(stdout, root, args, err, renderer)
	}

	if err := root.Execute(); err != nil {
		var usage *UsageError
		if errors.As(err, &usage) {
			return renderUsageError(stdout, root, args, err, renderer)
		}
		return ExitCode(err)
	}
	return 0
}

// renderUsageError emits one structured error document (GEV_INPUT_INVALID,
// offending input, deterministic recovery) and returns the usage exit code.
func renderUsageError(stdout io.Writer, root *cobra.Command, args []string, err error, renderer Renderer) int {
	doc := usageDocument(root, args, err)
	if renderer != nil {
		_ = renderer.RenderError(stdout, doc)
	}
	return ExitCode(doc)
}

func usageDocument(root *cobra.Command, args []string, err error) *gev.Error {
	msg, recovery := describeUsage(root, args, err)
	return gev.NewError(gev.CodeInputInvalid, msg).WithRecovery(recovery)
}

func describeUsage(root *cobra.Command, args []string, err error) (message, recovery string) {
	// Unknown flag: the error names the offending flag directly.
	if strings.HasPrefix(err.Error(), "unknown flag:") {
		return err.Error(), "run 'gev --help' for the flag list"
	}

	// Unknown command: name it and offer the closest deterministic
	// alternative when Cobra can compute one.
	typed := ""
	if len(args) > 0 {
		typed = args[0]
	}
	message = fmt.Sprintf("unknown command %q", typed)
	if s := root.SuggestionsFor(typed); len(s) > 0 {
		return message, fmt.Sprintf("did you mean %q? run 'gev --help' for the command list", s[0])
	}
	return message, "run 'gev --help' for the command list"
}
