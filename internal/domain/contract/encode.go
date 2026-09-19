package contract

import "encoding/json"

// Encode renders the document back to JSON deterministically, merging the
// known fields with every unknown field collected at decode time. Key order
// is canonical (sorted), so encode is a pure function of the decoded value.
func (r Request) Encode() ([]byte, error) {
	known := map[string]json.RawMessage{}
	if r.Model != "" {
		known["model"] = mustMarshal(r.Model)
	}
	if r.State != nil {
		known["state"] = r.State
	}
	if r.Questions != nil {
		known["questions"] = mustMarshal(r.Questions)
	}
	return encodeDoc(known, r.Extra)
}

// Encode renders the response losslessly, including unknown server fields.
func (r Response) Encode() ([]byte, error) {
	known := map[string]json.RawMessage{
		"model":   mustMarshal(r.Model),
		"answers": mustMarshal(r.Answers),
		"usage":   mustMarshal(r.Usage.encodeFields()),
	}
	return encodeDoc(known, r.Extra)
}

// Encode renders the models document, unknown fields included.
func (m Models) Encode() ([]byte, error) {
	known := map[string]json.RawMessage{
		"models": mustMarshal(m.Models),
	}
	return encodeDoc(known, m.Extra)
}

func (u Usage) encodeFields() map[string]json.RawMessage {
	fields := make(map[string]json.RawMessage, len(u.unknown)+2)
	for k, v := range u.unknown {
		fields[k] = v
	}
	fields["input_tokens"] = mustMarshal(u.InputTokens)
	fields["output_tokens"] = mustMarshal(u.OutputTokens)
	return fields
}

func mustMarshal(v any) json.RawMessage {
	raw, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage("null") // unreachable for these value shapes
	}
	return raw
}
