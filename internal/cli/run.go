package cli

import (
	"context"
	"errors"
	"io"
	"os"
	"os/signal"
	"strings"

	"github.com/cristianoliveira/jeq/internal/domain/jeq"
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
		root.PrintErrln("Error:", publicCLIError(err))
		return 2
	}
	if err := root.Execute(); err != nil {
		var policy *policyStatus
		if errors.As(err, &policy) {
			return policy.status
		}
		root.PrintErrln("Error:", publicCLIError(err))
		if strings.Contains(err.Error(), "unknown command") || strings.Contains(err.Error(), "unknown flag") || strings.Contains(err.Error(), "requires at most") {
			return 2
		}
		return ExitCode(err)
	}
	return 0
}

// publicCLIError keeps wrapped implementation details internal while Cobra
// owns the standard human-facing "Error:" rendering.
func publicCLIError(err error) error {
	var coded *jeq.Error
	if errors.As(err, &coded) {
		message := coded.Message
		if coded.Code == jeq.CodeAuthRejected {
			message = "the server rejected the credential"
		}
		return jeq.NewError(coded.Code, message)
	}
	return err
}
