package contract_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/cristianoliveira/jeq/internal/domain/contract"
)

// F-D1-1: semantically empty object/array state must be rejected regardless
// of whitespace; validation compares parsed JSON shape, not byte length.
// ValidateRequest is exercised directly with uncompacted raws — exactly what
// composed mode (jeq.Compose) feeds it.
func TestValidateRequestEmptyShapeIsWhitespaceIndependent(t *testing.T) {
	tests := []struct {
		name      string
		state     string
		wantEmpty bool
	}{
		{"compact empty object", `{}`, true},
		{"pretty empty object", "{\n}", true},
		{"spaced empty object", "{ }", true},
		{"compact empty array", `[]`, true},
		{"pretty empty array", "[\n]", true},
		{"blank string state", `"   "`, true},
		{"empty string state", `""`, true},
		{"nested empty array is still non-empty", "[[]]", false},
		{"object with empty member is still non-empty", `{"a":{}}`, false},
		{"non-empty object", `{"a":1}`, false},
		{"non-empty array", `[1, 2]`, false},
		{"plain string state", `"customer message"`, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := contract.Request{
				Model: "m",
				State: json.RawMessage(tt.state),
				Questions: map[string]contract.Question{
					"q": {Type: contract.TypeNoul, Instructions: json.RawMessage(`"i"`)},
				},
			}

			violations := contract.ValidateRequest(req)
			hasStateViolation := false
			for _, v := range violations {
				if v.Rule == "state_non_empty" {
					hasStateViolation = true
				}
			}

			assert.Equal(t, tt.wantEmpty, hasStateViolation, "state %s; violations=%v", tt.state, ruleNames(violations))
		})
	}
}

func TestEmptyInstructionsShapeAlsoNormalized(t *testing.T) {
	tests := []struct {
		name         string
		instructions string
		wantEmpty    bool
	}{
		{"pretty empty object", "{\n}", true},
		{"pretty empty array", "[\n]", true},
		{"non-empty object", `{"goal":"decide"}`, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := contract.Request{
				Model: "m",
				State: json.RawMessage(`"s"`),
				Questions: map[string]contract.Question{
					"q": {Type: contract.TypeNoul, Instructions: json.RawMessage(tt.instructions)},
				},
			}

			violations := contract.ValidateRequest(req)
			hasInstructionsViolation := false
			for _, v := range violations {
				if v.Rule == "instructions_non_empty" {
					hasInstructionsViolation = true
				}
			}
			assert.Equal(t, tt.wantEmpty, hasInstructionsViolation, "instructions %s; violations=%v", tt.instructions, ruleNames(violations))
		})
	}
}
