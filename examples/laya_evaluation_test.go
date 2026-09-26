package examples_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLayaEvaluationHarnessUsesControlledLocalResponses(t *testing.T) {
	result := runScript(t, "examples/laya-evaluation/test.sh", "", "", nil)
	assert.Equal(t, 0, result.exit)
	assert.Equal(t, "PASS laya evaluation harness\n", result.stdout)
	assert.Empty(t, result.stderr)
}
