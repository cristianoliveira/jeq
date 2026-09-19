package contract

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"

	"github.com/cristianoliveira/jeq/internal/domain/jeq/codes"
)

// DecodeRequest parses a native request document strictly: valid UTF-8, no
// duplicate keys, well-formed JSON, known-field types checked with the
// failing field path in the message. Unknown fields are preserved, never
// rejected.
func DecodeRequest(data []byte) (Request, *codes.Error) {
	if err := scanDoc(data); err != nil {
		return Request{}, codes.WrapError(codes.CodeRequestInvalid, err, "request document")
	}

	var raws map[string]json.RawMessage
	if err := json.Unmarshal(data, &raws); err != nil {
		return Request{}, requestErr(err, "request document")
	}

	req := Request{Questions: map[string]Question{}}
	for key, raw := range raws {
		switch key {
		case "model":
			if err := json.Unmarshal(raw, &req.Model); err != nil {
				return Request{}, requestErr(err, "model")
			}
		case "state":
			if err := checkStateJSON(raw); err != nil {
				return Request{}, err
			}
			req.State = compactRaw(raw)
		case "questions":
			qs, qerr := decodeQuestions(raw)
			if qerr != nil {
				return Request{}, qerr
			}
			req.Questions = qs
		default:
			if req.Extra == nil {
				req.Extra = map[string]json.RawMessage{}
			}
			req.Extra[key] = compactRaw(raw)
		}
	}
	return req, nil
}

// checkStateJSON accepts exactly string, object, or array.
func checkStateJSON(raw json.RawMessage) *codes.Error {
	trimmed := trimSpace(raw)
	if len(trimmed) == 0 {
		return codes.NewError(codes.CodeRequestInvalid, "state: field is missing or null; provide non-empty state")
	}
	switch trimmed[0] {
	case '"', '{', '[':
		return nil
	default:
		return codes.NewError(codes.CodeRequestInvalid, "state: must be a string, object, or array")
	}
}

func decodeQuestions(raw json.RawMessage) (map[string]Question, *codes.Error) {
	var raws map[string]json.RawMessage
	if err := json.Unmarshal(raw, &raws); err != nil {
		return nil, requestErr(err, "questions: must be an object of question ids")
	}

	qs := make(map[string]Question, len(raws))
	for id, qraw := range raws {
		q, qerr := decodeQuestion(qraw)
		if qerr != nil {
			return nil, questionErr(qerr, id)
		}
		qs[id] = q
	}
	return qs, nil
}

func decodeQuestion(raw json.RawMessage) (Question, *codes.Error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return Question{}, requestErr(err, "question: must be an object")
	}

	q := Question{}
	for key, v := range fields {
		switch key {
		case "type":
			if err := json.Unmarshal(v, &q.Type); err != nil {
				return Question{}, requestErr(err, "type: must be a string")
			}
		case "instructions":
			if err := checkInstructionsJSON(v); err != nil {
				return Question{}, err
			}
			q.Instructions = compactRaw(v)
		case "criteria":
			if err := checkCriteriaJSON(v); err != nil {
				return Question{}, err
			}
			q.Criteria = compactRaw(v)
		default:
			if q.Extra == nil {
				q.Extra = map[string]json.RawMessage{}
			}
			q.Extra[key] = compactRaw(v)
		}
	}
	return q, nil
}

// checkInstructionsJSON accepts string, object, or array.
func checkInstructionsJSON(raw json.RawMessage) *codes.Error {
	trimmed := trimSpace(raw)
	if len(trimmed) == 0 {
		return codes.NewError(codes.CodeRequestInvalid, "instructions: field is missing or null")
	}
	switch trimmed[0] {
	case '"', '{', '[':
		return nil
	default:
		return codes.NewError(codes.CodeRequestInvalid, "instructions: must be a string, object, or array")
	}
}

// checkCriteriaJSON accepts object or array at the JSON level; the
// primitive-specific shape is a validation rule, not a decode concern.
func checkCriteriaJSON(raw json.RawMessage) *codes.Error {
	trimmed := trimSpace(raw)
	if len(trimmed) == 0 {
		return codes.NewError(codes.CodeRequestInvalid, "criteria: field is missing or null")
	}
	switch trimmed[0] {
	case '{', '[':
		return nil
	default:
		return codes.NewError(codes.CodeRequestInvalid, "criteria: must be an object or an array")
	}
}

// DecodeResponse parses an evaluation response tolerantly: unknown server
// fields survive; a missing or mistyped documented-required field classifies
// as JEQ_RESPONSE_INVALID.
func DecodeResponse(data []byte) (Response, *codes.Error) {
	if err := scanDoc(data); err != nil {
		return Response{}, codes.WrapError(codes.CodeResponseInvalid, err, "response document")
	}

	var raws map[string]json.RawMessage
	if err := json.Unmarshal(data, &raws); err != nil {
		return Response{}, responseErr(err, "response document")
	}

	resp := Response{}
	var sawUsage, sawAnswers bool
	for key, raw := range raws {
		switch key {
		case "model":
			if err := json.Unmarshal(raw, &resp.Model); err != nil {
				return Response{}, responseErr(err, "model: must be a string")
			}
		case "answers":
			sawAnswers = true
			answers, aerr := decodeAnswers(raw)
			if aerr != nil {
				return Response{}, aerr
			}
			resp.Answers = answers
		case "usage":
			sawUsage = true
			usage, uerr := decodeUsage(raw)
			if uerr != nil {
				return Response{}, uerr
			}
			resp.Usage = usage
		default:
			if resp.Extra == nil {
				resp.Extra = map[string]json.RawMessage{}
			}
			resp.Extra[key] = compactRaw(raw)
		}
	}

	if resp.Model == "" {
		return Response{}, codes.NewError(codes.CodeResponseInvalid, "model: missing; the response must name the model that answered")
	}
	if !sawAnswers {
		return Response{}, codes.NewError(codes.CodeResponseInvalid, "answers: missing; the response must carry one answer per question")
	}
	if !sawUsage {
		return Response{}, codes.NewError(codes.CodeResponseInvalid, "usage: missing; the response must report token usage")
	}
	return resp, nil
}

func decodeAnswers(raw json.RawMessage) (map[string]Answer, *codes.Error) {
	var raws map[string]json.RawMessage
	if err := json.Unmarshal(raw, &raws); err != nil {
		return nil, responseErr(err, "answers: must be an object of answer ids")
	}

	answers := make(map[string]Answer, len(raws))
	for id, araw := range raws {
		a, aerr := decodeAnswer(araw)
		if aerr != nil {
			return nil, answerErr(aerr, id)
		}
		answers[id] = a
	}
	return answers, nil
}

func decodeAnswer(raw json.RawMessage) (Answer, *codes.Error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return Answer{}, responseErr(err, "answer: must be an object")
	}

	a := Answer{}
	for key, v := range fields {
		switch key {
		case "type":
			if err := json.Unmarshal(v, &a.Type); err != nil {
				return Answer{}, responseErr(err, "type: must be a string")
			}
		case "noul":
			a.Noul = new(float64)
			if err := json.Unmarshal(v, a.Noul); err != nil {
				return Answer{}, responseErr(err, "noul: must be a number between 0 and 1")
			}
		case "choice":
			a.Choice = new(string)
			if err := json.Unmarshal(v, a.Choice); err != nil {
				return Answer{}, responseErr(err, "choice: must be a string")
			}
		case "score":
			a.Score = new(float64)
			if err := json.Unmarshal(v, a.Score); err != nil {
				return Answer{}, responseErr(err, "score: must be a number")
			}
		case "confidence":
			a.Confidence = new(float64)
			if err := json.Unmarshal(v, a.Confidence); err != nil {
				return Answer{}, responseErr(err, "confidence: must be a number between 0 and 1")
			}
		case "probabilities":
			if err := json.Unmarshal(v, &a.Probs); err != nil {
				return Answer{}, responseErr(err, "probabilities: must be an object of numbers")
			}
		case "legend":
			if err := json.Unmarshal(v, &a.Legend); err != nil {
				return Answer{}, responseErr(err, "legend: must be an object of strings")
			}
		default:
			if a.Extra == nil {
				a.Extra = map[string]json.RawMessage{}
			}
			a.Extra[key] = compactRaw(v)
		}
	}

	if a.Type == "" {
		return Answer{}, codes.NewError(codes.CodeResponseInvalid, "type: missing; every answer carries its question's type")
	}
	switch a.Type {
	case TypeNoul:
		if a.Noul == nil {
			return Answer{}, codes.NewError(codes.CodeResponseInvalid, "noul: missing for a noul answer")
		}
	case TypeChoice:
		if a.Choice == nil || a.Probs == nil || a.Confidence == nil {
			return Answer{}, codes.NewError(codes.CodeResponseInvalid, "choice, probabilities, confidence: required for a choice answer")
		}
	case TypeScore:
		if a.Score == nil || a.Legend == nil || a.Probs == nil || a.Confidence == nil {
			return Answer{}, codes.NewError(codes.CodeResponseInvalid, "score, legend, probabilities, confidence: required for a score answer")
		}
	default:
		return Answer{}, codes.NewError(codes.CodeResponseInvalid, fmt.Sprintf("type: unknown primitive %q in answer", a.Type))
	}
	return a, nil
}

func decodeUsage(raw json.RawMessage) (Usage, *codes.Error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return Usage{}, responseErr(err, "usage: must be an object")
	}

	usage := Usage{}
	if fields != nil {
		usage.unknown = make(map[string]json.RawMessage, len(fields))
		for k, v := range fields {
			usage.unknown[k] = compactRaw(v)
		}
	}
	if v, ok := fields["input_tokens"]; ok {
		if err := strictInt(v, &usage.InputTokens); err != nil {
			return Usage{}, responseErr(err, "usage.input_tokens: must be an integer")
		}
	}
	if v, ok := fields["output_tokens"]; ok {
		if err := strictInt(v, &usage.OutputTokens); err != nil {
			return Usage{}, responseErr(err, "usage.output_tokens: must be an integer")
		}
	}
	return usage, nil
}

// strictInt decodes a JSON number that must carry no fraction.
func strictInt(raw json.RawMessage, out *int64) error {
	var f float64
	if err := json.Unmarshal(raw, &f); err != nil {
		return err
	}
	if f != math.Trunc(f) {
		return fmt.Errorf("must be an integer, got %v", f)
	}
	*out = int64(f)
	return nil
}

// trimSpace strips surrounding JSON whitespace from a raw value.
func trimSpace(raw json.RawMessage) []byte {
	return bytes.TrimSpace(raw)
}

// compactRaw normalizes a raw JSON value to its whitespace-free form so a
// second decode of an encoded document compares equal (json.Compact keeps
// number spellings and string bytes verbatim).
func compactRaw(raw json.RawMessage) json.RawMessage {
	var buf bytes.Buffer
	if err := json.Compact(&buf, raw); err != nil {
		return raw // malformed input never reaches the storage path
	}
	return buf.Bytes()
}

// DecodeModels parses the GET /v1/models document.
func DecodeModels(data []byte) (Models, *codes.Error) {
	if err := scanDoc(data); err != nil {
		return Models{}, codes.WrapError(codes.CodeResponseInvalid, err, "models document")
	}

	var raws map[string]json.RawMessage
	if err := json.Unmarshal(data, &raws); err != nil {
		return Models{}, responseErr(err, "models document")
	}

	models := Models{}
	for key, raw := range raws {
		switch key {
		case "models":
			var entries []json.RawMessage
			if err := json.Unmarshal(raw, &entries); err != nil {
				return Models{}, responseErr(err, "models: must be an array")
			}
			for i, entry := range entries {
				info, ierr := decodeModelInfo(entry)
				if ierr != nil {
					return Models{}, responseErr(ierr, fmt.Sprintf("models[%d]", i))
				}
				models.Models = append(models.Models, info)
			}
		default:
			if models.Extra == nil {
				models.Extra = map[string]json.RawMessage{}
			}
			models.Extra[key] = compactRaw(raw)
		}
	}
	return models, nil
}

func decodeModelInfo(raw json.RawMessage) (ModelInfo, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return ModelInfo{}, fmt.Errorf("must be an object")
	}

	info := ModelInfo{}
	for key, v := range fields {
		switch key {
		case "name":
			if err := json.Unmarshal(v, &info.Name); err != nil {
				return ModelInfo{}, fmt.Errorf("name: must be a string")
			}
		case "description":
			if err := json.Unmarshal(v, &info.Description); err != nil {
				return ModelInfo{}, fmt.Errorf("description: must be a string")
			}
		case "release_date":
			if err := json.Unmarshal(v, &info.ReleaseDate); err != nil {
				return ModelInfo{}, fmt.Errorf("release_date: must be a string")
			}
		default:
			if info.Extra == nil {
				info.Extra = map[string]json.RawMessage{}
			}
			info.Extra[key] = compactRaw(v)
		}
	}
	return info, nil
}

// requestErr, responseErr, questionErr, answerErr wrap a decode failure in
// the stable code for the document kind, keeping the field path visible.
func requestErr(err error, path string) *codes.Error {
	return codes.WrapError(codes.CodeRequestInvalid, err, path+": incompatible JSON type")
}

func responseErr(err error, path string) *codes.Error {
	return codes.WrapError(codes.CodeResponseInvalid, err, path)
}

func questionErr(qerr *codes.Error, id string) *codes.Error {
	return codes.NewError(qerr.Code, "questions."+id+"."+qerr.Message)
}

func answerErr(aerr *codes.Error, id string) *codes.Error {
	return codes.NewError(aerr.Code, "answers."+id+"."+aerr.Message)
}

// CheckStateValue verifies a raw state value against the API shape:
// string, object, or array, non-empty. Exported for composed mode, where
// jeq assembles the state from resolved sources.
func CheckStateValue(raw json.RawMessage) *codes.Error {
	return checkStateJSON(raw)
}

// DecodeQuestionsDoc parses a composed-mode questions document: an object
// with a required "questions" map of typed questions. Unknown top-level
// fields are returned so they can ride onto the outgoing request.
func DecodeQuestionsDoc(data []byte) (map[string]Question, map[string]json.RawMessage, *codes.Error) {
	if err := scanDoc(data); err != nil {
		return nil, nil, codes.WrapError(codes.CodeRequestInvalid, err, "questions document")
	}

	var raws map[string]json.RawMessage
	if err := json.Unmarshal(data, &raws); err != nil {
		return nil, nil, requestErr(err, "questions document: must be an object")
	}

	var questions map[string]Question
	extra := map[string]json.RawMessage{}
	for key, raw := range raws {
		if key != "questions" {
			extra[key] = compactRaw(raw)
			continue
		}
		qs, qerr := decodeQuestions(raw)
		if qerr != nil {
			return nil, nil, qerr
		}
		questions = qs
	}
	if questions == nil {
		return nil, nil, codes.NewError(codes.CodeInputInvalid,
			`questions: missing; the document must carry a "questions" object`)
	}
	return questions, extra, nil
}
