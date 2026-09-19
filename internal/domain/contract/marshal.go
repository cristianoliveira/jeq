package contract

import (
	"encoding/json"
)

// MarshalJSON implementations keep re-encoding lossless and deterministic:
// known fields render from typed values, unknown fields verbatim, and
// absent optional fields stay absent.

// MarshalJSON renders a question with its unknown fields in place.
func (q Question) MarshalJSON() ([]byte, error) {
	known := map[string]json.RawMessage{
		"type": mustMarshal(q.Type),
	}
	if len(q.Instructions) > 0 {
		known["instructions"] = q.Instructions
	}
	if len(q.Criteria) > 0 {
		known["criteria"] = q.Criteria
	}
	return encodeDoc(known, q.Extra)
}

// MarshalJSON renders an answer, omitting fields the answer type does not
// carry and preserving unknown server fields.
func (a Answer) MarshalJSON() ([]byte, error) {
	known := map[string]json.RawMessage{
		"type": mustMarshal(a.Type),
	}
	put := func(key string, v any) {
		known[key] = mustMarshal(v)
	}
	if a.Noul != nil {
		put("noul", a.Noul)
	}
	if a.Choice != nil {
		put("choice", a.Choice)
	}
	if a.Score != nil {
		put("score", a.Score)
	}
	if a.Confidence != nil {
		put("confidence", a.Confidence)
	}
	if a.Probs != nil {
		put("probabilities", a.Probs)
	}
	if a.Legend != nil {
		put("legend", a.Legend)
	}
	return encodeDoc(known, a.Extra)
}

// MarshalJSON renders the usage object verbatim plus the typed counters.
func (u Usage) MarshalJSON() ([]byte, error) {
	return json.Marshal(u.encodeFields())
}

// MarshalJSON renders a model entry, omitting absent optional fields.
func (i ModelInfo) MarshalJSON() ([]byte, error) {
	known := map[string]json.RawMessage{}
	put := func(key string, v any) {
		known[key] = mustMarshal(v)
	}
	if i.Name != "" {
		put("name", i.Name)
	}
	if i.Description != "" {
		put("description", i.Description)
	}
	if i.ReleaseDate != "" {
		put("release_date", i.ReleaseDate)
	}
	return encodeDoc(known, i.Extra)
}
