// Package contract is the TypeSafe System One language: request, question,
// answer, response, and models documents. Decode is strict (no duplicate
// keys, valid UTF-8, known-field types) yet lossless (unknown fields pass
// through byte-semantically). It depends on the standard library and
// domain/gev codes only.
package contract

import (
	"encoding/json"
	"fmt"
)

// QuestionType enumerates the known System One primitives.
type QuestionType string

// The three primitives. Anything else defers to the server (rejected only by
// the local known-type rule, never by decode).
const (
	TypeNoul   QuestionType = "noul"
	TypeChoice QuestionType = "choice"
	TypeScore  QuestionType = "score"
)

// Request is the native POST /v1/systemone document. State stays raw JSON
// because the API accepts string, object, or array; unknown top-level
// fields ride in Extra.
type Request struct {
	Model     string
	State     json.RawMessage
	Questions map[string]Question

	Extra map[string]json.RawMessage
}

// Question is one typed judgment. Instructions accept string, object, or
// array; Criteria's JSON shape depends on Type and is validated locally.
type Question struct {
	Type         QuestionType
	Instructions json.RawMessage
	Criteria     json.RawMessage // optional for noul; required for choice/score

	Extra map[string]json.RawMessage
}

// Usage is the token accounting for one evaluation.
type Usage struct {
	InputTokens  int64
	OutputTokens int64

	// unknown keeps every usage member verbatim for lossless re-encode.
	unknown map[string]json.RawMessage
}

// Response is the evaluation result. Decode is tolerant: unknown server
// fields survive in Extra; missing documented-required fields classify as
// GEV_RESPONSE_INVALID.
type Response struct {
	Model   string
	Answers map[string]Answer
	Usage   Usage

	Extra map[string]json.RawMessage
}

// Answer is the typed answer for one question id.
type Answer struct {
	Type QuestionType

	Noul       *float64
	Choice     *string
	Score      *float64
	Legend     map[string]string
	Probs      map[string]float64
	Confidence *float64

	Extra map[string]json.RawMessage
}

// ModelInfo describes one entry of GET /v1/models.
type ModelInfo struct {
	Name        string
	Description string
	ReleaseDate string

	Extra map[string]json.RawMessage
}

// Models is the GET /v1/models document.
type Models struct {
	Models []ModelInfo

	Extra map[string]json.RawMessage
}

// encodeDoc merges known fields with unknown Extra fields into one
// deterministic JSON object (keys sorted by encoding/json).
func encodeDoc(known map[string]json.RawMessage, extra map[string]json.RawMessage) ([]byte, error) {
	doc := make(map[string]json.RawMessage, len(known)+len(extra))
	for k, v := range known {
		doc[k] = v
	}
	for k, v := range extra {
		if _, clash := doc[k]; clash {
			// Unreachable by construction: decode never puts known keys in Extra.
			return nil, fmt.Errorf("unknown field collides with a known field: %s", k)
		}
		doc[k] = v
	}
	return json.Marshal(doc)
}
