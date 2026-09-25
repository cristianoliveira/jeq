package cli

import (
	"io"
	"sort"
	"time"

	"github.com/cristianoliveira/jeq/internal/trace"
	"github.com/spf13/cobra"

	"github.com/cristianoliveira/jeq/internal/domain/contract"
	"github.com/cristianoliveira/jeq/internal/domain/jeq"
)

// CodedError is the shell-facing alias for stable domain failures. It keeps
// the composition root from importing the domain package directly.
type CodedError = jeq.Error

type traceAttachable interface{ SetTraceObserver(trace.Observer) }

func traceMetadata(cmd *cobra.Command, model, source, framing, pointer string, questions map[string]contract.Question) {
	if cfg := trace.FromContext(cmd.Context()); cfg != nil {
		names := make([]string, 0, len(questions))
		for name := range questions {
			names = append(names, name)
		}
		sort.Strings(names)
		types := make([]string, 0, len(names))
		for _, name := range names {
			types = append(types, string(questions[name].Type))
		}
		cfg.EmitMetadata(cmd.CommandPath(), model, source, framing, pointer, names, types)
	}
}

func newClient(deps AskDeps, provider ResolvedProvider, timeout time.Duration, maxRetries int, diagnostic func(string)) APIClient {
	if deps.NewProfileClient != nil {
		return deps.NewProfileClient(provider.BaseURL, timeout, provider.APIKey, provider.Auth, maxRetries, diagnostic)
	}
	return deps.NewClient(provider.BaseURL, timeout, provider.APIKey, maxRetries, diagnostic)
}

func attachTrace(cmd *cobra.Command, client APIClient) {
	observer, ok := client.(traceAttachable)
	if !ok {
		return
	}
	observers := make([]trace.Observer, 0, 2)
	if cfg := trace.FromContext(cmd.Context()); cfg != nil {
		observers = append(observers, cfg)
	}
	if summary := usageSummaryFrom(cmd.Context()); summary != nil {
		observers = append(observers, summary)
	}
	switch len(observers) {
	case 1:
		observer.SetTraceObserver(observers[0])
	case 2:
		observer.SetTraceObserver(combinedTraceObserver(observers))
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
