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

// PickMaxOptions is the TypeSafe Choice option limit.
const PickMaxOptions = 255

// NewPickCmd creates the one-request candidate selection primitive.
func NewPickCmd(deps AskDeps) *cobra.Command {
	var name, input, state, stateFile, stateJSON, instruction, idPointer, criteriaPointer, model, config, baseURL, timeoutText string
	var maxRetries int
	cmd := &cobra.Command{
		Use: "pick", Short: "Select one candidate with a TypeSafe Choice", Long: "Selects one original candidate and attaches complete Choice evidence under _jeq.<as>. Choice is relative: include an explicit fallback candidate when nothing may fit. Pipe the result unchanged into jq or another explicit JEQ stage.", Args: cobra.NoArgs,
		Example: `  cat handlers.ndjson | jeq pick --as route --state-file request.txt --instruction 'Which handler best fits this request?' --id-pointer /name --criteria-pointer /description`,
		RunE: withBareHelp(func(cmd *cobra.Command, _ []string) error {
			return runPick(cmd, deps, pickFlags{
				name: name, input: input, state: state, stateFile: stateFile, stateJSON: stateJSON, instruction: instruction, idPointer: idPointer, criteriaPointer: criteriaPointer, model: model, config: config, baseURL: baseURL, timeout: timeoutText, maxRetries: maxRetries,
				nameSet: cmd.Flags().Changed("as"), inputSet: cmd.Flags().Changed("input"), stateSet: cmd.Flags().Changed("state"), stateFileSet: cmd.Flags().Changed("state-file"), stateJSONSet: cmd.Flags().Changed("state-json"), instructionSet: cmd.Flags().Changed("instruction"), idPointerSet: cmd.Flags().Changed("id-pointer"), criteriaPointerSet: cmd.Flags().Changed("criteria-pointer"), modelSet: cmd.Flags().Changed("model"), configSet: cmd.Flags().Changed("config"), baseURLSet: cmd.Flags().Changed("base-url"),
			})
		}),
	}
	flags := cmd.Flags()
	flags.StringVar(&name, "as", "", "evidence name (required)")
	flags.StringVar(&input, "input", "json", "input framing: json or ndjson")
	flags.StringVar(&state, "state", "", "literal state text")
	flags.StringVar(&stateFile, "state-file", "", "state text file or -")
	flags.StringVar(&stateJSON, "state-json", "", "state JSON file or -")
	flags.StringVar(&instruction, "instruction", "", "single Choice question instruction")
	flags.StringVar(&idPointer, "id-pointer", "", "pointer to each candidate's unique string id")
	flags.StringVar(&criteriaPointer, "criteria-pointer", "", "pointer to each candidate's Choice criteria")
	flags.StringVar(&model, "model", "", "model override")
	flags.StringVar(&config, "config", "", "user config JSON path")
	flags.StringVar(&baseURL, "base-url", "", "TypeSafe API root")
	flags.StringVar(&timeoutText, "timeout", DefaultTimeout.String(), "request timeout")
	flags.IntVar(&maxRetries, "max-retries", DefaultMaxRetries, "maximum retries (0-5)")
	return cmd
}

type pickFlags struct {
	name, input, state, stateFile, stateJSON, instruction, idPointer, criteriaPointer, model, config, baseURL, timeout                         string
	maxRetries                                                                                                                                 int
	nameSet, inputSet, stateSet, stateFileSet, stateJSONSet, instructionSet, idPointerSet, criteriaPointerSet, modelSet, configSet, baseURLSet bool
}

func runPick(cmd *cobra.Command, deps AskDeps, f pickFlags) error {
	if !f.nameSet || f.name == "" || !f.instructionSet || strings.TrimSpace(f.instruction) == "" || !f.idPointerSet || !f.criteriaPointerSet {
		return jeq.NewError(jeq.CodeInputInvalid, "--as, --instruction, --id-pointer, and --criteria-pointer are required")
	}
	if err := pipeline.ValidateName(f.name); err != nil {
		return jeq.NewError(jeq.CodeInputInvalid, err.Error())
	}
	if err := pipeline.ValidatePointer(f.idPointer); err != nil {
		return jeq.NewError(jeq.CodeInputInvalid, err.Error())
	}
	if err := pipeline.ValidatePointer(f.criteriaPointer); err != nil {
		return jeq.NewError(jeq.CodeInputInvalid, err.Error())
	}
	if f.input != "json" && f.input != "ndjson" {
		return jeq.NewError(jeq.CodeInputInvalid, "--input must be json or ndjson")
	}
	stateSources := 0
	if f.stateSet {
		stateSources++
	}
	if f.stateFileSet {
		stateSources++
	}
	if f.stateJSONSet {
		stateSources++
	}
	if stateSources > 1 {
		return jeq.NewError(jeq.CodeSourceConflict, "choose exactly one of --state, --state-file, or --state-json")
	}
	if stateSources == 0 {
		return jeq.NewError(jeq.CodeInputInvalid, "one of --state, --state-file, or --state-json is required")
	}
	if (f.stateFileSet && f.stateFile == "-") || (f.stateJSONSet && f.stateJSON == "-") {
		return jeq.NewError(jeq.CodeSourceConflict, "candidate input stdin cannot also be used as state stdin")
	}
	if f.maxRetries < 0 || f.maxRetries > MaxRetriesLimit {
		return jeq.NewError(jeq.CodeInputInvalid, fmt.Sprintf("--max-retries must be between 0 and %d", MaxRetriesLimit))
	}
	timeout, err := time.ParseDuration(f.timeout)
	if err != nil || timeout <= 0 {
		return jeq.NewError(jeq.CodeInputInvalid, "--timeout must be a positive duration")
	}
	if deps.Stdin == nil || deps.ReadStdin == nil || deps.ReadFile == nil || deps.NewClient == nil || deps.Getenv == nil {
		return jeq.NewError(jeq.CodeInputInvalid, "pick dependencies are unavailable")
	}
	stateBytes, stateErr := pickState(deps, f)
	if stateErr != nil {
		return stateErr
	}
	candidatesBytes, readErr := deps.ReadStdin(deps.Stdin, MapMaxInputBytes, true)
	if readErr != nil {
		return readErr
	}
	records, framingErr := mapRecords(candidatesBytes, f.input)
	if framingErr != nil {
		return framingErr
	}
	if len(records) > PickMaxOptions {
		return jeq.NewError(jeq.CodeInputInvalid, "candidate count exceeds the 255 Choice option limit")
	}
	ids := make(map[string]struct{}, len(records))
	criteria := make(map[string]json.RawMessage, len(records))
	for _, record := range records {
		if slotErr := pipeline.CheckEvidenceAvailable(record, f.name); slotErr != nil {
			return slotErr
		}
		id, criteriaValue, err := pickCandidate(record, f.idPointer, f.criteriaPointer)
		if err != nil {
			return err
		}
		if _, exists := ids[id]; exists {
			return jeq.NewError(jeq.CodeInputInvalid, fmt.Sprintf("duplicate candidate id %q", id))
		}
		ids[id] = struct{}{}
		criteria[id] = criteriaValue
	}
	criteriaJSON, _ := json.Marshal(criteria)
	stateValue := json.RawMessage(stateBytes)
	if f.stateSet {
		stateValue = json.RawMessage(strconvQuote(f.state))
	}
	if f.stateFileSet {
		stateValue = json.RawMessage(stateBytes)
	}
	if f.stateJSONSet {
		if err := json.Unmarshal(stateBytes, &stateValue); err != nil {
			return jeq.NewError(jeq.CodeInputInvalid, "state JSON is invalid")
		}
	}
	if err := contract.CheckStateValue(stateValue); err != nil {
		return err
	}
	resolvedModel, _, modelErr := ResolveConfiguredModelWithSource(f.model, f.config, deps.Getenv, deps.ReadFile, deps.ReadOptionalFile)
	if modelErr != nil {
		return modelErr
	}
	request := contract.Request{Model: resolvedModel, State: stateValue, Questions: map[string]contract.Question{f.name: {Type: contract.TypeChoice, Instructions: json.RawMessage(strconvQuote(f.instruction)), Criteria: criteriaJSON}}}
	if violations := contract.ValidateRequest(request); len(violations) > 0 {
		return violations[0].Error
	}
	apiKey := strings.TrimSpace(deps.Getenv("TYPESAFE_API_KEY"))
	if apiKey == "" {
		return jeq.NewError(jeq.CodeAuthMissing, "TYPESAFE_API_KEY is not set")
	}
	rootURL := f.baseURL
	if !f.baseURLSet {
		rootURL = deps.Getenv("TYPESAFE_BASE_URL")
		if rootURL == "" {
			rootURL = DefaultBaseURL
		}
	}
	client := deps.NewClient(rootURL, timeout, apiKey, f.maxRetries, func(line string) { _, _ = fmt.Fprintln(cmd.ErrOrStderr(), line) })
	response, callErr := client.Evaluate(cmd.Context(), request)
	if callErr != nil {
		return callErr
	}
	answer, ok := response.Answers[f.name]
	if !ok || answer.Type != contract.TypeChoice || answer.Choice == nil {
		return jeq.NewError(jeq.CodeResponseInvalid, "Choice response is missing the selected id")
	}
	selected := answer.Choice
	if _, ok := ids[*selected]; !ok {
		return jeq.NewError(jeq.CodeResponseInvalid, fmt.Sprintf("Choice selected unknown candidate id %q", *selected))
	}
	var selectedRecord []byte
	for _, record := range records {
		id, _, _ := pickCandidate(record, f.idPointer, f.criteriaPointer)
		if id == *selected {
			selectedRecord = record
			break
		}
	}
	output, attachErr := pipeline.AttachResponse(selectedRecord, f.name, response)
	if attachErr != nil {
		return attachErr
	}
	return renderRaw(deps.Renderer, cmd.OutOrStdout(), output)
}

func pickState(deps AskDeps, f pickFlags) ([]byte, *jeq.Error) {
	if f.stateSet {
		return []byte(strconvQuote(f.state)), nil
	}
	path := f.stateFile
	if f.stateJSONSet {
		path = f.stateJSON
	}
	if path == "-" {
		b, err := deps.ReadStdin(deps.Stdin, 64<<20, true)
		if err != nil {
			return nil, err
		}
		if f.stateFileSet {
			return []byte(strconvQuote(string(b))), nil
		}
		return b, nil
	}
	b, err := deps.ReadFile(path, 64<<20)
	if err != nil {
		return nil, err
	}
	if f.stateFileSet {
		return []byte(strconvQuote(string(b))), nil
	}
	return b, nil
}

func pickCandidate(record []byte, idPointer, criteriaPointer string) (string, json.RawMessage, *jeq.Error) {
	if err := pipeline.ValidateRecord(record); err != nil {
		return "", nil, jeq.NewError(jeq.CodeInputInvalid, err.Error())
	}
	idRaw, err := pipeline.Select(record, idPointer)
	if err != nil {
		return "", nil, jeq.NewError(jeq.CodeInputInvalid, err.Error())
	}
	var id string
	if json.Unmarshal(idRaw, &id) != nil || strings.TrimSpace(id) == "" {
		return "", nil, jeq.NewError(jeq.CodeInputInvalid, "candidate id must be a non-empty string")
	}
	criteria, err := pipeline.Select(record, criteriaPointer)
	if err != nil {
		return "", nil, jeq.NewError(jeq.CodeInputInvalid, err.Error())
	}
	return id, append(json.RawMessage(nil), criteria...), nil
}

func strconvQuote(value string) string { b, _ := json.Marshal(value); return string(b) }
