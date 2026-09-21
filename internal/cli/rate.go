package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/cristianoliveira/jeq/internal/domain/contract"
	"github.com/cristianoliveira/jeq/internal/domain/jeq"
	"github.com/cristianoliveira/jeq/internal/domain/pipeline"
	"github.com/spf13/cobra"
)

// NewRateCmd creates the ergonomic Score adapter over the map pipeline.
func NewRateCmd(deps AskDeps) *cobra.Command {
	var name, input, statePointer, instruction, model, timeoutText string
	var levels []string
	var maxRetries int
	cmd := &cobra.Command{
		Use: "rate", Short: "Rate each JSON record with a TypeSafe Score rubric", Long: "Jev supplies typed semantic evidence per record; the caller owns deterministic policy and actions. Sends one Score question per record through the same map pipeline. Score is an independent semantic rating against the ordered levels; it is not an exact measurement.", Args: cobra.NoArgs,
		Example: `  cat issues.ndjson | jeq rate --as severity --input ndjson --state-pointer /description --instruction 'How severe is this issue?' --level 'Cosmetic: no impact' --level 'Critical: service at risk'`,
		RunE: withBareHelp(func(cmd *cobra.Command, _ []string) error {
			return runRate(cmd, deps, rateFlags{name: name, input: input, statePointer: statePointer, instruction: instruction, levels: levels, model: model, timeout: timeoutText, maxRetries: maxRetries, nameSet: cmd.Flags().Changed("as"), instructionSet: cmd.Flags().Changed("instruction"), statePointerSet: cmd.Flags().Changed("state-pointer"), modelSet: cmd.Flags().Changed("model")})
		}),
	}
	flags := cmd.Flags()
	flags.StringVar(&name, "as", "", "evidence name (required)")
	flags.StringVar(&input, "input", "json", "input framing: json or ndjson")
	flags.StringVar(&statePointer, "state-pointer", "", "RFC 6901 pointer to state (required)")
	flags.StringVar(&instruction, "instruction", "", "Score instruction (required)")
	flags.StringArrayVar(&levels, "level", nil, "ordered Score level description (repeat at least twice)")
	flags.StringVar(&model, "model", "", "composed-mode model")
	flags.StringVar(&timeoutText, "timeout", DefaultTimeout.String(), "request timeout")
	flags.IntVar(&maxRetries, "max-retries", DefaultMaxRetries, "maximum retries (0-5)")
	return cmd
}

type rateFlags struct {
	name, input, statePointer, instruction, model, timeout string
	levels                                                 []string
	maxRetries                                             int
	nameSet, instructionSet, statePointerSet, modelSet     bool
}

func runRate(cmd *cobra.Command, deps AskDeps, f rateFlags) error {
	if !f.nameSet || strings.TrimSpace(f.name) == "" || !f.statePointerSet || !f.instructionSet || strings.TrimSpace(f.instruction) == "" {
		return jeq.NewError(jeq.CodeInputInvalid, "--as, --state-pointer, and --instruction are required")
	}
	if len(f.levels) < 2 {
		return jeq.NewError(jeq.CodeInputInvalid, "at least two --level values are required")
	}
	for _, level := range f.levels {
		if strings.TrimSpace(level) == "" {
			return jeq.NewError(jeq.CodeInputInvalid, "--level values cannot be blank")
		}
	}
	if err := pipeline.ValidateName(f.name); err != nil {
		return jeq.NewError(jeq.CodeInputInvalid, err.Error())
	}
	if err := pipeline.ValidatePointer(f.statePointer); err != nil {
		return jeq.NewError(jeq.CodeInputInvalid, err.Error())
	}
	if f.input != "json" && f.input != "ndjson" {
		return jeq.NewError(jeq.CodeInputInvalid, "--input must be json or ndjson")
	}
	if f.maxRetries < 0 || f.maxRetries > MaxRetriesLimit {
		return jeq.NewError(jeq.CodeInputInvalid, fmt.Sprintf("--max-retries must be between 0 and %d", MaxRetriesLimit))
	}
	timeout, err := time.ParseDuration(f.timeout)
	if err != nil || timeout <= 0 {
		return jeq.NewError(jeq.CodeInputInvalid, "--timeout must be a positive duration")
	}
	if deps.Stdin == nil || deps.ReadStdin == nil || deps.ReadFile == nil || deps.NewClient == nil || deps.Getenv == nil {
		return jeq.NewError(jeq.CodeInputInvalid, "rate dependencies are unavailable")
	}
	inputDoc, readErr := deps.ReadStdin(deps.Stdin, MapMaxInputBytes, true)
	if readErr != nil {
		return readErr
	}
	records, framingErr := mapRecords(inputDoc, f.input)
	if framingErr != nil {
		return framingErr
	}
	resolvedModel, modelSource, modelErr := ResolveConfiguredModelWithSource(f.model, strings.TrimSpace(deps.Getenv("JEQ_CONFIG")), deps.Getenv, deps.ReadFile, deps.ReadOptionalFile)
	if modelErr != nil {
		return modelErr
	}
	criteria, _ := json.Marshal(f.levels)
	question := contract.Question{Type: contract.TypeScore, Instructions: json.RawMessage(strconvQuote(f.instruction)), Criteria: criteria}
	mf := mapFlags{name: f.name, input: f.input, statePointer: f.statePointer, model: resolvedModel, nameSet: true}
	if err := validateMapRecord(records[0], mf, map[string]contract.Question{f.name: question}, nil, resolvedModel); err != nil {
		return err
	}
	rootURL, rootErr := ResolveBaseURL(deps.Getenv)
	if rootErr != nil {
		return rootErr
	}

	apiKey := strings.TrimSpace(deps.Getenv("TYPESAFE_API_KEY"))
	if apiKey == "" {
		return jeq.NewError(jeq.CodeAuthMissing, "TYPESAFE_API_KEY is not set")
	}
	traceMetadata(cmd, resolvedModel, modelSource, f.input, f.statePointer, map[string]contract.Question{f.name: question})
	client := deps.NewClient(rootURL, timeout, apiKey, f.maxRetries, func(line string) { _, _ = fmt.Fprintln(cmd.ErrOrStderr(), line) })
	attachTrace(cmd, client)
	return processMapInput(cmd.Context(), cmd, deps.Renderer, inputDoc, mf, map[string]contract.Question{f.name: question}, nil, resolvedModel, client)
}
