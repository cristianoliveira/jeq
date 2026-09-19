package pipeline

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/cristianoliveira/jeq/internal/domain/contract"
	"github.com/cristianoliveira/jeq/internal/domain/jeq"
)

// Reduce evaluates one bounded collection as one request and returns a stable
// envelope. It is deliberately not an iterative fold: one input collection,
// one request, one response.
func Reduce(ctx context.Context, items []byte, name string, request contract.Request, evaluator Evaluator) ([]byte, *jeq.Error) {
	if err := validateName(name); err != nil {
		return nil, inputError(err.Error())
	}
	if err := contract.ValidateJSON(items); err != nil {
		return nil, inputError(err.Error())
	}
	trimmed := bytes.TrimSpace(items)
	if len(trimmed) == 0 || trimmed[0] != '[' {
		return nil, inputError("reduce items must be a JSON array")
	}
	var values []json.RawMessage
	if err := json.Unmarshal(trimmed, &values); err != nil {
		return nil, inputError(err.Error())
	}
	if len(values) == 0 {
		return nil, inputError("reduce collection is empty")
	}
	request.State = append(json.RawMessage(nil), trimmed...)
	if request.Model == "" || evaluator == nil {
		return nil, inputError("model and evaluator are required")
	}
	if violations := contract.ValidateRequest(request); len(violations) > 0 {
		return nil, violations[0].Error
	}
	response, evalErr := evaluator.Evaluate(ctx, request)
	if evalErr != nil {
		return nil, evalErr
	}
	encoded, encodeErr := response.Encode()
	if encodeErr != nil {
		return nil, jeq.WrapError(jeq.CodeResponseInvalid, encodeErr, "encoding evaluator response")
	}
	nameJSON, _ := json.Marshal(name)
	var output bytes.Buffer
	output.WriteString(`{"items":`)
	output.Write(trimmed)
	output.WriteString(`,"_jeq":{`)
	output.Write(nameJSON)
	output.WriteByte(':')
	output.Write(encoded)
	output.WriteString("}}")
	if !json.Valid(output.Bytes()) {
		return nil, jeq.NewError(jeq.CodeResponseInvalid, fmt.Sprintf("encoding reduce envelope %q", name))
	}
	return output.Bytes(), nil
}
