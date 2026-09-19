package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"

	"github.com/cristianoliveira/gev/internal/domain/gev"
	"github.com/spf13/cobra"
)

// Run is the dependency-free command entry point used by offline commands.
// The composition root uses RunWithDeps to enable ask.
func Run(args []string, stdout, stderr io.Writer, renderer Renderer) int {
	return RunWithDeps(args, stdout, stderr, renderer, AskDeps{})
}

// RunWithDeps builds a fresh command tree, executes args against injected
// streams and adapters, and returns the process exit code.
func RunWithDeps(args []string, stdout, stderr io.Writer, renderer Renderer, deps AskDeps) int {
	deps.Renderer = renderer
	root := NewRootCmd(deps)
	root.SetOut(stdout)
	root.SetErr(stderr)
	root.SetArgs(args)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	root.SetContext(ctx)

	// Cobra rejects unknown commands during Find, before flag parsing. Build
	// the structured document ourselves: never echo raw Cobra prose.
	root.InitDefaultHelpCmd()
	root.InitDefaultCompletionCmd()
	if _, _, err := root.Find(args); err != nil {
		return renderUsageError(stdout, root, args, err, renderer)
	}

	if err := root.Execute(); err != nil {
		var usage *UsageError
		if errors.As(err, &usage) {
			return renderUsageError(stdout, root, args, err, renderer)
		}
		var coded *gev.Error
		if errors.As(err, &coded) {
			if renderer != nil {
				_ = renderer.RenderError(stdout, ensureRecovery(coded))
			}
			return ExitCode(coded)
		}
		return ExitCode(err)
	}
	return 0
}

func ensureRecovery(e *gev.Error) *gev.Error {
	if e.Recovery != "" {
		return e
	}
	switch e.Code {
	case gev.CodeAuthMissing, gev.CodeAuthRejected:
		return e.WithRecovery("check TYPESAFE_API_KEY and try again")
	case gev.CodeRequestInvalid, gev.CodeInputInvalid, gev.CodeSourceConflict:
		return e.WithRecovery("correct the input and retry")
	case gev.CodeInterrupted:
		return e.WithRecovery("rerun the command when ready")
	default:
		return e.WithRecovery("retry the request; if it persists, inspect the service or network")
	}
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
	errText := err.Error()
	if strings.HasPrefix(errText, "unknown flag:") {
		return errText, "run 'gev --help' for the flag list"
	}
	if strings.Contains(errText, "invalid argument") || strings.Contains(errText, "invalid value") || strings.Contains(errText, "parse error") {
		return "invalid flag value", "run 'gev ask --help' for valid flag values"
	}
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
