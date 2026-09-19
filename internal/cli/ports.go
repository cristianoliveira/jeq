package cli

import (
	"io"

	"github.com/cristianoliveira/gev/internal/domain/contract"
	"github.com/cristianoliveira/gev/internal/domain/gev"
)

// Renderer emits the machine output contract: exactly one deterministic
// document plus one trailing newline, per stream. infra/render satisfies it
// implicitly; cli never imports infra (ADR 0002 ports live with consumers).
type Renderer interface {
	RenderSuccess(w io.Writer, resp contract.Response) error
	RenderError(w io.Writer, e *gev.Error) error
}
