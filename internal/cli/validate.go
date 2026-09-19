package cli

import (
	"github.com/cristianoliveira/gev/internal/domain/gev"
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
		Use: "validate", Short: "Validate one request without network access",
		Example: `  gev validate --questions questions.json --state-json state.json
  gev examples ask-native`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			format, _ := cmd.Flags().GetString("output")
			flags := askFlags{
				request: request, questions: questions, state: state, stateFile: stateFile, stateJSON: stateJSON, model: model,
				output: format, requestSet: cmd.Flags().Changed("request"), questionsSet: cmd.Flags().Changed("questions"),
				stateSet: cmd.Flags().Changed("state"), stateFileSet: cmd.Flags().Changed("state-file"), stateJSONSet: cmd.Flags().Changed("state-json"),
			}
			return runValidate(cmd, deps, flags)
		},
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
	sources := gev.Sources{Request: f.requestSet, Questions: f.questionsSet, StateText: f.stateSet, StateFile: f.stateFileSet, StateJSON: f.stateJSONSet}
	if err := gev.CheckSources(sources); err != nil {
		return err
	}
	if err := gev.CheckStdin(f.requestSet && f.request == "-", f.questionsSet && f.questions == "-", (f.stateFileSet && f.stateFile == "-") || (f.stateJSONSet && f.stateJSON == "-")); err != nil {
		return err
	}
	read := func(path string) ([]byte, *gev.Error) {
		if path == "-" {
			if deps.Stdin == nil {
				return nil, gev.NewError(gev.CodeInputInvalid, "stdin is unavailable").WithRecovery("provide an explicit file or stdin stream")
			}
			return deps.ReadStdin(deps.Stdin, SourceLimit, true)
		}
		return deps.ReadFile(path, SourceLimit)
	}
	var requestDoc, questionsDoc []byte
	var stateInput gev.StateInput
	var err *gev.Error
	if f.requestSet {
		requestDoc, err = read(f.request)
	} else {
		questionsDoc, err = read(f.questions)
		if err == nil {
			switch {
			case f.stateSet:
				stateInput = gev.StateInput{Kind: gev.SourceStateText, Text: f.state}
			case f.stateFileSet:
				var data []byte
				data, err = read(f.stateFile)
				stateInput = gev.StateInput{Kind: gev.SourceStateText, Text: string(data)}
			case f.stateJSONSet:
				var data []byte
				data, err = read(f.stateJSON)
				stateInput = gev.StateInput{Kind: gev.SourceStateJSON, JSON: data}
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
	req, composeErr := gev.Compose(gev.ComposeInput{RequestDoc: requestDoc, QuestionsDoc: questionsDoc, State: stateInput, Model: resolvedModel})
	if composeErr != nil {
		return composeErr
	}
	renderer, renderErr := outputRenderer(cmd, deps)
	if renderErr != nil {
		return renderErr
	}
	mode := "composed"
	if f.requestSet {
		mode = "native"
	}
	return renderer.RenderValue(cmd.OutOrStdout(), validateDocument{Valid: true, Mode: mode, Model: req.Model, QuestionCount: len(req.Questions)})
}
