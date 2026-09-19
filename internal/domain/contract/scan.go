package contract

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"unicode/utf8"
)

// scanDoc performs the raw-document checks that encoding/json cannot:
// valid UTF-8, no BOM, well-formed JSON, no duplicate keys, no trailing
// data. It returns a plain *scanError; callers wrap it with the stable code
// for their document kind.
func scanDoc(data []byte) error {
	if !utf8.Valid(data) {
		return fmt.Errorf("document is not valid UTF-8")
	}
	if bytes.HasPrefix(data, []byte("\xEF\xBB\xBF")) {
		return fmt.Errorf("document starts with a UTF-8 BOM; strip it before decoding")
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
			return fmt.Errorf("malformed JSON at byte %d: %v", dec.InputOffset(), err)
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
					return fmt.Errorf("malformed JSON: unexpected closing delimiter at byte %d", dec.InputOffset())
				}
				stack = stack[:len(stack)-1]
				closeValue()
			}
		default:
			if len(stack) == 0 {
				return fmt.Errorf("malformed JSON: value outside a document at byte %d", dec.InputOffset())
			}
			top := stack[len(stack)-1]
			switch {
			case top.object && top.expectKey:
				key, ok := tok.(string)
				if !ok {
					return fmt.Errorf("malformed JSON: non-string object key at byte %d", dec.InputOffset())
				}
				if _, dup := top.keys[key]; dup {
					return fmt.Errorf("duplicate object key %q at byte %d", key, dec.InputOffset())
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
		return fmt.Errorf("malformed JSON: unclosed container")
	}
	return nil
}
