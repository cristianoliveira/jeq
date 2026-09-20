package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/cristianoliveira/jeq/internal/domain/contract"
	"github.com/cristianoliveira/jeq/internal/domain/jeq"
	"github.com/cristianoliveira/jeq/internal/domain/pipeline"
	"github.com/cristianoliveira/jeq/internal/trace"
	"github.com/spf13/cobra"
)

// ReduceMaxItems bounds the number of aggregate values.
const ReduceMaxItems = MapMaxRecords

// NewReduceCmd creates the one-call bounded aggregate primitive.
func NewReduceCmd(deps AskDeps) *cobra.Command {
	var name, input, model, timeoutText string
	var source questionSourceFlags
	var maxRetries int
	cmd := &cobra.Command{
		Use:   "reduce",
		Short: "Aggregate JSON records with one named judgment",
		Args:  cobra.NoArgs,
		Example: `  printf '%s\n' '{"id":"a"}' '{"id":"b"}' | jeq reduce --as coherent --input ndjson --questions-json '{"questions":{"coherent":{"type":"noul","instructions":"Is this coherent?"}}}'
  jeq examples reduce-gate`,
		RunE: withBareHelp(func(cmd *cobra.Command, _ []string) error {
			source.fileSet = cmd.Flags().Changed("questions")
			source.inlineSet = cmd.Flags().Changed("questions-json")
			return runReduce(cmd, deps, reduceFlags{
				name: name, input: input, source: source, model: model, timeout: timeoutText, maxRetries: maxRetries,
				nameSet: cmd.Flags().Changed("as"), inputSet: cmd.Flags().Changed("input"), modelSet: cmd.Flags().Changed("model"),
			})
		}),
	}
	flags := cmd.Flags()
	flags.StringVar(&name, "as", "", "evidence name (required)")
	flags.StringVar(&input, "input", "json", "input framing: json or ndjson")
	addQuestionSourceFlags(flags, &source)
	flags.StringVar(&model, "model", "", "model override")
	flags.StringVar(&timeoutText, "timeout", DefaultTimeout.String(), "request timeout")
	flags.IntVar(&maxRetries, "max-retries", DefaultMaxRetries, "maximum retries (0-5)")
	return cmd
}

type reduceFlags struct {
	name, input, model, timeout string
	source                      questionSourceFlags
	maxRetries                  int
	nameSet, inputSet, modelSet bool
}

func runReduce(cmd *cobra.Command, deps AskDeps, f reduceFlags) error {
	if !f.nameSet || f.name == "" {
		return jeq.NewError(jeq.CodeInputInvalid, "--as is required")
	}
	if err := pipeline.ValidateName(f.name); err != nil {
		return jeq.NewError(jeq.CodeInputInvalid, err.Error())
	}
	if f.input != "json" && f.input != "ndjson" {
		return jeq.NewError(jeq.CodeInputInvalid, "--input must be json or ndjson")
	}
	if err := checkQuestionSource(f.source); err != nil {
		return err
	}
	if f.maxRetries < 0 || f.maxRetries > MaxRetriesLimit {
		return jeq.NewError(jeq.CodeInputInvalid, fmt.Sprintf("--max-retries must be between 0 and %d", MaxRetriesLimit))
	}
	timeout, err := time.ParseDuration(f.timeout)
	if err != nil || timeout <= 0 {
		return jeq.NewError(jeq.CodeInputInvalid, "--timeout must be a positive duration")
	}
	if deps.Stdin == nil || deps.ReadStdin == nil || deps.ReadFile == nil || deps.NewClient == nil || deps.Getenv == nil {
		return jeq.NewError(jeq.CodeInputInvalid, "reduce dependencies are unavailable")
	}

	inputDoc, readErr := deps.ReadStdin(deps.Stdin, MapMaxInputBytes, true)
	if readErr != nil {
		return readErr
	}
	items, collectionErr := reduceItems(inputDoc, f.input)
	if collectionErr != nil {
		return collectionErr
	}
	questions, extra, sourceErr := readQuestionSource(deps, f.source)
	if sourceErr != nil {
		return sourceErr
	}
	resolvedModel, _, modelErr := ResolveConfiguredModelWithSource(f.model, strings.TrimSpace(deps.Getenv("JEQ_CONFIG")), deps.Getenv, deps.ReadFile, deps.ReadOptionalFile)
	if modelErr != nil {
		return modelErr
	}
	if strings.TrimSpace(resolvedModel) == "" {
		return jeq.NewError(jeq.CodeInputInvalid, "model is required")
	}

	rootURL, rootErr := ResolveBaseURL(deps.Getenv)
	if rootErr != nil {
		return rootErr
	}

	apiKey := strings.TrimSpace(deps.Getenv("TYPESAFE_API_KEY"))
	if apiKey == "" {
		return jeq.NewError(jeq.CodeAuthMissing, "TYPESAFE_API_KEY is not set")
	}
	traceMetadata(cmd, resolvedModel, "", f.input, "", questions)
	client := deps.NewClient(rootURL, timeout, apiKey, f.maxRetries, func(line string) { _, _ = fmt.Fprintln(cmd.ErrOrStderr(), line) })
	attachTrace(cmd, client)
	request := contract.Request{Model: resolvedModel, State: items, Questions: questions, Extra: extra}
	output, evalErr := pipeline.Reduce(cmd.Context(), items, f.name, request, client)
	if evalErr != nil {
		return evalErr
	}
	if tr := trace.FromContext(cmd.Context()); tr != nil {
		tr.EmitSummary(cmd.CommandPath(), len(items), 1, 1, 0)
	}
	return renderRaw(deps.Renderer, cmd.OutOrStdout(), output)
}

func reduceItems(input []byte, framing string) ([]byte, *jeq.Error) {
	if framing != "json" && framing != "ndjson" {
		return nil, jeq.NewError(jeq.CodeInputInvalid, "--input must be json or ndjson")
	}
	if framing == "json" {
		trimmed := bytes.TrimSpace(input)
		if len(trimmed) > MapMaxRecordBytes {
			return nil, jeq.NewError(jeq.CodeInputInvalid, "JSON collection exceeds the byte limit")
		}
		if err := contract.ValidateJSON(trimmed); err != nil {
			return nil, jeq.NewError(jeq.CodeInputInvalid, err.Error())
		}
		if len(trimmed) < 2 || trimmed[0] != '[' {
			return nil, jeq.NewError(jeq.CodeInputInvalid, "JSON reduce input must be an array")
		}
		var values []json.RawMessage
		if err := json.Unmarshal(trimmed, &values); err != nil {
			return nil, jeq.NewError(jeq.CodeInputInvalid, err.Error())
		}
		if len(values) == 0 {
			return nil, jeq.NewError(jeq.CodeInputInvalid, "reduce collection is empty")
		}
		if len(values) > ReduceMaxItems {
			return nil, jeq.NewError(jeq.CodeInputInvalid, "reduce collection exceeds the item limit")
		}
		return trimmed, nil
	}
	lines := bytes.Split(input, []byte{'\n'})
	items := make([][]byte, 0, len(lines))
	for _, line := range lines {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		if len(line) > MapMaxRecordBytes {
			return nil, jeq.NewError(jeq.CodeInputInvalid, "NDJSON item exceeds the byte limit")
		}
		if len(items) >= ReduceMaxItems {
			return nil, jeq.NewError(jeq.CodeInputInvalid, "reduce collection exceeds the item limit")
		}
		if err := contract.ValidateJSON(line); err != nil {
			return nil, jeq.NewError(jeq.CodeInputInvalid, err.Error())
		}
		items = append(items, line)
	}
	if len(items) == 0 {
		return nil, jeq.NewError(jeq.CodeInputInvalid, "reduce collection is empty")
	}
	var out bytes.Buffer
	out.WriteByte('[')
	for i, item := range items {
		if i > 0 {
			out.WriteByte(',')
		}
		out.Write(item)
	}
	out.WriteByte(']')
	return out.Bytes(), nil
}
