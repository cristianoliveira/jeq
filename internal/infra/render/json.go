// Package render writes gev's machine output: deterministic, lossless
// documents with exactly one trailing newline. It is the only importer of
// encoding details for output and implements the cli.Renderer port.
package render

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/cristianoliveira/gev/internal/domain/contract"
)

// JSON is the version 1 renderer: lossless JSON documents with the
// document-plus-newline contract.
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

// RenderRaw preserves a command's already validated JSON document.
func (JSON) RenderRaw(w io.Writer, raw []byte) error { return writeJSON(w, raw) }

func writeJSON(w io.Writer, out []byte) error {
	if _, err := w.Write(out); err != nil {
		return err
	}
	_, err := w.Write([]byte("\n"))
	return err
}
