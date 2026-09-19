package pipeline

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"

	"github.com/cristianoliveira/jeq/internal/domain/contract"
	"github.com/cristianoliveira/jeq/internal/domain/jeq"
)

// ProbabilityTolerance permits small floating-point serialization drift around a unit sum.
const ProbabilityTolerance = 1e-6

// RankedItem is one candidate and its relative Choice probability.
type RankedItem struct {
	ID          string          `json:"id"`
	Probability float64         `json:"probability"`
	Candidate   json.RawMessage `json:"candidate"`
}

// Rank validates a complete Choice distribution and returns a deterministic envelope.
func Rank(records [][]byte, ids []string, answer contract.Answer) ([]byte, *jeq.Error) {
	if answer.Choice == nil {
		return nil, jeq.NewError(jeq.CodeResponseInvalid, "Choice response is missing the selected id")
	}
	if len(records) != len(ids) || len(answer.Probs) != len(ids) {
		return nil, jeq.NewError(jeq.CodeResponseInvalid, "Choice probabilities must contain every candidate exactly once")
	}
	known := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		if _, ok := known[id]; ok {
			return nil, jeq.NewError(jeq.CodeResponseInvalid, fmt.Sprintf("duplicate candidate id %q", id))
		}
		known[id] = struct{}{}
	}
	if _, ok := known[*answer.Choice]; !ok {
		return nil, jeq.NewError(jeq.CodeResponseInvalid, fmt.Sprintf("Choice selected unknown candidate id %q", *answer.Choice))
	}
	items := make([]RankedItem, len(records))
	sum := 0.0
	for i, id := range ids {
		probability, ok := answer.Probs[id]
		if !ok {
			return nil, jeq.NewError(jeq.CodeResponseInvalid, "Choice probabilities must contain every candidate exactly once")
		}
		if math.IsNaN(probability) || math.IsInf(probability, 0) || probability < 0 || probability > 1 {
			return nil, jeq.NewError(jeq.CodeResponseInvalid, "Choice probabilities must be finite values in [0,1]")
		}
		sum += probability
		items[i] = RankedItem{ID: id, Probability: probability, Candidate: append(json.RawMessage(nil), records[i]...)}
	}
	if math.Abs(sum-1) > ProbabilityTolerance {
		return nil, jeq.NewError(jeq.CodeResponseInvalid, "Choice probabilities must sum to 1")
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Probability != items[j].Probability {
			return items[i].Probability > items[j].Probability
		}
		if items[i].ID == *answer.Choice {
			return true
		}
		if items[j].ID == *answer.Choice {
			return false
		}
		return false
	})
	maxProbability := items[0].Probability
	count := 0
	for _, item := range items {
		if item.Probability == maxProbability {
			count++
		}
	}
	if count == 1 && items[0].ID != *answer.Choice {
		return nil, jeq.NewError(jeq.CodeResponseInvalid, "Choice selected id is not the highest-probability candidate")
	}
	envelope, err := json.Marshal(map[string]any{"items": items})
	if err != nil {
		return nil, jeq.WrapError(jeq.CodeResponseInvalid, err, "encoding rank envelope")
	}
	return envelope, nil
}
