package trace

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetailBudgetSuppressesTenThousandOperations(t *testing.T) {
	var out bytes.Buffer
	cfg := New(true, "bounded", &out)
	for i := 0; i < 10000; i++ {
		cfg.EmitOperation("jeq map", "operation.started", "evaluation", "started", "", i+1, 10000)
		cfg.EmitOperation("jeq map", "operation.completed", "evaluation", "success", "", i+1, 10000)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	require.LessOrEqual(t, len(lines), 260)
	assert.Contains(t, out.String(), `"event":"events.suppressed"`)
}
