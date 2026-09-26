package pipeline

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cristianoliveira/jeq/internal/domain/jeq"
)

func TestGateAppendsDecisionAndPreservesEvidence(t *testing.T) {
	for _, tc := range []struct {
		name, record, pointer string
		want                  GateDecision
	}{
		{"pass", `{"score":0.9,"_jeq":{"first":{"kept":true}}}`, "/score", DecisionPass},
		{"reject", `{"score":0.1}`, "/score", DecisionReject},
		{"uncertain", `{"score":0.5}`, "/score", DecisionUncertain},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, decision, err := Gate([]byte(tc.record), "policy", tc.pointer, GatePolicy{PassMin: 0.8, RejectMax: 0.2})
			require.Nil(t, err)
			assert.Equal(t, tc.want, decision)
			var doc map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(out, &doc))
			var evidence map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(doc["_jeq"], &evidence))
			assert.Contains(t, evidence, "policy")
			if tc.name == "pass" {
				assert.Contains(t, evidence, "first")
			}
		})
	}
}

func TestGateRejectsInvalidInputBeforeDecision(t *testing.T) {
	cases := []struct {
		name, record, pointer string
		policy                GatePolicy
	}{
		{"collision", `{"value":0.9,"_jeq":{"policy":{}}}`, "/value", GatePolicy{PassMin: .8, RejectMax: .2}},
		{"nonnumeric", `{"value":"0.9"}`, "/value", GatePolicy{PassMin: .8, RejectMax: .2}},
		{"out of range", `{"value":2}`, "/value", GatePolicy{PassMin: .8, RejectMax: .2}},
		{"bad thresholds", `{"value":0.9}`, "/value", GatePolicy{PassMin: .2, RejectMax: .2}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := Gate([]byte(tc.record), "policy", tc.pointer, tc.policy)
			require.NotNil(t, err)
			assert.Equal(t, jeq.CodeInputInvalid, err.Code)
		})
	}
}
