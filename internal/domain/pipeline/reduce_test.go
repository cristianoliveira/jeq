package pipeline

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cristianoliveira/jeq/internal/domain/contract"
	"github.com/cristianoliveira/jeq/internal/domain/jeq"
)

type reduceEvaluator struct {
	calls   int
	request contract.Request
	err     *jeq.Error
}

func (e *reduceEvaluator) Evaluate(_ context.Context, request contract.Request) (contract.Response, *jeq.Error) {
	e.calls++
	e.request = request
	if e.err != nil {
		return contract.Response{}, e.err
	}
	return contract.Response{Model: "m", Answers: map[string]contract.Answer{}, Usage: contract.Usage{}, Extra: map[string]json.RawMessage{"extra": json.RawMessage(`true`)}}, nil
}

func TestReduceSendsOneExactCollectionAndPreservesResponseExtras(t *testing.T) {
	e := &reduceEvaluator{}
	items := []byte(`[1e400,{"x":1}]`)
	out, err := Reduce(context.Background(), items, "batch", contract.Request{Model: "m", State: items, Questions: map[string]contract.Question{"q": {Type: contract.TypeNoul, Instructions: json.RawMessage(`"judge"`)}}}, e)
	require.Nil(t, err)
	assert.Equal(t, 1, e.calls)
	assert.Equal(t, string(items), string(e.request.State))
	assert.NotEmpty(t, out)
	assert.True(t, containsBytes(out, []byte(`"extra":true`)))
}

func TestReduceRejectsEmptyCollectionAndEvaluatorErrors(t *testing.T) {
	t.Run("empty collection fails without calling evaluator", func(t *testing.T) {
		e := &reduceEvaluator{}
		request := contract.Request{Model: "m", State: json.RawMessage(`[]`), Questions: map[string]contract.Question{"q": {Type: contract.TypeNoul, Instructions: json.RawMessage(`"judge"`)}}}
		_, err := Reduce(context.Background(), []byte(`[]`), "batch", request, e)
		require.NotNil(t, err)
		assert.Zero(t, e.calls)
	})

	t.Run("evaluator error is returned unchanged", func(t *testing.T) {
		evalErr := jeq.NewError(jeq.CodeServerError, "boom")
		e := &reduceEvaluator{err: evalErr}
		request := contract.Request{Model: "m", State: json.RawMessage(`[1]`), Questions: map[string]contract.Question{"q": {Type: contract.TypeNoul, Instructions: json.RawMessage(`"judge"`)}}}
		_, err := Reduce(context.Background(), []byte(`[1]`), "batch", request, e)
		assert.Same(t, evalErr, err)
		assert.Equal(t, 1, e.calls)
	})
}

func containsBytes(data, needle []byte) bool {
	for i := 0; i+len(needle) <= len(data); i++ {
		if string(data[i:i+len(needle)]) == string(needle) {
			return true
		}
	}
	return false
}
