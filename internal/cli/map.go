package cli

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/cristianoliveira/jeq/internal/domain/contract"
	"github.com/cristianoliveira/jeq/internal/domain/jeq"
	"github.com/cristianoliveira/jeq/internal/domain/pipeline"
	"github.com/cristianoliveira/jeq/internal/trace"
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
	var name, input, statePointer, requestPointer, model, timeoutText string
	var source questionSourceFlags
	var maxRetries int
	cmd := &cobra.Command{
		Use:   "map",
		Short: "Enrich each JSON record with one named judgment",
		Long:  "Jev supplies typed semantic evidence per record; the caller owns state, deterministic policy, and actions.",
		Args:  cobra.NoArgs,
		Example: `  printf '%s\n' '{"change":"small"}' | jeq map --as risk --state-pointer /change --questions-json '{"questions":{"risk":{"type":"noul","instructions":"Is this low risk?"}}}'
  jeq examples map-gate`,
		RunE: withBareHelp(func(cmd *cobra.Command, _ []string) error {
			if err := runMap(cmd, deps, mapFlags{
				name: name, input: input, source: source, statePointer: statePointer,
				requestPointer: requestPointer, model: model,
				timeout: timeoutText, maxRetries: maxRetries,
				nameSet: cmd.Flags().Changed("as"), inputSet: cmd.Flags().Changed("input"),
				requestPointerSet: cmd.Flags().Changed("request-pointer"),
				modelSet:          cmd.Flags().Changed("model"),
			}); err != nil {
				return err
			}
			return nil
		}),
	}
	flags := cmd.Flags()
	flags.StringVar(&name, "as", "", "evidence name (required)")
	flags.StringVar(&input, "input", "json", "input framing: json or ndjson")
	addQuestionSourceFlags(flags, &source)
	flags.StringVar(&statePointer, "state-pointer", "", "RFC 6901 pointer to state")
	flags.StringVar(&requestPointer, "request-pointer", "", "RFC 6901 pointer to a native request")
	flags.StringVar(&model, "model", "", "composed-mode model")
	flags.StringVar(&timeoutText, "timeout", DefaultTimeout.String(), "request timeout")
	flags.IntVar(&maxRetries, "max-retries", DefaultMaxRetries, "maximum retries (0-5)")
	return cmd
}

type mapFlags struct {
	name, input, statePointer, requestPointer, model, timeout string
	source                                                    questionSourceFlags
	maxRetries                                                int
	nameSet, inputSet, requestPointerSet, modelSet            bool
}

func runMap(cmd *cobra.Command, deps AskDeps, f mapFlags) error {
	if !f.nameSet || f.name == "" {
		return jeq.NewError(jeq.CodeInputInvalid, "--as is required")
	}
	if err := pipeline.ValidateName(f.name); err != nil {
		return jeq.NewError(jeq.CodeInputInvalid, err.Error())
	}
	if err := pipeline.ValidatePointer(f.statePointer); err != nil {
		return jeq.NewError(jeq.CodeInputInvalid, err.Error())
	}
	if err := pipeline.ValidatePointer(f.requestPointer); err != nil {
		return jeq.NewError(jeq.CodeInputInvalid, err.Error())
	}
	if f.input != "json" && f.input != "ndjson" {
		return jeq.NewError(jeq.CodeInputInvalid, "--input must be json or ndjson")
	}
	f.source.fileSet = cmd.Flags().Changed("questions")
	f.source.inlineSet = cmd.Flags().Changed("questions-json")
	if f.requestPointerSet == (f.source.fileSet || f.source.inlineSet) {
		return jeq.NewError(jeq.CodeInputInvalid, "choose exactly one of --questions/--questions-json or --request-pointer")
	}
	if err := checkQuestionSource(f.source); err != nil && !f.requestPointerSet {
		return err
	}
	if f.requestPointerSet && (f.modelSet || f.statePointer != "") {
		return jeq.NewError(jeq.CodeInputInvalid, "--request-pointer cannot be combined with --model or --state-pointer")
	}

	if f.maxRetries < 0 || f.maxRetries > MaxRetriesLimit {
		return jeq.NewError(jeq.CodeInputInvalid, fmt.Sprintf("--max-retries must be between 0 and %d", MaxRetriesLimit))
	}
	timeout, err := time.ParseDuration(f.timeout)
	if err != nil || timeout <= 0 {
		return jeq.NewError(jeq.CodeInputInvalid, "--timeout must be a positive duration")
	}
	if deps.Stdin == nil || deps.ReadStdin == nil || deps.ReadFile == nil || deps.NewClient == nil || deps.Getenv == nil {
		return jeq.NewError(jeq.CodeInputInvalid, "map dependencies are unavailable")
	}

	var inputDoc []byte
	var readErr *jeq.Error
	var stream *bufio.Reader
	var firstRecord []byte
	if f.input == "ndjson" {
		stream = bufio.NewReader(deps.Stdin)
		var eof bool
		firstRecord, eof, readErr = readNDJSONRecord(stream)
		if readErr != nil {
			return readErr
		}
		if eof {
			return jeq.NewError(jeq.CodeInputInvalid, "input is empty")
		}
	} else {
		inputDoc, readErr = deps.ReadStdin(deps.Stdin, MapMaxInputBytes, true)
		if readErr != nil {
			return readErr
		}
	}
	var questions map[string]contract.Question
	var extra map[string]json.RawMessage
	if f.source.fileSet || f.source.inlineSet {
		var sourceErr *jeq.Error
		questions, extra, sourceErr = readQuestionSource(deps, f.source)
		if sourceErr != nil {
			return sourceErr
		}
	}
	var records [][]byte
	if f.input == "ndjson" {
		records = [][]byte{firstRecord}
	} else {
		var framingErr *jeq.Error
		records, framingErr = mapRecords(inputDoc, f.input)
		if framingErr != nil {
			return framingErr
		}
	}
	resolvedModel := "native"
	modelSource := "native"
	if f.source.fileSet || f.source.inlineSet {
		var modelErr *jeq.Error
		resolvedModel, modelSource, modelErr = ResolveConfiguredModelWithSource(f.model, strings.TrimSpace(deps.Getenv("JEQ_CONFIG")), deps.Getenv, deps.ReadFile, deps.ReadOptionalFile)
		if modelErr != nil {
			return modelErr
		}
	}
	if err := validateMapRecord(records[0], f, questions, extra, resolvedModel); err != nil {
		return withTracePhase(err, "state_selection")
	}

	provider, providerErr := ResolveProvider(deps.Getenv, deps.ReadFile, deps.ReadOptionalFile)
	if providerErr != nil {
		return providerErr
	}
	if f.source.fileSet || f.source.inlineSet {
		if resolvedModel == "" {
			return jeq.NewError(jeq.CodeInputInvalid, "model is required in composed mode")
		}
	}
	traceMetadata(cmd, resolvedModel, modelSource, f.input, f.statePointer, questions)
	client := newClient(deps, provider, timeout, f.maxRetries, func(line string) { _, _ = fmt.Fprintln(cmd.ErrOrStderr(), line) })
	attachTrace(cmd, client)
	evaluator := client
	if f.input == "ndjson" {
		return processMapStream(cmd.Context(), cmd, deps.Renderer, stream, firstRecord, f, questions, extra, resolvedModel, evaluator)
	}
	if processErr := processMapInput(cmd.Context(), cmd, deps.Renderer, inputDoc, f, questions, extra, resolvedModel, evaluator, 0); processErr != nil {
		return processErr
	}
	return nil
}

func validateMapRecord(record []byte, f mapFlags, questions map[string]contract.Question, extra map[string]json.RawMessage, model string) *jeq.Error {
	if err := pipeline.ValidateRecord(record); err != nil {
		return jeq.NewError(jeq.CodeInputInvalid, err.Error())
	}
	if f.requestPointerSet {
		raw, err := pipeline.Select(record, f.requestPointer)
		if err != nil {
			return jeq.NewError(jeq.CodeInputInvalid, err.Error())
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
		return jeq.NewError(jeq.CodeInputInvalid, err.Error())
	}
	if stateErr := contract.CheckStateValue(state); stateErr != nil {
		return stateErr
	}
	if model == "" {
		return jeq.NewError(jeq.CodeInputInvalid, "model is required in composed mode")
	}
	req := contract.Request{Model: model, State: state, Questions: questions, Extra: extra}
	if violations := contract.ValidateRequest(req); len(violations) > 0 {
		return violations[0].Error
	}
	return nil
}

func processMapInput(ctx context.Context, cmd *cobra.Command, renderer Renderer, input []byte, f mapFlags, questions map[string]contract.Question, extra map[string]json.RawMessage, model string, evaluator pipeline.Evaluator, offset int) error {
	records, framingErr := mapRecords(input, f.input)
	if framingErr != nil {
		return framingErr
	}
	tr := trace.FromContext(ctx)
	for index, record := range records {
		if tr != nil {
			tr.EmitOperation(cmd.CommandPath(), "operation.started", "evaluation", "started", "", index+1+offset, len(records)+offset)
		}
		var output []byte
		var err *jeq.Error
		if f.requestPointerSet {
			raw, selectErr := pipeline.Select(record, f.requestPointer)
			if selectErr != nil {
				if tr != nil {
					tr.EmitOperation(cmd.CommandPath(), "run.failed", "state_selection", "failed", string(jeq.CodeInputInvalid), index+1+offset, len(records)+offset)
				}
				err = jeq.NewError(jeq.CodeInputInvalid, selectErr.Error())
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
				if tr != nil {
					tr.EmitOperation(cmd.CommandPath(), "run.failed", "state_selection", "failed", string(jeq.CodeInputInvalid), index+1+offset, len(records)+offset)
				}
				err = jeq.NewError(jeq.CodeInputInvalid, selectErr.Error())
			} else if stateErr := contract.CheckStateValue(state); stateErr != nil {
				err = stateErr
			} else {
				req := contract.Request{Model: model, State: state, Questions: questions, Extra: extra}
				output, err = pipeline.EnrichRequest(ctx, record, f.name, req, evaluator)
			}
		}
		if err != nil {
			if tr != nil {
				tr.EmitOperation(cmd.CommandPath(), "run.failed", "evaluation", "failed", string(err.Code), index+1+offset, len(records)+offset)
			}
			return renderStreamError(renderer, cmd.OutOrStdout(), err, f.input == "ndjson")
		}
		if writeErr := renderRaw(renderer, cmd.OutOrStdout(), output); writeErr != nil {
			if tr != nil {
				tr.EmitOperation(cmd.CommandPath(), "run.failed", "output", "failed", string(jeq.CodeResponseInvalid), index+1+offset, len(records)+offset)
			}
			return withTracePhase(jeq.WrapError(jeq.CodeResponseInvalid, writeErr, "writing map output"), "output_write")
		}
		if tr != nil {
			tr.EmitOperation(cmd.CommandPath(), "operation.completed", "evaluation", "success", "", index+1+offset, len(records)+offset)
			tr.EmitOperation(cmd.CommandPath(), "output.written", "output", "success", "", index+1+offset, len(records)+offset)
		}
	}
	if tr != nil {
		tr.EmitSummary(cmd.CommandPath(), len(records)+offset, len(records)+offset, len(records)+offset, 0)
	}
	return nil
}

func readNDJSONRecord(reader *bufio.Reader) ([]byte, bool, *jeq.Error) {
	var record []byte
	for {
		part, err := reader.ReadSlice('\n')
		if len(record)+len(part) > MapMaxRecordBytes {
			return nil, false, jeq.NewError(jeq.CodeInputInvalid, "NDJSON record exceeds the byte limit")
		}
		record = append(record, part...)
		if len(record) > 0 && record[len(record)-1] == '\n' {
			if strings.TrimSpace(string(record)) != "" {
				return bytes.TrimSpace(record), false, nil
			}
			record = record[:0]
		}
		if err == io.EOF {
			if len(record) == 0 {
				return nil, true, nil
			}
			if strings.TrimSpace(string(record)) != "" {
				return bytes.TrimSpace(record), false, nil
			}
			return nil, true, nil
		}
		if err != nil && err != bufio.ErrBufferFull {
			return nil, false, jeq.NewError(jeq.CodeInputInvalid, "reading NDJSON input")
		}
	}
}

func processMapStream(ctx context.Context, cmd *cobra.Command, renderer Renderer, reader *bufio.Reader, first []byte, f mapFlags, questions map[string]contract.Question, extra map[string]json.RawMessage, model string, evaluator pipeline.Evaluator) error {
	record := first
	offset := 0
	for {
		if ctx.Err() != nil {
			return jeq.NewError(jeq.CodeInterrupted, "map input cancelled")
		}
		if err := processMapInput(ctx, cmd, renderer, record, f, questions, extra, model, evaluator, offset); err != nil {
			return err
		}
		next, eof, readErr := readNDJSONRecord(reader)
		if readErr != nil {
			return readErr
		}
		if eof {
			return nil
		}
		record = next
		offset++
	}
}

func mapRecords(input []byte, framing string) ([][]byte, *jeq.Error) {
	if framing == "json" {
		if len(input) > MapMaxRecordBytes {
			return nil, jeq.NewError(jeq.CodeInputInvalid, "JSON record exceeds the byte limit")
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
			return nil, jeq.NewError(jeq.CodeInputInvalid, "NDJSON record exceeds the byte limit")
		}
		if len(records) >= MapMaxRecords {
			return nil, jeq.NewError(jeq.CodeInputInvalid, "NDJSON record count exceeds the limit")
		}
		records = append(records, line)
	}
	if len(records) == 0 {
		return nil, jeq.NewError(jeq.CodeInputInvalid, "NDJSON input is empty")
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

func renderStreamError(_ Renderer, _ io.Writer, err *jeq.Error, framed bool) error {
	if framed {
		return err.WithRecovery("fix this record and retry the stream")
	}
	return err
}
