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
	var request, questions, state, stateFile, stateJSON, model string
	cmd := &cobra.Command{
		Use: "validate", Short: "Validate one request without network access", Args: cobra.NoArgs,
		Example: `  jeq validate --questions questions.json --state-json state.json
  jeq examples ask-native`,
		RunE: withBareHelp(func(cmd *cobra.Command, _ []string) error {
			flags := askFlags{
				request: request, questions: questions, state: state, stateFile: stateFile, stateJSON: stateJSON, model: model,
				requestSet: cmd.Flags().Changed("request"), questionsSet: cmd.Flags().Changed("questions"),
				stateSet: cmd.Flags().Changed("state"), stateFileSet: cmd.Flags().Changed("state-file"), stateJSONSet: cmd.Flags().Changed("state-json"),
			}
			return runValidate(cmd, deps, flags)
		}),
	}
	flags := cmd.Flags()
	flags.StringVar(&request, "request", "", "complete native request JSON file or -")
	flags.StringVar(&questions, "questions", "", "questions JSON file or -")
	flags.StringVar(&state, "state", "", "literal state text")
	flags.StringVar(&stateFile, "state-file", "", "state text file or -")
	flags.StringVar(&stateJSON, "state-json", "", "state JSON file or -")
	flags.StringVar(&model, "model", "", "composed-mode model")
	return cmd
}

func runValidate(cmd *cobra.Command, deps AskDeps, f askFlags) error {
	sources := jeq.Sources{Request: f.requestSet, Questions: f.questionsSet, StateText: f.stateSet, StateFile: f.stateFileSet, StateJSON: f.stateJSONSet}
	if err := jeq.CheckSources(sources); err != nil {
		return err
	}
	if err := jeq.CheckStdin(f.requestSet && f.request == "-", f.questionsSet && f.questions == "-", (f.stateFileSet && f.stateFile == "-") || (f.stateJSONSet && f.stateJSON == "-")); err != nil {
		return err
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
		questionsDoc, err = read(f.questions)
		if err == nil {
			switch {
			case f.stateSet:
				stateInput = jeq.StateInput{Kind: jeq.SourceStateText, Text: f.state}
			case f.stateFileSet:
				var data []byte
				data, err = read(f.stateFile)
				stateInput = jeq.StateInput{Kind: jeq.SourceStateText, Text: string(data)}
			case f.stateJSONSet:
				var data []byte
				data, err = read(f.stateJSON)
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
