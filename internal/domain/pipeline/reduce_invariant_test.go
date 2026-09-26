package pipeline

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cristianoliveira/jeq/internal/domain/contract"
)

func TestReduceUsesItemsAsTheRequestStateInvariant(t *testing.T) {
	evaluator := &reduceEvaluator{}
	items := json.RawMessage(`[1,2]`)
	request := contract.Request{Model: "m", State: json.RawMessage(`[999]`), Questions: map[string]contract.Question{"q": {Type: contract.TypeNoul, Instructions: json.RawMessage(`"judge"`)}}}
	_, err := Reduce(context.Background(), items, "batch", request, evaluator)
	require.Nil(t, err)
	assert.Equal(t, string(items), string(evaluator.request.State))
}
