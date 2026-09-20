package trace

import (
	"bytes"
	"strings"
	"testing"
)

func TestMetadataEmissionIsAllowlistedAndDeterministic(t *testing.T) {
	var out bytes.Buffer
	c := New(true, "id", &out)
	c.EmitMetadata("jeq map", "m", "config", "ndjson", "/state", []string{"a", "b"}, []string{"noul", "score"})
	line := out.String()
	if !strings.Contains(line, `"question_names":["a","b"]`) || strings.Contains(line, "prompt") {
		t.Fatal(line)
	}
}
