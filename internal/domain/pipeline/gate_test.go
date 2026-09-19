package pipeline

import (
	"encoding/json"
	"testing"

	"github.com/cristianoliveira/gev/internal/domain/gev"
)

func TestGateAppendsDecisionAndPreservesEvidence(t *testing.T) {
	for _, tc := range []struct {
		name, record, pointer string
		want                  GateDecision
	}{
		{"pass", `{"score":0.9,"_gev":{"first":{"kept":true}}}`, "/score", DecisionPass},
		{"reject", `{"score":0.1}`, "/score", DecisionReject},
		{"uncertain", `{"score":0.5}`, "/score", DecisionUncertain},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, decision, err := Gate([]byte(tc.record), "policy", tc.pointer, GatePolicy{PassMin: 0.8, RejectMax: 0.2})
			if err != nil || decision != tc.want {
				t.Fatalf("decision=%s err=%v", decision, err)
			}
			var doc map[string]json.RawMessage
			if err := json.Unmarshal(out, &doc); err != nil {
				t.Fatal(err)
			}
			var evidence map[string]json.RawMessage
			if err := json.Unmarshal(doc["_gev"], &evidence); err != nil {
				t.Fatal(err)
			}
			if evidence["policy"] == nil {
				t.Fatal("missing policy evidence")
			}
			if tc.name == "pass" && evidence["first"] == nil {
				t.Fatal("existing evidence was dropped")
			}
		})
	}
}

func TestGateRejectsInvalidInputBeforeDecision(t *testing.T) {
	cases := []struct {
		name, record, pointer string
		policy                GatePolicy
	}{
		{"collision", `{"value":0.9,"_gev":{"policy":{}}}`, "/value", GatePolicy{PassMin: .8, RejectMax: .2}},
		{"nonnumeric", `{"value":"0.9"}`, "/value", GatePolicy{PassMin: .8, RejectMax: .2}},
		{"out of range", `{"value":2}`, "/value", GatePolicy{PassMin: .8, RejectMax: .2}},
		{"bad thresholds", `{"value":0.9}`, "/value", GatePolicy{PassMin: .2, RejectMax: .2}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, err := Gate([]byte(tc.record), "policy", tc.pointer, tc.policy); err == nil || err.Code != gev.CodeInputInvalid {
				t.Fatalf("err=%v", err)
			}
		})
	}
}
