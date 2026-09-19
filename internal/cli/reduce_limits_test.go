package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestReduceCollectionBoundsRejectEmptyAndExcessItems(t *testing.T) {
	if _, err := reduceItems([]byte(`[]`), "json"); err == nil {
		t.Fatal("empty JSON collection must fail")
	}
	var lines bytes.Buffer
	for i := 0; i <= ReduceMaxItems; i++ {
		lines.WriteString("1\n")
	}
	if _, err := reduceItems(lines.Bytes(), "ndjson"); err == nil {
		t.Fatal("excess NDJSON items must fail")
	}
	if _, err := reduceItems([]byte(`[{"x":1,"x":2}]`), "json"); err == nil {
		t.Fatal("recursive duplicate keys must fail")
	}
	for _, tc := range []struct{ name, framing, input string }{
		{"bad framing", "yaml", `[]`},
		{"non array", "json", `{"x":1}`},
		{"malformed item", "ndjson", `{"x":`},
		{"item byte limit", "ndjson", strings.Repeat("x", MapMaxRecordBytes+1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := reduceItems([]byte(tc.input), tc.framing); err == nil {
				t.Fatal("invalid collection accepted")
			}
		})
	}
}
