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
func Run(args []string, stdout, stderr io.Writer, renderer Renderer) int {
	return RunWithDeps(args, stdout, stderr, renderer, AskDeps{})
}

// RunWithDeps builds a fresh command tree and keeps human diagnostics on stderr.
func RunWithDeps(args []string, stdout, stderr io.Writer, renderer Renderer, deps AskDeps) int {
	deps.Renderer = renderer
	if deps.Getenv == nil {
		deps.Getenv = os.Getenv
	}
	root := NewRootCmd(deps)
	root.SetOut(stdout)
	root.SetErr(stderr)
	root.SetArgs(args)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	root.SetContext(ctx)
	root.InitDefaultHelpCmd()
	root.InitDefaultCompletionCmd()
	if _, _, err := root.Find(args); err != nil {
		return renderUsageError(stderr, root, args, err, renderer)
	}
	if err := root.Execute(); err != nil {
		var policy *policyStatus
		if errors.As(err, &policy) {
			return policy.status
		}
		var coded *gev.Error
		if errors.As(err, &coded) {
			coded = ensureRecovery(coded)
			if renderer != nil {
				_ = renderer.RenderError(io.Discard, coded)
			}
			writeCLIError(stderr, coded)
			return ExitCode(coded)
		}
		if strings.HasPrefix(err.Error(), "unknown flag:") {
			coded := gev.NewError(gev.CodeInputInvalid, err.Error()).WithRecovery("run 'gev --help' for the flag list")
			if renderer != nil {
				_ = renderer.RenderError(io.Discard, coded)
			}
			writeCLIError(stderr, coded)
			return 2
		}
		writeCLIError(stderr, gev.NewError(gev.CodeResponseInvalid, err.Error()))
		return ExitCode(err)
	}
	return 0
}

func writeCLIError(w io.Writer, err *gev.Error) {
	_, _ = fmt.Fprintf(w, "Error: %s: %s\n", err.Code, err.Message)
	if err.Recovery != "" {
		_, _ = fmt.Fprintf(w, "Try: %s\n", err.Recovery)
	}
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

func renderUsageError(stderr io.Writer, root *cobra.Command, args []string, err error, renderer Renderer) int {
	message, recovery := describeUsage(root, args, err)
	coded := gev.NewError(gev.CodeInputInvalid, message).WithRecovery(recovery)
	if renderer != nil {
		_ = renderer.RenderError(io.Discard, coded)
	}
	writeCLIError(stderr, coded)
	return 2
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
