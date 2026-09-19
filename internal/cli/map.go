package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/cristianoliveira/gev/internal/domain/contract"
	"github.com/cristianoliveira/gev/internal/domain/gev"
	"github.com/cristianoliveira/gev/internal/domain/pipeline"
	"github.com/spf13/cobra"
)

const (
	// MapMaxRecordBytes bounds the size of one JSON input record.
	MapMaxRecordBytes = 8 << 20
	// MapMaxRecords bounds the number of records read from stdin.
	MapMaxRecords = 10000
	// MapMaxInputBytes bounds one command's buffered input.
	MapMaxInputBytes = 64 << 20
)

// NewMapCmd creates the sequential, one-request-per-record map primitive.
func NewMapCmd(deps AskDeps) *cobra.Command {
	var name, input, questions, statePointer, requestPointer, model, baseURL, timeoutText string
	var maxRetries int
	cmd := &cobra.Command{
		Use:   "map",
		Short: "Enrich each JSON record with one named judgment",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runMap(cmd, deps, mapFlags{
				name: name, input: input, questions: questions, statePointer: statePointer,
				requestPointer: requestPointer, model: model, baseURL: baseURL,
				timeout: timeoutText, maxRetries: maxRetries,
				nameSet: cmd.Flags().Changed("as"), inputSet: cmd.Flags().Changed("input"),
				questionsSet: cmd.Flags().Changed("questions"), requestPointerSet: cmd.Flags().Changed("request-pointer"),
				modelSet: cmd.Flags().Changed("model"), baseURLSet: cmd.Flags().Changed("base-url"),
			})
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&name, "as", "", "evidence name (required)")
	flags.StringVar(&input, "input", "json", "input framing: json or ndjson")
	flags.StringVar(&questions, "questions", "", "questions document file")
	flags.StringVar(&statePointer, "state-pointer", "", "RFC 6901 pointer to state")
	flags.StringVar(&requestPointer, "request-pointer", "", "RFC 6901 pointer to a native request")
	flags.StringVar(&model, "model", "", "composed-mode model")
	flags.StringVar(&baseURL, "base-url", "", "TypeSafe API root")
	flags.StringVar(&timeoutText, "timeout", DefaultTimeout.String(), "request timeout")
	flags.IntVar(&maxRetries, "max-retries", DefaultMaxRetries, "maximum retries (0-5)")
	return cmd
}

type mapFlags struct {
	name, input, questions, statePointer, requestPointer, model, baseURL, timeout string
	maxRetries                                                                    int
	nameSet, inputSet, questionsSet, requestPointerSet, modelSet, baseURLSet      bool
}

func runMap(cmd *cobra.Command, deps AskDeps, f mapFlags) error {
	if output, err := cmd.Flags().GetString("output"); err != nil || output != "json" {
		return gev.NewError(gev.CodeInputInvalid, fmt.Sprintf("unsupported output format %q", output)).WithRecovery("set --output json")
	}
	if !f.nameSet || f.name == "" {
		return gev.NewError(gev.CodeInputInvalid, "--as is required")
	}
	if err := pipeline.ValidateName(f.name); err != nil {
		return gev.NewError(gev.CodeInputInvalid, err.Error())
	}
	if err := pipeline.ValidatePointer(f.statePointer); err != nil {
		return gev.NewError(gev.CodeInputInvalid, err.Error())
	}
	if err := pipeline.ValidatePointer(f.requestPointer); err != nil {
		return gev.NewError(gev.CodeInputInvalid, err.Error())
	}
	if f.input != "json" && f.input != "ndjson" {
		return gev.NewError(gev.CodeInputInvalid, "--input must be json or ndjson")
	}
	if f.requestPointerSet == f.questionsSet {
		return gev.NewError(gev.CodeInputInvalid, "choose exactly one of --questions or --request-pointer")
	}
	if f.requestPointerSet && (f.modelSet || f.statePointer != "") {
		return gev.NewError(gev.CodeInputInvalid, "--request-pointer cannot be combined with --model or --state-pointer")
	}
	if f.questions == "-" {
		return gev.NewError(gev.CodeSourceConflict, "--questions cannot read stdin while input records use stdin")
	}
	if f.maxRetries < 0 || f.maxRetries > MaxRetriesLimit {
		return gev.NewError(gev.CodeInputInvalid, fmt.Sprintf("--max-retries must be between 0 and %d", MaxRetriesLimit))
	}
	timeout, err := time.ParseDuration(f.timeout)
	if err != nil || timeout <= 0 {
		return gev.NewError(gev.CodeInputInvalid, "--timeout must be a positive duration")
	}
	if deps.Stdin == nil || deps.ReadStdin == nil || deps.ReadFile == nil || deps.NewClient == nil || deps.Getenv == nil {
		return gev.NewError(gev.CodeInputInvalid, "map dependencies are unavailable")
	}

	var inputDoc []byte
	inputDoc, readErr := deps.ReadStdin(deps.Stdin, MapMaxInputBytes, true)
	if readErr != nil {
		return readErr
	}
	var questions map[string]contract.Question
	var extra map[string]json.RawMessage
	if f.questionsSet {
		questionsDoc, fileErr := deps.ReadFile(f.questions, MapMaxRecordBytes)
		if fileErr != nil {
			return fileErr
		}
		var decodeErr *gev.Error
		questions, extra, decodeErr = contract.DecodeQuestionsDoc(questionsDoc)
		if decodeErr != nil {
			return decodeErr
		}
		probe := contract.Request{Model: "probe", State: json.RawMessage(`"probe"`), Questions: questions}
		if violations := contract.ValidateRequest(probe); len(violations) > 0 {
			return violations[0].Error
		}
	}
	records, framingErr := mapRecords(inputDoc, f.input)
	if framingErr != nil {
		return framingErr
	}
	if err := validateMapRecord(records[0], f, questions, extra, modelForValidation(f, deps.Getenv)); err != nil {
		return err
	}

	apiKey := strings.TrimSpace(deps.Getenv("TYPESAFE_API_KEY"))
	if apiKey == "" {
		return gev.NewError(gev.CodeAuthMissing, "TYPESAFE_API_KEY is not set")
	}
	rootURL := f.baseURL
	if !f.baseURLSet {
		rootURL = deps.Getenv("TYPESAFE_BASE_URL")
		if rootURL == "" {
			rootURL = DefaultBaseURL
		}
	}
	resolvedModel := f.model
	if f.questionsSet && resolvedModel == "" {
		resolvedModel = ResolveModel("", deps.Getenv)
	}
	if f.questionsSet && resolvedModel == "" {
		return gev.NewError(gev.CodeInputInvalid, "model is required in composed mode")
	}
	client := deps.NewClient(rootURL, timeout, apiKey, f.maxRetries, func(line string) { _, _ = fmt.Fprintln(cmd.ErrOrStderr(), line) })
	evaluator := client
	return processMapInput(cmd.Context(), cmd, deps.Renderer, inputDoc, f, questions, extra, resolvedModel, evaluator)
}

func modelForValidation(f mapFlags, getenv func(string) string) string {
	if f.model != "" {
		return f.model
	}
	if f.questionsSet {
		return ResolveModel("", getenv)
	}
	return "native"
}

func validateMapRecord(record []byte, f mapFlags, questions map[string]contract.Question, extra map[string]json.RawMessage, model string) *gev.Error {
	if err := pipeline.ValidateRecord(record); err != nil {
		return gev.NewError(gev.CodeInputInvalid, err.Error())
	}
	if f.requestPointerSet {
		raw, err := pipeline.Select(record, f.requestPointer)
		if err != nil {
			return gev.NewError(gev.CodeInputInvalid, err.Error())
		}
		req, decodeErr := contract.DecodeRequest(raw)
		if decodeErr != nil {
			return decodeErr
		}
		if violations := contract.ValidateRequest(req); len(violations) > 0 {
			return violations[0].Error
		}
		return nil
	}
	state, err := pipeline.Select(record, f.statePointer)
	if err != nil {
		return gev.NewError(gev.CodeInputInvalid, err.Error())
	}
	if stateErr := contract.CheckStateValue(state); stateErr != nil {
		return stateErr
	}
	if model == "" {
		return gev.NewError(gev.CodeInputInvalid, "model is required in composed mode")
	}
	req := contract.Request{Model: model, State: state, Questions: questions, Extra: extra}
	if violations := contract.ValidateRequest(req); len(violations) > 0 {
		return violations[0].Error
	}
	return nil
}

func processMapInput(ctx context.Context, cmd *cobra.Command, renderer Renderer, input []byte, f mapFlags, questions map[string]contract.Question, extra map[string]json.RawMessage, model string, evaluator pipeline.Evaluator) error {
	records, framingErr := mapRecords(input, f.input)
	if framingErr != nil {
		return framingErr
	}
	for _, record := range records {
		var output []byte
		var err *gev.Error
		if f.requestPointerSet {
			raw, selectErr := pipeline.Select(record, f.requestPointer)
			if selectErr != nil {
				err = gev.NewError(gev.CodeInputInvalid, selectErr.Error())
			} else {
				req, decodeErr := contract.DecodeRequest(raw)
				if decodeErr != nil {
					err = decodeErr
				} else if violations := contract.ValidateRequest(req); len(violations) > 0 {
					err = violations[0].Error
				} else {
					output, err = pipeline.EnrichRequest(ctx, record, f.name, req, evaluator)
				}
			}
		} else {
			state, selectErr := pipeline.Select(record, f.statePointer)
			if selectErr != nil {
				err = gev.NewError(gev.CodeInputInvalid, selectErr.Error())
			} else if stateErr := contract.CheckStateValue(state); stateErr != nil {
				err = stateErr
			} else {
				req := contract.Request{Model: model, State: state, Questions: questions, Extra: extra}
				output, err = pipeline.EnrichRequest(ctx, record, f.name, req, evaluator)
			}
		}
		if err != nil {
			return renderStreamError(renderer, cmd.OutOrStdout(), err, f.input == "ndjson")
		}
		if writeErr := renderRaw(renderer, cmd.OutOrStdout(), output); writeErr != nil {
			return gev.WrapError(gev.CodeResponseInvalid, writeErr, "writing map output")
		}
	}
	return nil
}

func mapRecords(input []byte, framing string) ([][]byte, *gev.Error) {
	if framing == "json" {
		if len(input) > MapMaxRecordBytes {
			return nil, gev.NewError(gev.CodeInputInvalid, "JSON record exceeds the byte limit")
		}
		return [][]byte{bytes.TrimSpace(input)}, nil
	}
	lines := bytes.Split(input, []byte{'\n'})
	records := make([][]byte, 0, len(lines))
	for _, line := range lines {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		if len(line) > MapMaxRecordBytes {
			return nil, gev.NewError(gev.CodeInputInvalid, "NDJSON record exceeds the byte limit")
		}
		if len(records) >= MapMaxRecords {
			return nil, gev.NewError(gev.CodeInputInvalid, "NDJSON record count exceeds the limit")
		}
		records = append(records, line)
	}
	if len(records) == 0 {
		return nil, gev.NewError(gev.CodeInputInvalid, "NDJSON input is empty")
	}
	return records, nil
}

func renderRaw(renderer Renderer, w io.Writer, raw []byte) error {
	if rr, ok := renderer.(RawRenderer); ok {
		return rr.RenderRaw(w, raw)
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return err
	}
	return renderer.(ValueRenderer).RenderValue(w, value)
}

func renderStreamError(renderer Renderer, w io.Writer, err *gev.Error, framed bool) error {
	if framed {
		if renderErr := renderer.RenderError(w, err.WithRecovery("fix this record and retry the stream")); renderErr != nil {
			return gev.WrapError(gev.CodeResponseInvalid, renderErr, "writing map error")
		}
		return &renderedError{err: err}
	}
	return err
}
