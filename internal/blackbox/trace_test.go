package blackbox_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBlackBoxVerboseKeepsStdoutSeparateAndCorrelates(t *testing.T) {
	off := runBinary(t, "", nil, "version")
	on := runBinary(t, "", map[string]string{"JEQ_TRACE_ID": "chain-42"}, "--verbose", "version")
	require.Equal(t, 0, off.exit)
	require.Equal(t, 0, on.exit)
	assert.Equal(t, off.stdout, on.stdout, "stdout must be unchanged")
	for _, field := range []string{`"schema":"jeq.trace.v1"`, `"trace_id":"chain-42"`, `"event":"run.started"`, `"event":"run.completed"`} {
		assert.Contains(t, on.stderr, field)
	}
	assert.NotContains(t, on.stdout, "jeq.trace.v1")
}
