package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cristianoliveira/jeq/internal/domain/contract"
	"github.com/cristianoliveira/jeq/internal/domain/jeq"
)

type questionSourceFlags struct {
	file, inline       string
	fileSet, inlineSet bool
}

func addQuestionSourceFlags(flags interface {
	StringVar(*string, string, string, string)
}, source *questionSourceFlags,
) {
	flags.StringVar(&source.file, "questions", "", "questions document file")
	flags.StringVar(&source.inline, "questions-json", "", "inline questions document JSON")
}

func checkQuestionSource(source questionSourceFlags) *jeq.Error {
	if source.fileSet == source.inlineSet {
		return jeq.NewError(jeq.CodeSourceConflict, "choose exactly one of --questions or --questions-json")
	}
	if source.fileSet && strings.TrimSpace(source.file) == "-" {
		return jeq.NewError(jeq.CodeSourceConflict, "--questions cannot read stdin while input records use stdin")
	}
	return nil
}

func validateInlineQuestionsDocument(document []byte) *jeq.Error {
	if err := validateQuestionsDocument(document); err != nil {
		return jeq.NewError(err.Code, "inline questions JSON is invalid or violates local question rules")
	}
	return nil
}

func validateQuestionsDocument(document []byte) *jeq.Error {
	questions, extra, err := contract.DecodeQuestionsDoc(document)
	if err != nil {
		return jeq.NewError(err.Code, err.Message)
	}
	probe := contract.Request{Model: "probe", State: json.RawMessage(`"probe"`), Questions: questions, Extra: extra}
	if violations := contract.ValidateRequest(probe); len(violations) > 0 {
		return violations[0].Error
	}
	return nil
}

func validateInlineStateJSON(raw []byte, flag string) *jeq.Error {
	return validateStateJSON(raw, flag, SourceLimit)
}

func validateStateJSON(raw []byte, flag string, limit int64) *jeq.Error {
	if int64(len(raw)) > limit {
		return jeq.NewError(jeq.CodeInputInvalid, fmt.Sprintf("%s exceeds the %d byte limit", flag, limit))
	}
	if err := contract.CheckStateValue(raw); err != nil {
		recovery := "check the selected JSON input and provide a non-empty string, object, or array"
		if flag == "--state-json" {
			recovery = "pass JSON directly to --state-json; use --state-json-file PATH for a JSON file"
		}
		message := "state JSON must be a valid non-empty JSON string, object, or array"
		if flag == "--state-json" {
			message += "; use --state-json-file PATH for a JSON file"
		}
		return jeq.NewError(jeq.CodeInputInvalid, message).WithRecovery(recovery)
	}
	return nil
}

func readQuestionSource(deps AskDeps, source questionSourceFlags) (map[string]contract.Question, map[string]json.RawMessage, *jeq.Error) {
	var document []byte
	if source.fileSet {
		data, err := deps.ReadFile(source.file, MapMaxRecordBytes)
		if err != nil {
			return nil, nil, err
		}
		document = data
	} else {
		if len([]byte(source.inline)) > MapMaxRecordBytes {
			return nil, nil, jeq.NewError(jeq.CodeInputInvalid, fmt.Sprintf("--questions-json exceeds the %d byte limit", MapMaxRecordBytes))
		}
		document = []byte(source.inline)
	}
	questions, extra, err := contract.DecodeQuestionsDoc(document)
	if err != nil {
		if source.inlineSet {
			return nil, nil, jeq.NewError(err.Code, "inline questions JSON is invalid or violates local question rules")
		}
		return nil, nil, err
	}
	probe := contract.Request{Model: "probe", State: json.RawMessage(`"probe"`), Questions: questions, Extra: extra}
	if violations := contract.ValidateRequest(probe); len(violations) > 0 {
		if source.inlineSet {
			return nil, nil, jeq.NewError(violations[0].Error.Code, "inline questions JSON is invalid or violates local question rules")
		}
		return nil, nil, violations[0].Error
	}
	return questions, extra, nil
}
