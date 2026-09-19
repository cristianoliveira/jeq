package contract

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"unicode/utf8"
)

// scanError distinguishes syntax failures from duplicate-key failures so
// each can map to its own local rule.
type scanError struct {
	duplicate bool
	msg       string
}

func (e *scanError) Error() string { return e.msg }

// ValidateJSON performs the strict raw-document checks shared by CLI config and documents.
func ValidateJSON(data []byte) error { return scanDoc(data) }

// scanDoc performs the raw-document checks that encoding/json cannot:
// valid UTF-8, no BOM, well-formed JSON, no duplicate keys, no trailing
// data. It returns a *scanError; callers wrap it with the stable code for
// their document kind.
func scanDoc(data []byte) error {
	if !utf8.Valid(data) {
		return &scanError{msg: "document is not valid UTF-8"}
	}
	if bytes.HasPrefix(data, []byte("\xEF\xBB\xBF")) {
		return &scanError{msg: "document starts with a UTF-8 BOM; strip it before decoding"}
	}

	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()

	// Frame tracks one open container while walking the token stream.
	type frame struct {
		object    bool
		keys      map[string]struct{}
		expectKey bool // object only: next string token is a key
	}

	var stack []*frame
	closeValue := func() {
		if len(stack) > 0 && stack[len(stack)-1].object {
			stack[len(stack)-1].expectKey = true
		}
	}

	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return &scanError{msg: fmt.Sprintf("malformed JSON at byte %d: %v", dec.InputOffset(), err)}
		}

		switch t := tok.(type) {
		case json.Delim:
			switch t {
			case '{':
				stack = append(stack, &frame{object: true, keys: map[string]struct{}{}, expectKey: true})
			case '[':
				stack = append(stack, &frame{})
			default: // '}' or ']'
				if len(stack) == 0 {
					return &scanError{msg: fmt.Sprintf("malformed JSON: unexpected closing delimiter at byte %d", dec.InputOffset())}
				}
				stack = stack[:len(stack)-1]
				closeValue()
			}
		default:
			if len(stack) == 0 {
				return &scanError{msg: fmt.Sprintf("malformed JSON: value outside a document at byte %d", dec.InputOffset())}
			}
			top := stack[len(stack)-1]
			switch {
			case top.object && top.expectKey:
				key, ok := tok.(string)
				if !ok {
					return fmt.Errorf("malformed JSON: non-string object key at byte %d", dec.InputOffset())
				}
				if _, dup := top.keys[key]; dup {
					return &scanError{duplicate: true, msg: fmt.Sprintf("duplicate object key %q at byte %d", key, dec.InputOffset())}
				}
				top.keys[key] = struct{}{}
				top.expectKey = false
			case !top.object:
				// array element: nothing to track
			default:
				closeValue() // scalar value inside an object
			}
		}
	}

	if len(stack) != 0 {
		return &scanError{msg: "malformed JSON: unclosed container"}
	}
	return nil
}
