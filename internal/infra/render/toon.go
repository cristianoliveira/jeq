package render

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"reflect"
	"strconv"

	"github.com/cristianoliveira/gev/internal/domain/contract"
	"github.com/cristianoliveira/gev/internal/domain/gev"
	toon "github.com/toon-format/toon-go"
)

// TOON is the default D3 renderer. It encodes the canonical JSON document,
// decodes it again, and rejects any semantic loss before writing output.
type TOON struct{}

// RenderSuccess writes a lossless TOON response document plus one newline.
func (TOON) RenderSuccess(w io.Writer, resp contract.Response) error {
	raw, err := resp.Encode()
	if err != nil {
		return fmt.Errorf("encoding success document: %w", err)
	}
	return writeTOON(w, raw)
}

// RenderValue writes a structured non-response document as TOON.
func (TOON) RenderValue(w io.Writer, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("encoding value: %w", err)
	}
	return writeTOON(w, raw)
}

// RenderError writes a lossless TOON error document plus one newline.
func (TOON) RenderError(w io.Writer, e *gev.Error) error {
	doc := struct {
		Code     gev.Code `json:"code"`
		Message  string   `json:"message"`
		Recovery string   `json:"recovery"`
	}{e.Code, e.Message, e.Recovery}
	raw, err := json.Marshal(doc)
	if err != nil {
		return fmt.Errorf("encoding error document: %w", err)
	}
	return writeTOON(w, raw)
}

func writeTOON(w io.Writer, raw []byte) error {
	value, err := decodeJSON(raw)
	if err != nil {
		return fmt.Errorf("TOON input is not JSON: %w", err)
	}
	encoded, err := toon.Marshal(value)
	if err != nil {
		return fmt.Errorf("encoding TOON: %w", err)
	}
	decoded, err := toon.Decode(encoded)
	if err != nil {
		return fmt.Errorf("TOON round-trip decode: %w", err)
	}
	if !semanticEqual(value, decoded) {
		return fmt.Errorf("TOON round-trip changed document semantics")
	}
	if _, err := w.Write(encoded); err != nil {
		return err
	}
	_, err = io.WriteString(w, "\n")
	return err
}

func decodeJSON(raw []byte) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var value any
	if err := dec.Decode(&value); err != nil {
		return nil, err
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("trailing JSON value")
		}
		return nil, err
	}
	return value, nil
}

func semanticEqual(a, b any) bool {
	switch av := a.(type) {
	case nil:
		return b == nil
	case bool:
		bv, ok := b.(bool)
		return ok && av == bv
	case string:
		bv, ok := b.(string)
		return ok && av == bv
	case json.Number:
		return equalNumber(av.String(), b)
	case []any:
		bv, ok := b.([]any)
		if !ok || len(av) != len(bv) {
			return false
		}
		for i := range av {
			if !semanticEqual(av[i], bv[i]) {
				return false
			}
		}
		return true
	case map[string]any:
		bv, ok := b.(map[string]any)
		if !ok || len(av) != len(bv) {
			return false
		}
		for key, value := range av {
			if !semanticEqual(value, bv[key]) {
				return false
			}
		}
		return true
	default:
		return reflect.DeepEqual(a, b)
	}
}

func equalNumber(source string, decoded any) bool {
	var decodedText string
	switch value := decoded.(type) {
	case float64:
		decodedText = strconv.FormatFloat(value, 'g', -1, 64)
	case json.Number:
		decodedText = value.String()
	default:
		return false
	}
	a, ok := new(big.Rat).SetString(source)
	if !ok {
		return false
	}
	b, ok := new(big.Rat).SetString(decodedText)
	return ok && a.Cmp(b) == 0
}
