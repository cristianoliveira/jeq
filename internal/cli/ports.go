package cli

import (
	"io"

	"github.com/cristianoliveira/gev/internal/domain/contract"
	"github.com/cristianoliveira/gev/internal/domain/gev"
)

// CodedError is the shell-facing alias for stable domain failures. It keeps
// the composition root from importing the domain package directly.
type CodedError = gev.Error

// Renderer emits the machine output contract: exactly one deterministic
// document plus one trailing newline, per stream. infra/render satisfies it
// implicitly; cli never imports infra (ADR 0002 ports live with consumers).
type Renderer interface {
	RenderSuccess(w io.Writer, resp contract.Response) error
	RenderError(w io.Writer, e *gev.Error) error
}

// ValueRenderer renders command documents that are not TypeSafe responses,
// such as home, version, and models.
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
