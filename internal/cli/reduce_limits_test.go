package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReduceCollectionBoundsRejectEmptyAndExcessItems(t *testing.T) {
	_, err := reduceItems([]byte(`[]`), "json")
	require.NotNil(t, err, "empty JSON collection must fail")
	var lines bytes.Buffer
	for i := 0; i <= ReduceMaxItems; i++ {
		lines.WriteString("1\n")
	}
	_, err = reduceItems(lines.Bytes(), "ndjson")
	require.NotNil(t, err, "excess NDJSON items must fail")
	_, err = reduceItems([]byte(`[{"x":1,"x":2}]`), "json")
	require.NotNil(t, err, "recursive duplicate keys must fail")
	for _, tc := range []struct{ name, framing, input string }{
		{"bad framing", "yaml", `[]`},
		{"non array", "json", `{"x":1}`},
		{"malformed item", "ndjson", `{"x":`},
		{"item byte limit", "ndjson", strings.Repeat("x", MapMaxRecordBytes+1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := reduceItems([]byte(tc.input), tc.framing)
			require.NotNil(t, err, "invalid collection accepted")
		})
	}
}
