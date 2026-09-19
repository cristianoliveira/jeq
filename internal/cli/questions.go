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
		return nil, nil, err
	}
	probe := contract.Request{Model: "probe", State: json.RawMessage(`"probe"`), Questions: questions}
	if violations := contract.ValidateRequest(probe); len(violations) > 0 {
		return nil, nil, violations[0].Error
	}
	return questions, extra, nil
}
