package cli

import (
	"fmt"

	"github.com/cristianoliveira/jeq/internal/domain/jeq"
	"github.com/spf13/cobra"
)

// validateDocument is intentionally smaller than a request: state never
// appears in a validation receipt.
type validateDocument struct {
	Valid         bool   `json:"valid"`
	Mode          string `json:"mode"`
	Model         string `json:"model"`
	QuestionCount int    `json:"question_count"`
}

// NewValidateCmd creates the offline validation command.
func NewValidateCmd(deps AskDeps) *cobra.Command {
	var request, questions, questionsJSON, state, stateFile, stateJSON, stateJSONFile, model string
	cmd := &cobra.Command{
		Use: "validate", Short: "Validate one request without network access", Long: "Validate request or composed inputs locally. `--state-json` is inline JSON; use `--state-json-file` for a file. Inline values may enter shell history, so use files or stdin when that matters.", Args: cobra.NoArgs,
		Example: `  jeq validate --questions-json '{"questions":{"q":{"type":"noul","instructions":"Is this urgent?"}}}' --state-json '{"ticket":"abc"}' --model jev-latest
  jeq validate --questions questions.json --state-json-file state.json
  jeq examples validate`,
		RunE: withBareHelp(func(cmd *cobra.Command, _ []string) error {
			flags := askFlags{
				request: request, questions: questions, questionsJSON: questionsJSON, state: state, stateFile: stateFile, stateJSON: stateJSON, stateJSONFile: stateJSONFile, model: model,
				requestSet: cmd.Flags().Changed("request"), questionsSet: cmd.Flags().Changed("questions"), questionsJSONSet: cmd.Flags().Changed("questions-json"),
				stateSet: cmd.Flags().Changed("state"), stateFileSet: cmd.Flags().Changed("state-file"), stateJSONSet: cmd.Flags().Changed("state-json"), stateJSONFileSet: cmd.Flags().Changed("state-json-file"),
			}
			return runValidate(cmd, deps, flags)
		}),
	}
	flags := cmd.Flags()
	flags.StringVar(&request, "request", "", "complete native request JSON file or -")
	flags.StringVar(&questions, "questions", "", "questions document file or -")
	flags.StringVar(&questionsJSON, "questions-json", "", "inline questions document JSON")
	flags.StringVar(&state, "state", "", "literal state text")
	flags.StringVar(&stateFile, "state-file", "", "state text file or -")
	flags.StringVar(&stateJSON, "state-json", "", "inline state JSON")
	flags.StringVar(&stateJSONFile, "state-json-file", "", "state JSON file or -")
	flags.StringVar(&model, "model", "", "composed-mode model")
	return cmd
}

func runValidate(cmd *cobra.Command, deps AskDeps, f askFlags) error {
	if f.questionsSet && f.questionsJSONSet {
		return jeq.NewError(jeq.CodeSourceConflict, "choose exactly one of --questions or --questions-json")
	}
	sources := jeq.Sources{Request: f.requestSet, Questions: f.questionsSet || f.questionsJSONSet, StateText: f.stateSet, StateFile: f.stateFileSet, StateJSON: f.stateJSONSet, StateJSONFile: f.stateJSONFileSet}
	if err := jeq.CheckSources(sources); err != nil {
		return err
	}
	if err := jeq.CheckStdin(f.requestSet && f.request == "-", f.questionsSet && f.questions == "-", (f.stateFileSet && f.stateFile == "-") || (f.stateJSONFileSet && f.stateJSONFile == "-")); err != nil {
		return err
	}
	if f.questionsJSONSet {
		if len([]byte(f.questionsJSON)) > SourceLimit {
			return jeq.NewError(jeq.CodeInputInvalid, fmt.Sprintf("--questions-json exceeds the %d byte limit", SourceLimit))
		}
		if err := validateInlineQuestionsDocument([]byte(f.questionsJSON)); err != nil {
			return err
		}
	}
	if f.stateJSONSet {
		if err := validateInlineStateJSON([]byte(f.stateJSON), "--state-json"); err != nil {
			return err
		}
	}
	read := func(path string) ([]byte, *jeq.Error) {
		if path == "-" {
			if deps.Stdin == nil {
				return nil, jeq.NewError(jeq.CodeInputInvalid, "stdin is unavailable").WithRecovery("provide an explicit file or stdin stream")
			}
			return deps.ReadStdin(deps.Stdin, SourceLimit, true)
		}
		return deps.ReadFile(path, SourceLimit)
	}
	var requestDoc, questionsDoc []byte
	var stateInput jeq.StateInput
	var err *jeq.Error
	if f.requestSet {
		requestDoc, err = read(f.request)
	} else {
		if f.questionsSet {
			questionsDoc, err = read(f.questions)
		} else {
			questionsDoc = []byte(f.questionsJSON)
			if len(questionsDoc) > SourceLimit {
				err = jeq.NewError(jeq.CodeInputInvalid, fmt.Sprintf("--questions-json exceeds the %d byte limit", SourceLimit))
			}
		}
		if err == nil && f.questionsSet {
			err = validateQuestionsDocument(questionsDoc)
		}
		if err == nil {
			switch {
			case f.stateSet:
				stateInput = jeq.StateInput{Kind: jeq.SourceStateText, Text: f.state}
			case f.stateFileSet:
				var data []byte
				data, err = read(f.stateFile)
				stateInput = jeq.StateInput{Kind: jeq.SourceStateText, Text: string(data)}
			case f.stateJSONSet:
				if err = validateInlineStateJSON([]byte(f.stateJSON), "--state-json"); err == nil {
					stateInput = jeq.StateInput{Kind: jeq.SourceStateJSON, JSON: []byte(f.stateJSON)}
				}
			case f.stateJSONFileSet:
				var data []byte
				data, err = read(f.stateJSONFile)
				if err == nil {
					err = validateInlineStateJSON(data, "--state-json-file")
				}
				stateInput = jeq.StateInput{Kind: jeq.SourceStateJSON, JSON: data}
			}
		}
	}
	if err != nil {
		return err
	}
	resolvedModel := ""
	if !f.requestSet {
		resolvedModel = ResolveModel(f.model, deps.Getenv)
	}
	req, composeErr := jeq.Compose(jeq.ComposeInput{RequestDoc: requestDoc, QuestionsDoc: questionsDoc, State: stateInput, Model: resolvedModel})
	if composeErr != nil {
		return composeErr
	}
	mode := "composed"
	if f.requestSet {
		mode = "native"
	}
	traceMetadata(cmd, req.Model, mode, "json", "", req.Questions)
	doc := validateDocument{Valid: true, Mode: mode, Model: req.Model, QuestionCount: len(req.Questions)}
	_, writeErr := fmt.Fprintf(cmd.OutOrStdout(), "valid: %t\nmode: %s\nmodel: %s\nquestion_count: %d\n", doc.Valid, doc.Mode, doc.Model, doc.QuestionCount)
	if writeErr != nil {
		return withTracePhase(jeq.WrapError(jeq.CodeResponseInvalid, writeErr, "writing validation output"), "output_write")
	}
	return nil
}
