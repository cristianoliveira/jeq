package contract

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/cristianoliveira/gev/internal/domain/gev/codes"
)

// Rule is one client-owned local invariant. This list is closed (OQ-2):
// anything not listed here defers to the server, and a 422 classifies as an
// API failure, never as a local rejection.
type Rule struct {
	Name     string
	Code     codes.Code
	Recovery string
}

// Violation pairs a broken rule with the coded error naming the field path.
type Violation struct {
	Rule  string
	Error *codes.Error
}

// The client-owned local rules, in checking order. All are local document
// validation, so all carry GEV_REQUEST_INVALID.
var (
	ruleWellformed = Rule{
		Name:     "json_wellformed",
		Code:     codes.CodeRequestInvalid,
		Recovery: "reformat the document as strict JSON (no BOM, no trailing data)",
	}
	ruleDuplicateKeys = Rule{
		Name:     "no_duplicate_keys",
		Code:     codes.CodeRequestInvalid,
		Recovery: "remove the repeated key so every key appears exactly once",
	}
	ruleRequestSchema = Rule{
		Name:     "request_schema",
		Code:     codes.CodeRequestInvalid,
		Recovery: "correct the type of the field named in the message",
	}
	ruleStateNonEmpty = Rule{
		Name:     "state_non_empty",
		Code:     codes.CodeRequestInvalid,
		Recovery: "provide the content to evaluate in state",
	}
	ruleQuestionsNonEmpty = Rule{
		Name:     "questions_non_empty",
		Code:     codes.CodeRequestInvalid,
		Recovery: "add at least one question to the questions object",
	}
	ruleQuestionTypeKnown = Rule{
		Name:     "question_type_known",
		Code:     codes.CodeRequestInvalid,
		Recovery: "use one of the known primitives: noul, choice, score",
	}
	ruleInstructionsNonEmpty = Rule{
		Name:     "instructions_non_empty",
		Code:     codes.CodeRequestInvalid,
		Recovery: "write the question itself in instructions",
	}
	ruleChoiceCriteriaNonEmpty = Rule{
		Name:     "choice_criteria_non_empty",
		Code:     codes.CodeRequestInvalid,
		Recovery: "list the possible options in criteria",
	}
	ruleScoreCriteriaMinLevels = Rule{
		Name:     "score_criteria_min_levels",
		Code:     codes.CodeRequestInvalid,
		Recovery: "define at least two ordered level descriptions in criteria",
	}
	ruleNoulCriteriaShape = Rule{
		Name:     "noul_criteria_shape",
		Code:     codes.CodeRequestInvalid,
		Recovery: `describe yes and no as {"true": "...", "false": "..."} in criteria`,
	}
)

// Rules returns the complete set of client-owned local rules.
func Rules() []Rule {
	return []Rule{
		ruleWellformed,
		ruleDuplicateKeys,
		ruleRequestSchema,
		ruleStateNonEmpty,
		ruleQuestionsNonEmpty,
		ruleQuestionTypeKnown,
		ruleInstructionsNonEmpty,
		ruleChoiceCriteriaNonEmpty,
		ruleScoreCriteriaMinLevels,
		ruleNoulCriteriaShape,
	}
}

// Validate runs every client-owned rule over a native request document, in
// stable order. It is pure: no credential lookup, no filesystem, no network.
func Validate(data []byte) []Violation {
	if err := scanDoc(data); err != nil {
		var se *scanError
		if errors.As(err, &se) && se.duplicate {
			return []Violation{newViolation(ruleDuplicateKeys, err)}
		}
		return []Violation{newViolation(ruleWellformed, err)}
	}

	req, derr := DecodeRequest(data)
	if derr != nil {
		return []Violation{newViolation(ruleRequestSchema, errors.New(derr.Message))}
	}
	return ValidateRequest(req)
}

// ValidateRequest runs the content rules over an already-decoded request
// (the composition path, where no raw document exists).
func ValidateRequest(req Request) []Violation {
	var out []Violation

	if emptyJSONValue(req.State) {
		out = append(out, newViolation(ruleStateNonEmpty, errors.New("state: is empty")))
	}
	if len(req.Questions) == 0 {
		out = append(out, newViolation(ruleQuestionsNonEmpty, errors.New("questions: is empty")))
	}

	ids := make([]string, 0, len(req.Questions))
	for id := range req.Questions {
		ids = append(ids, id)
	}
	sort.Strings(ids) // deterministic violation order

	for _, id := range ids {
		out = append(out, validateQuestion(id, req.Questions[id])...)
	}
	return out
}

func validateQuestion(id string, q Question) []Violation {
	var out []Violation
	path := "questions." + id

	if !knownType(q.Type) {
		out = append(out, newViolation(ruleQuestionTypeKnown,
			fmt.Errorf("%s.type: unknown primitive %q", path, q.Type)))
		return out
	}
	if emptyJSONValue(q.Instructions) {
		out = append(out, newViolation(ruleInstructionsNonEmpty,
			fmt.Errorf("%s.instructions: is empty", path)))
	}

	switch q.Type {
	case TypeChoice:
		if !nonEmptyObject(q.Criteria) {
			out = append(out, newViolation(ruleChoiceCriteriaNonEmpty,
				fmt.Errorf("%s.criteria: choice needs a non-empty map of options", path)))
		}
	case TypeScore:
		if arrayLength(q.Criteria) < 2 {
			out = append(out, newViolation(ruleScoreCriteriaMinLevels,
				fmt.Errorf("%s.criteria: score needs at least two level descriptions", path)))
		}
	case TypeNoul:
		if len(q.Criteria) > 0 && !validNoulCriteria(q.Criteria) {
			out = append(out, newViolation(ruleNoulCriteriaShape,
				fmt.Errorf("%s.criteria: noul criteria must be an object with optional true/false string descriptions", path)))
		}
	}
	return out
}

func knownType(t QuestionType) bool {
	return t == TypeNoul || t == TypeChoice || t == TypeScore
}

// emptyJSONValue reports whether raw is absent, null, a blank string, or a
// semantically empty object/array. It parses the JSON shape, so formatting
// and whitespace never change the verdict (F-D1-1).
func emptyJSONValue(raw json.RawMessage) bool {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return true
	}
	switch trimmed[0] {
	case '"':
		var s string
		if json.Unmarshal(trimmed, &s) != nil {
			return false
		}
		return strings.TrimSpace(s) == ""
	case '{':
		var m map[string]json.RawMessage
		return json.Unmarshal(trimmed, &m) != nil || len(m) == 0
	case '[':
		var a []json.RawMessage
		return json.Unmarshal(trimmed, &a) != nil || len(a) == 0
	default:
		return false
	}
}

func nonEmptyObject(raw json.RawMessage) bool {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(bytes.TrimSpace(raw), &m); err != nil {
		return false
	}
	return len(m) > 0
}

func arrayLength(raw json.RawMessage) int {
	var a []json.RawMessage
	if err := json.Unmarshal(bytes.TrimSpace(raw), &a); err != nil {
		return 0
	}
	return len(a)
}

func validNoulCriteria(raw json.RawMessage) bool {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(bytes.TrimSpace(raw), &m); err != nil {
		return false
	}
	for key, val := range m {
		if key != "true" && key != "false" {
			return false
		}
		var s string
		if err := json.Unmarshal(val, &s); err != nil {
			return false
		}
	}
	return true
}

func newViolation(rule Rule, err error) Violation {
	message := err.Error()
	return Violation{
		Rule:  rule.Name,
		Error: codes.NewError(rule.Code, message+"; "+rule.Recovery),
	}
}
