package cli

import (
	"errors"
	"io"

	"github.com/cristianoliveira/jeq/internal/trace"
	"github.com/spf13/cobra"

	"github.com/cristianoliveira/jeq/internal/domain/contract"
	"github.com/cristianoliveira/jeq/internal/domain/jeq"
)

// CodedError is the shell-facing alias for stable domain failures. It keeps
// the composition root from importing the domain package directly.
type CodedError = jeq.Error

type traceAttachable interface{ SetTraceObserver(trace.Observer) }

func beginLifecycle(cmd *cobra.Command) func(error) {
	cfg := trace.FromContext(cmd.Context())
	if cfg != nil {
		cfg.Emit(cmd.CommandPath(), "preflight.completed", "preflight", "success", "")
	}
	return func(err error) {
		if cfg != nil {
			if err != nil {
				cfg.Emit(cmd.CommandPath(), "run.failed", "execution", "failed", func() string {
					var coded *jeq.Error
					if errors.As(err, &coded) {
						return string(coded.Code)
					}
					return ""
				}())
			} else {
				cfg.Emit(cmd.CommandPath(), "output.written", "output", "success", "")
			}
		}
	}
}

func attachTrace(cmd *cobra.Command, client APIClient) {
	if observer, ok := client.(traceAttachable); ok {
		if cfg := trace.FromContext(cmd.Context()); cfg != nil {
			observer.SetTraceObserver(cfg)
		}
	}
}

// Renderer emits successful machine result documents. Human output and errors
// stay at the Cobra boundary; infra/render satisfies this result-only port.
type Renderer interface {
	RenderSuccess(w io.Writer, resp contract.Response) error
}

// ValueRenderer renders structured map/reduce/gate result values.
type ValueRenderer interface {
	Renderer
	RenderValue(w io.Writer, value any) error
}

// RawRenderer writes a validated JSON document without decoding numeric
// lexemes into floating point values.
type RawRenderer interface {
	Renderer
	RenderRaw(w io.Writer, raw []byte) error
}
