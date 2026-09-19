// Package pipeline provides one-record, append-only semantic enrichment.
// It has no filesystem, HTTP, CLI, or rendering dependencies.
package pipeline

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/cristianoliveira/gev/internal/domain/contract"
	"github.com/cristianoliveira/gev/internal/domain/gev"
)

const maxNameLength = 64

// Config describes one append-only enrichment.
type Config struct {
	Name      string
	Pointer   string
	Model     string
	Questions map[string]contract.Question
}

// Evaluator is the existing TypeSafe evaluation port.
type Evaluator interface {
	Evaluate(context.Context, contract.Request) (contract.Response, *gev.Error)
}

// Enrich selects state from one strict JSON object, evaluates one normal
// request, and appends the complete typed response under _gev.<name>.
func Enrich(ctx context.Context, record []byte, config Config, evaluator Evaluator) ([]byte, *gev.Error) {
	members, err := decodeObject(record, "record")
	if err != nil {
		return nil, inputError(err.Error())
	}
	if err := validateName(config.Name); err != nil {
		return nil, inputError(err.Error())
	}
	if config.Model == "" || evaluator == nil {
		return nil, inputError("model and evaluator are required")
	}
	gevMembers, err := existingEvidence(members)
	if err != nil {
		return nil, inputError(err.Error())
	}
	if _, exists := gevMembers[config.Name]; exists {
		return nil, inputError(fmt.Sprintf("_gev.%s already exists; choose a new name", config.Name))
	}
	selected, err := resolvePointer(record, config.Pointer)
	if err != nil {
		return nil, inputError(err.Error())
	}
	if stateErr := contract.CheckStateValue(selected); stateErr != nil {
		return nil, stateErr
	}
	request := contract.Request{Model: config.Model, State: selected, Questions: config.Questions}
	if violations := contract.ValidateRequest(request); len(violations) > 0 {
		return nil, violations[0].Error
	}
	response, evalErr := evaluator.Evaluate(ctx, request)
	if evalErr != nil {
		return nil, evalErr
	}
	encoded, encodeErr := response.Encode()
	if encodeErr != nil {
		return nil, gev.WrapError(gev.CodeResponseInvalid, encodeErr, "encoding evaluator response")
	}
	gevMembers[config.Name] = encoded
	members["_gev"], _ = json.Marshal(gevMembers)
	result, marshalErr := json.Marshal(members)
	if marshalErr != nil {
		return nil, gev.WrapError(gev.CodeResponseInvalid, marshalErr, "encoding enriched record")
	}
	return result, nil
}

func validateName(name string) error {
	if name == "" || len(name) > maxNameLength {
		return fmt.Errorf("name must be 1..%d characters", maxNameLength)
	}
	for index, char := range name {
		if (index == 0 && !asciiAlphaNumeric(char)) || (index > 0 && !asciiAlphaNumeric(char) && char != '_' && char != '-') {
			return fmt.Errorf("name %q must start with a letter or digit and then contain only letters, digits, _ or -", name)
		}
	}
	return nil
}

func asciiAlphaNumeric(char rune) bool {
	return char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9'
}

func existingEvidence(members map[string]json.RawMessage) (map[string]json.RawMessage, error) {
	raw, ok := members["_gev"]
	if !ok {
		return map[string]json.RawMessage{}, nil
	}
	return decodeObject(raw, "_gev")
}

func decodeObject(data []byte, path string) (map[string]json.RawMessage, error) {
	if err := scanJSON(data); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	var members map[string]json.RawMessage
	if err := json.Unmarshal(data, &members); err != nil || members == nil {
		return nil, fmt.Errorf("%s must be a JSON object", path)
	}
	return members, nil
}

func scanJSON(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := scanValue(decoder); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return fmt.Errorf("trailing JSON value")
		}
		return err
	}
	return nil
}

func scanValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delim, isDelim := token.(json.Delim)
	if !isDelim {
		return nil
	}
	switch delim {
	case '{':
		seen := map[string]bool{}
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string)
			if !ok || seen[key] {
				return fmt.Errorf("duplicate object key %q", key)
			}
			seen[key] = true
			if err := scanValue(decoder); err != nil {
				return err
			}
		}
		_, err = decoder.Token()
		return err
	case '[':
		for decoder.More() {
			if err := scanValue(decoder); err != nil {
				return err
			}
		}
		_, err = decoder.Token()
		return err
	default:
		return fmt.Errorf("unexpected JSON delimiter %q", delim)
	}
}

func resolvePointer(document []byte, pointer string) (json.RawMessage, error) {
	if pointer == "" {
		return append(json.RawMessage(nil), document...), nil
	}
	if pointer[0] != '/' {
		return nil, fmt.Errorf("pointer %q must be empty or start with /", pointer)
	}
	current := append(json.RawMessage(nil), document...)
	for _, token := range strings.Split(pointer[1:], "/") {
		part, err := unescape(token)
		if err != nil {
			return nil, fmt.Errorf("pointer %q: %w", pointer, err)
		}
		current, err = pointerChild(current, part)
		if err != nil {
			return nil, fmt.Errorf("pointer %q: %w", pointer, err)
		}
	}
	return current, nil
}

func unescape(token string) (string, error) {
	for index := 0; index < len(token); index++ {
		if token[index] == '~' {
			if index+1 >= len(token) || (token[index+1] != '0' && token[index+1] != '1') {
				return "", fmt.Errorf("invalid ~ escape")
			}
		}
	}
	return strings.ReplaceAll(strings.ReplaceAll(token, "~1", "/"), "~0", "~"), nil
}

func pointerChild(current json.RawMessage, token string) (json.RawMessage, error) {
	trimmed := bytes.TrimSpace(current)
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("empty value")
	}
	switch trimmed[0] {
	case '{':
		members, err := decodeObject(trimmed, "pointer object")
		if err != nil {
			return nil, err
		}
		value, ok := members[token]
		if !ok {
			return nil, fmt.Errorf("missing object member %q", token)
		}
		return value, nil
	case '[':
		if token == "" || (len(token) > 1 && token[0] == '0') {
			return nil, fmt.Errorf("array index %q is not canonical", token)
		}
		index, err := strconv.Atoi(token)
		if err != nil || index < 0 {
			return nil, fmt.Errorf("array index %q is invalid", token)
		}
		var values []json.RawMessage
		if err := json.Unmarshal(trimmed, &values); err != nil || index >= len(values) {
			return nil, fmt.Errorf("array index %q is out of range", token)
		}
		return values[index], nil
	default:
		return nil, fmt.Errorf("cannot traverse JSON scalar with %q", token)
	}
}

func inputError(message string) *gev.Error {
	return gev.NewError(gev.CodeInputInvalid, message)
}
