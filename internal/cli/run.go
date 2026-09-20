package cli

import (
	"context"
	"errors"
	"io"
	"os"
	"os/signal"
	"strings"

	"github.com/cristianoliveira/jeq/internal/domain/jeq"
	"github.com/cristianoliveira/jeq/internal/trace"
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
	target, _, err := root.Find(args)
	if err != nil {
		root.PrintErrln("Error:", publicCLIError(err))
		return 2
	}
	if err := root.Execute(); err != nil {
		if cfg := trace.FromContext(root.Context()); cfg != nil {
			code := ""
			var coded *jeq.Error
			if errors.As(err, &coded) {
				code = string(coded.Code)
			}
			phase := failurePhase(code, err)
			if target.Name() == "gate" {
				phase = "offline_policy"
			}
			cfg.Emit(target.CommandPath(), "run.failed", phase, "failed", code)
		}
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

type (
	tracePhasedError interface{ TracePhase() string }
	phasedError      struct {
		error
		phase string
	}
)

func (e phasedError) TracePhase() string { return e.phase }
func (e phasedError) Unwrap() error      { return e.error }
func withTracePhase(err error, phase string) error {
	if err == nil {
		return nil
	}
	return phasedError{error: err, phase: phase}
}

// publicCLIError keeps wrapped implementation details internal while Cobra
// owns the standard human-facing "Error:" rendering.
func failurePhase(code string, err error) string {
	var phased tracePhasedError
	if errors.As(err, &phased) {
		return phased.TracePhase()
	}
	switch code {
	case string(jeq.CodeInputInvalid):
		return "input_validation"
	case string(jeq.CodeAuthMissing), string(jeq.CodeAuthRejected):
		return "transport"
	case string(jeq.CodeNetworkError), string(jeq.CodeTimeout), string(jeq.CodeInterrupted):
		return "transport"
	case string(jeq.CodeResponseInvalid):

		return "response_validation"
	default:
		return "request_creation"
	}
}

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
