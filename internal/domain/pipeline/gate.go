package pipeline

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/cristianoliveira/gev/internal/domain/gev"
)

// GatePolicy is an offline probability policy. Values are inclusive at both
// boundaries: pass when value >= PassMin, reject when value <= RejectMax.
type GatePolicy struct {
	PassMin   float64
	RejectMax float64
}

// GateDecision is the stable policy result.
type GateDecision string

const (
	// DecisionPass indicates the value meets the pass threshold.
	DecisionPass GateDecision = "pass"
	// DecisionReject indicates the value meets the reject threshold.
	DecisionReject GateDecision = "reject"
	// DecisionUncertain indicates the value falls between both thresholds.
	DecisionUncertain GateDecision = "uncertain"
)

// Gate appends a numeric policy receipt to a strict object envelope. It never
// calls a model or interprets any model-derived string as an instruction.
func Gate(record []byte, name, pointer string, policy GatePolicy) ([]byte, GateDecision, *gev.Error) {
	members, err := decodeObject(record, "record")
	if err != nil {
		return nil, "", inputError(err.Error())
	}
	if err := validateName(name); err != nil {
		return nil, "", inputError(err.Error())
	}
	if err := validatePolicy(policy); err != nil {
		return nil, "", inputError(err.Error())
	}
	gevMembers, err := existingEvidence(members)
	if err != nil {
		return nil, "", inputError(err.Error())
	}
	if _, exists := gevMembers[name]; exists {
		return nil, "", inputError(fmt.Sprintf("_gev.%s already exists; choose a new name", name))
	}
	selected, err := Select(record, pointer)
	if err != nil {
		return nil, "", inputError(err.Error())
	}
	value, err := numericValue(selected)
	if err != nil {
		return nil, "", inputError(fmt.Sprintf("value pointer %q: %s", pointer, err))
	}
	if value < 0 || value > 1 {
		return nil, "", inputError(fmt.Sprintf("value pointer %q must select a number between 0 and 1", pointer))
	}
	decision := DecisionUncertain
	if value >= policy.PassMin {
		decision = DecisionPass
	} else if value <= policy.RejectMax {
		decision = DecisionReject
	}
	receipt := map[string]any{
		"decision": string(decision),
		"value":    value,
		"policy": map[string]float64{
			"pass_min":   policy.PassMin,
			"reject_max": policy.RejectMax,
		},
	}
	receiptRaw, _ := json.Marshal(receipt)
	gevMembers[name] = receiptRaw
	members["_gev"], _ = json.Marshal(gevMembers)
	result, marshalErr := json.Marshal(members)
	if marshalErr != nil {
		return nil, "", gev.WrapError(gev.CodeResponseInvalid, marshalErr, "encoding gated record")
	}
	return result, decision, nil
}

func validatePolicy(policy GatePolicy) error {
	if policy.RejectMax < 0 || policy.PassMin < 0 || policy.RejectMax >= policy.PassMin || policy.PassMin > 1 {
		return fmt.Errorf("thresholds must satisfy 0 <= reject-max < pass-min <= 1")
	}
	return nil
}

func numericValue(raw []byte) (float64, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) || bytes.Equal(trimmed, []byte("true")) || bytes.Equal(trimmed, []byte("false")) || trimmed[0] == '"' || trimmed[0] == '{' || trimmed[0] == '[' {
		return 0, fmt.Errorf("must select a JSON number")
	}
	value, err := strconv.ParseFloat(string(trimmed), 64)
	if err != nil {
		return 0, fmt.Errorf("must select a JSON number")
	}
	return value, nil
}
