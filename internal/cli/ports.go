package cli

import (
	"io"

	"github.com/cristianoliveira/gev/internal/domain/contract"
	"github.com/cristianoliveira/gev/internal/domain/gev"
)

// CodedError is the shell-facing alias for stable domain failures. It keeps
// the composition root from importing the domain package directly.
type CodedError = gev.Error

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
