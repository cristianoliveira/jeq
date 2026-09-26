package trace

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMetadataEmissionIsAllowlistedAndDeterministic(t *testing.T) {
	var out bytes.Buffer
	c := New(true, "id", &out)
	c.EmitMetadata("jeq map", "m", "config", "ndjson", "/state", []string{"a", "b"}, []string{"noul", "score"})
	line := out.String()
	assert.Contains(t, line, `"question_names":["a","b"]`)
	assert.NotContains(t, line, "prompt")
}
