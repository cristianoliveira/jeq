package cli

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/cristianoliveira/jeq/internal/domain/contract"
	"github.com/cristianoliveira/jeq/internal/domain/jeq"
	"github.com/spf13/cobra"
)

const (
	// DefaultBaseURL is the production TypeSafe API root.
	DefaultBaseURL = "https://api.typesafe.ai"
	// DefaultTimeout is the request timeout when --timeout is omitted.
	DefaultTimeout = 10 * time.Second
	// DefaultMaxRetries is the retry count when --max-retries is omitted.
	DefaultMaxRetries = 2
	// MaxRetriesLimit is the CLI safety bound for retry configuration.
	MaxRetriesLimit = 5
	// SourceLimit bounds every selected request/state read.
	SourceLimit = 8 << 20
)

// APIClient is the narrow outbound port used by ask.
type APIClient interface {
	Evaluate(context.Context, contract.Request) (contract.Response, *jeq.Error)
}

// AskDeps contains every side effect ask needs. The composition root supplies
// production adapters; tests supply bounded readers and an httptest-backed
// client. This keeps source I/O and HTTP out of the shell policy.
type AskDeps struct {
	ReadFile         func(path string, limit int64) ([]byte, *jeq.Error)
	ReadOptionalFile func(path string, limit int64) ([]byte, *jeq.Error, bool)
	ReadStdin        func(stdin io.Reader, limit int64, forbidEmpty bool) ([]byte, *jeq.Error)
	NewClient        func(baseURL string, timeout time.Duration, apiKey string, maxRetries int, diagnostic func(string)) APIClient
	NewProfileClient func(baseURL string, timeout time.Duration, apiKey, authMode string, maxRetries int, diagnostic func(string)) APIClient
	Getenv           func(string) string
	Stdin            io.Reader
	Renderer         Renderer
}

func (d AskDeps) sourceReady() bool {
	return d.ReadFile != nil && d.ReadStdin != nil && d.Getenv != nil
}

func (d AskDeps) modelsReady() bool {
	return d.Getenv != nil && d.NewClient != nil
}

func (d AskDeps) valid() bool {
	return d.sourceReady() && d.NewClient != nil
}

// NewAskCmd creates the ask command with native and composed source flags.
func NewAskCmd(deps AskDeps) *cobra.Command {
	var (
		request, questions, questionsJSON, state, stateFile, stateJSON, stateJSONFile string
		model, timeoutText                                                            string
		maxRetries                                                                    int
	)

	cmd := &cobra.Command{
		Use:   "ask",
		Short: "Send one System One request",
		Long: `Jev returns caller-defined typed semantic decisions and probabilities; it does not write replies, produce code, or return reasoning explanations. Use --state-json for inline JSON and --state-json-file for a JSON file. Inline values may be saved in shell history; use files or stdin when that matters.

Composed model precedence is --model, JEQ_DEFAULT_MODEL, selected provider default_model, config.default_model, legacy TYPESAFE_DEFAULT_MODEL, then jev-latest. Native --request documents own their model; --model is rejected with --request, so edit the document's model field instead.`,
		Args: cobra.NoArgs,
		Example: `  jeq ask --request request.json
  jeq ask --questions-json '{"questions":{"q":{"type":"noul","instructions":"Is this urgent?"}}}' --state-json '{"ticket":"abc"}' --model jev-latest
  jeq examples ask`,
		RunE: withBareHelp(func(cmd *cobra.Command, _ []string) error {
			return runAsk(cmd, deps, askFlags{
				request: request, questions: questions, questionsJSON: questionsJSON, state: state, stateFile: stateFile, stateJSON: stateJSON, stateJSONFile: stateJSONFile,
				model: model, timeout: timeoutText, maxRetries: maxRetries, modelSet: cmd.Flags().Changed("model"),
				requestSet: cmd.Flags().Changed("request"), questionsSet: cmd.Flags().Changed("questions"), questionsJSONSet: cmd.Flags().Changed("questions-json"),
				stateSet: cmd.Flags().Changed("state"), stateFileSet: cmd.Flags().Changed("state-file"), stateJSONSet: cmd.Flags().Changed("state-json"), stateJSONFileSet: cmd.Flags().Changed("state-json-file"),
			})
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
	flags.StringVar(&timeoutText, "timeout", DefaultTimeout.String(), "request timeout")
	flags.IntVar(&maxRetries, "max-retries", DefaultMaxRetries, "maximum retries (0-5)")
	return cmd
}

type askFlags struct {
	request, questions, questionsJSON, state, stateFile, stateJSON, stateJSONFile                      string
	model, timeout                                                                                     string
	modelSet                                                                                           bool
	maxRetries                                                                                         int
	requestSet, questionsSet, questionsJSONSet, stateSet, stateFileSet, stateJSONSet, stateJSONFileSet bool
}

func runAsk(cmd *cobra.Command, deps AskDeps, f askFlags) error {
	if f.requestSet && f.modelSet {
		return askError(jeq.NewError(jeq.CodeSourceConflict, "--model cannot be used with --request; remove --model or edit the model in the request document"))
	}
	if f.questionsSet && f.questionsJSONSet {
		return askError(jeq.NewError(jeq.CodeSourceConflict, "choose exactly one of --questions or --questions-json"))
	}
	sources := jeq.Sources{
		Request: f.requestSet, Questions: f.questionsSet || f.questionsJSONSet, StateText: f.stateSet,
		StateFile: f.stateFileSet, StateJSON: f.stateJSONSet, StateJSONFile: f.stateJSONFileSet,
	}
	// These checks are deliberately before readers and before every env lookup.
	if err := jeq.CheckSources(sources); err != nil {
		return askError(err)
	}
	if err := jeq.CheckStdin(
		f.requestSet && f.request == "-", f.questionsSet && f.questions == "-",
		(f.stateFileSet && f.stateFile == "-") || (f.stateJSONFileSet && f.stateJSONFile == "-")); err != nil {
		return askError(err)
	}
	if f.questionsJSONSet {
		if len([]byte(f.questionsJSON)) > SourceLimit {
			return askError(jeq.NewError(jeq.CodeInputInvalid, fmt.Sprintf("--questions-json exceeds the %d byte limit", SourceLimit)))
		}
		if err := validateInlineQuestionsDocument([]byte(f.questionsJSON)); err != nil {
			return askError(err)
		}
	}
	if f.stateJSONSet {
		if err := validateInlineStateJSON([]byte(f.stateJSON), "--state-json"); err != nil {
			return askError(err)
		}
	}

	read := func(path string, forbidEmpty bool) ([]byte, *jeq.Error) {
		if path == "-" {
			if deps.Stdin == nil {
				return nil, jeq.NewError(jeq.CodeInputInvalid, "stdin is unavailable; pipe the selected document").WithRecovery("provide an explicit file or stdin stream")
			}
			return deps.ReadStdin(deps.Stdin, SourceLimit, forbidEmpty)
		}
		return deps.ReadFile(path, SourceLimit)
	}

	var requestDoc, questionsDoc []byte
	var stateInput jeq.StateInput
	var err *jeq.Error
	if f.requestSet {
		requestDoc, err = read(f.request, true)
	} else {
		if f.questionsSet {
			questionsDoc, err = read(f.questions, true)
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
				data, err = read(f.stateFile, true)
				stateInput = jeq.StateInput{Kind: jeq.SourceStateText, Text: string(data)}
			case f.stateJSONSet:
				if err = validateInlineStateJSON([]byte(f.stateJSON), "--state-json"); err == nil {
					stateInput = jeq.StateInput{Kind: jeq.SourceStateJSON, JSON: []byte(f.stateJSON)}
				}
			case f.stateJSONFileSet:
				var data []byte
				data, err = read(f.stateJSONFile, true)
				if err == nil {
					err = validateInlineStateJSON(data, "--state-json-file")
				}
				stateInput = jeq.StateInput{Kind: jeq.SourceStateJSON, JSON: data}
			}
		}
	}
	if err != nil {
		return askError(err)
	}

	resolvedModel := ""
	modelSource := ""
	if !f.requestSet {
		var modelErr *jeq.Error
		resolvedModel, modelSource, modelErr = ResolveConfiguredModelWithSource(f.model, strings.TrimSpace(deps.Getenv("JEQ_CONFIG")), deps.Getenv, deps.ReadFile, deps.ReadOptionalFile)
		if modelErr != nil {
			return askError(modelErr)
		}
	}
	if f.requestSet {
		modelSource = "native"
	}
	req, composeErr := jeq.Compose(jeq.ComposeInput{
		RequestDoc: requestDoc, QuestionsDoc: questionsDoc, State: stateInput, Model: resolvedModel,
	})
	if composeErr != nil {
		return askError(composeErr)
	}

	// Validate local configuration before credential lookup and client creation.
	// This keeps malformed flags in the usage class and never starts I/O.
	if f.maxRetries < 0 || f.maxRetries > MaxRetriesLimit {
		return jeq.NewError(jeq.CodeInputInvalid,
			fmt.Sprintf("--max-retries must be between 0 and %d, got %d", MaxRetriesLimit, f.maxRetries)).WithRecovery("set --max-retries to an integer from 0 through 5")
	}
	timeout, parseErr := time.ParseDuration(f.timeout)
	if parseErr != nil || timeout <= 0 {
		return jeq.NewError(jeq.CodeInputInvalid, fmt.Sprintf("invalid --timeout %q", f.timeout)).WithRecovery("set --timeout to a positive Go duration, for example 10s")
	}

	provider, providerErr := ResolveProvider(deps.Getenv, deps.ReadFile, deps.ReadOptionalFile)
	if providerErr != nil {
		return providerErr
	}
	traceMetadata(cmd, req.Model, modelSource, "json", "", req.Questions)
	client := newClient(deps, provider, timeout, f.maxRetries, func(line string) {
		_, _ = fmt.Fprintln(cmd.ErrOrStderr(), line)
	})
	attachTrace(cmd, client)
	tracker := newUsageEvaluator(cmd.Context(), client)
	if tracker.summary != nil {
		tracker.summary.addProcessedRecords(1)
	}
	resp, evalErr := tracker.Evaluate(cmd.Context(), req)
	if evalErr != nil {
		return askError(evalErr)
	}
	tracker.recordAttachedAnswers()
	if deps.Renderer != nil {
		if err := deps.Renderer.RenderSuccess(cmd.OutOrStdout(), resp); err != nil {
			return withTracePhase(jeq.WrapError(jeq.CodeResponseInvalid, err, "rendering success document").WithRecovery("retry the request; if it persists, report the renderer failure"), "output_write")
		}
		return nil
	}
	return jeq.NewError(jeq.CodeResponseInvalid, "no output renderer configured").WithRecovery("run jeq through its standard composition root")
}

func askError(err *jeq.Error) *jeq.Error {
	if err == nil || err.Recovery != "" {
		return err
	}
	return err.WithRecovery("inspect the error message and correct the input or retry")
}
