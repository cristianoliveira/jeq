package jeq

import (
	"encoding/json"
	"strings"

	"github.com/cristianoliveira/jeq/internal/domain/contract"
)

// StateInput is the resolved state source. File sources are read by the
// shell layer; the domain only sees text or raw JSON.
type StateInput struct {
	Kind Source // SourceStateText or SourceStateJSON
	Text string
	JSON json.RawMessage
}

// ComposeInput is the pure composition input: raw documents and resolved
// values only — no filesystem, environment, or network access happens here.
type ComposeInput struct {
	RequestDoc   []byte // native mode: the complete request document
	QuestionsDoc []byte // composed mode: the questions document
	State        StateInput
	Model        string // pre-resolved model (cli precedence); used in composed mode
}

// Compose assembles the outgoing request. Native mode validates and passes
// the document through, unknown fields included. Composed mode parses the
// questions document, attaches the state and the explicit model, and keeps
// unknown questions-document fields on the request. All local rules run
// here; the first violation (stable order) is returned.
func Compose(in ComposeInput) (contract.Request, *Error) {
	if err := CheckSources(Sources{
		Request:   in.RequestDoc != nil,
		Questions: in.QuestionsDoc != nil,
		StateText: in.State.Kind == SourceStateText,
		StateJSON: in.State.Kind == SourceStateJSON,
	}); err != nil {
		return contract.Request{}, err
	}

	if in.RequestDoc != nil {
		return composeNative(in)
	}
	return composeComposed(in)
}

func composeNative(in ComposeInput) (contract.Request, *Error) {
	req, err := contract.DecodeRequest(in.RequestDoc)
	if err != nil {
		return contract.Request{}, err
	}
	if violations := contract.ValidateRequest(req); len(violations) > 0 {
		return contract.Request{}, violations[0].Error
	}
	// The document's own model is authoritative in native mode; the
	// composed-model precedence does not apply.
	return req, nil
}

func composeComposed(in ComposeInput) (contract.Request, *Error) {
	if in.Model == "" {
		return contract.Request{}, NewError(CodeInputInvalid,
			"model: is required in composed mode; resolve --model, TYPESAFE_DEFAULT_MODEL, or jev-latest first")
	}

	questions, extra, err := contract.DecodeQuestionsDoc(in.QuestionsDoc)
	if err != nil {
		return contract.Request{}, err
	}

	state, err := stateValue(in.State)
	if err != nil {
		return contract.Request{}, err
	}

	req := contract.Request{
		Model:     in.Model,
		State:     state,
		Questions: questions,
		Extra:     extra,
	}
	if violations := contract.ValidateRequest(req); len(violations) > 0 {
		return contract.Request{}, violations[0].Error
	}
	return req, nil
}

// stateValue converts the resolved state source into its JSON form and
// checks it against the API's string|object|array shape.
func stateValue(in StateInput) (json.RawMessage, *Error) {
	switch in.Kind {
	case SourceStateText:
		if strings.TrimSpace(in.Text) == "" {
			return nil, NewError(CodeInputInvalid,
				"state: resolved to an empty value; provide non-empty state content")
		}
		raw, err := json.Marshal(in.Text)
		if err != nil {
			return nil, WrapError(CodeInputInvalid, err, "state: cannot encode text")
		}
		return raw, nil
	case SourceStateJSON:
		if err := contract.CheckStateValue(in.JSON); err != nil {
			return nil, err
		}
		return in.JSON, nil
	default:
		return nil, NewError(CodeInputInvalid, "state: no state source provided")
	}
}
