package pipeline

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/cristianoliveira/gev/internal/domain/contract"
)

func TestReduceUsesItemsAsTheRequestStateInvariant(t *testing.T) {
	evaluator := &reduceEvaluator{}
	items := json.RawMessage(`[1,2]`)
	request := contract.Request{Model: "m", State: json.RawMessage(`[999]`), Questions: map[string]contract.Question{"q": {Type: contract.TypeNoul, Instructions: json.RawMessage(`"judge"`)}}}
	if _, err := Reduce(context.Background(), items, "batch", request, evaluator); err != nil {
		t.Fatal(err)
	}
	if string(evaluator.request.State) != string(items) {
		t.Fatalf("state=%s want=%s", evaluator.request.State, items)
	}
}
