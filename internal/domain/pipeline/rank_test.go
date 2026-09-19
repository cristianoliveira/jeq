package pipeline

import (
	"encoding/json"
	"testing"

	"github.com/cristianoliveira/jeq/internal/domain/contract"
)

func TestRankStableTieSelectedFirstAndPreservesCandidates(t *testing.T) {
	choice := "b"
	answer := contract.Answer{Type: contract.TypeChoice, Choice: &choice, Probs: map[string]float64{"a": .5, "b": .5}}
	envelope, err := Rank([][]byte{[]byte(`{"id":"a","value":1}`), []byte(`{"id":"b","value":2}`)}, []string{"a", "b"}, answer)
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		Items []RankedItem `json:"items"`
	}
	if json.Unmarshal(envelope, &result) != nil {
		t.Fatal("invalid envelope")
	}
	if result.Items[0].ID != "b" || result.Items[1].ID != "a" || string(result.Items[1].Candidate) != `{"id":"a","value":1}` {
		t.Fatalf("unexpected %#v", result.Items)
	}
}

func TestRankRejectsInvalidDistributions(t *testing.T) {
	cases := []map[string]float64{{"a": -.1, "b": 1.1}, {"a": 1.2, "b": -.2}, {"a": .2, "b": .2}, {"a": .5}}
	for _, probs := range cases {
		choice := "a"
		if _, err := Rank([][]byte{[]byte(`{}`), []byte(`{}`)}, []string{"a", "b"}, contract.Answer{Type: contract.TypeChoice, Choice: &choice, Probs: probs}); err == nil {
			t.Fatalf("accepted %v", probs)
		}
	}
}
