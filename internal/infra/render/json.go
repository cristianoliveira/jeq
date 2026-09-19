// Package render writes gev's machine output: deterministic, lossless
// documents with exactly one trailing newline. It is the only importer of
// encoding details for output and implements the cli.Renderer port.
package render

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/cristianoliveira/gev/internal/domain/contract"
	"github.com/cristianoliveira/gev/internal/domain/gev"
)

// JSON is the D2 interim renderer: lossless JSON documents. TASK-0009 adds
// TOON alongside it; neither changes the document-plus-newline contract.
type JSON struct{}

// RenderSuccess writes the response as exactly one deterministic JSON
// document plus one trailing newline. Unknown server fields ride along
// (the response encodes itself losslessly).
func (JSON) RenderSuccess(w io.Writer, resp contract.Response) error {
	out, err := resp.Encode()
	if err != nil {
		return fmt.Errorf("rendering success document: %w", err)
	}
	return writeJSON(w, out)
}

// RenderValue writes an arbitrary structured command document as JSON.
func (JSON) RenderValue(w io.Writer, value any) error {
	out, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("rendering value: %w", err)
	}
	return writeJSON(w, out)
}

func writeJSON(w io.Writer, out []byte) error {
	if _, err := w.Write(out); err != nil {
		return err
	}
	_, err := w.Write([]byte("\n"))
	return err
}

// errorDoc is the stable error document shape: code, message, recovery.
// The internal cause is deliberately unreachable from here.
type errorDoc struct {
	Code     gev.Code `json:"code"`
	Message  string   `json:"message"`
	Recovery string   `json:"recovery"`
}

// RenderError writes the error document. Only code, message, and recovery
// are exposed: never the internal cause, secrets, stacks, or raw
// dependency prose.
func (JSON) RenderError(w io.Writer, e *gev.Error) error {
	return json.NewEncoder(w).Encode(errorDoc{
		Code:     e.Code,
		Message:  e.Message,
		Recovery: e.Recovery,
	})
}
