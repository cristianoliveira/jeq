package pipeline

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cristianoliveira/jeq/internal/domain/contract"
)

func TestRankStableTieSelectedFirstAndPreservesCandidates(t *testing.T) {
	choice := "b"
	answer := contract.Answer{Type: contract.TypeChoice, Choice: &choice, Probs: map[string]float64{"a": .5, "b": .5}}
	envelope, err := Rank([][]byte{[]byte(`{"id":"a","value":1}`), []byte(`{"id":"b","value":2}`)}, []string{"a", "b"}, answer)
	require.Nil(t, err)
	var result struct {
		Items []RankedItem `json:"items"`
	}
	require.NoError(t, json.Unmarshal(envelope, &result))
	require.Len(t, result.Items, 2)
	assert.Equal(t, "b", result.Items[0].ID)
	assert.Equal(t, "a", result.Items[1].ID)
	assert.Equal(t, `{"id":"a","value":1}`, string(result.Items[1].Candidate))
}

func TestRankRejectsInvalidDistributions(t *testing.T) {
	cases := []map[string]float64{{"a": -.1, "b": 1.1}, {"a": 1.2, "b": -.2}, {"a": .2, "b": .2}, {"a": .5}}
	for _, probs := range cases {
		choice := "a"
		_, err := Rank([][]byte{[]byte(`{}`), []byte(`{}`)}, []string{"a", "b"}, contract.Answer{Type: contract.TypeChoice, Choice: &choice, Probs: probs})
		require.NotNil(t, err)
	}
}
