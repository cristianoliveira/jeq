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
		request, questions, state, stateFile, stateJSON string
		model, timeoutText                              string
		maxRetries                                      int
	)

	cmd := &cobra.Command{
		Use:   "ask",
		Short: "Send one System One request",
		Args:  cobra.NoArgs,
		Example: `  jeq ask --request request.json
  jeq examples ask-native`,
		RunE: withBareHelp(func(cmd *cobra.Command, _ []string) error {
			return runAsk(cmd, deps, askFlags{
				request: request, questions: questions, state: state, stateFile: stateFile, stateJSON: stateJSON,
				model: model, timeout: timeoutText, maxRetries: maxRetries,
				requestSet: cmd.Flags().Changed("request"), questionsSet: cmd.Flags().Changed("questions"),
				stateSet: cmd.Flags().Changed("state"), stateFileSet: cmd.Flags().Changed("state-file"), stateJSONSet: cmd.Flags().Changed("state-json"),
			})
		}),
	}
	flags := cmd.Flags()
	flags.StringVar(&request, "request", "", "complete native request JSON file or -")
	flags.StringVar(&questions, "questions", "", "questions JSON file or -")
	flags.StringVar(&state, "state", "", "literal state text")
	flags.StringVar(&stateFile, "state-file", "", "state text file or -")
	flags.StringVar(&stateJSON, "state-json", "", "state JSON file or -")
	flags.StringVar(&model, "model", "", "composed-mode model")
	flags.StringVar(&timeoutText, "timeout", DefaultTimeout.String(), "request timeout")
	flags.IntVar(&maxRetries, "max-retries", DefaultMaxRetries, "maximum retries (0-5)")
	return cmd
}

type askFlags struct {
	request, questions, state, stateFile, stateJSON                string
	model, timeout                                                 string
	maxRetries                                                     int
	requestSet, questionsSet, stateSet, stateFileSet, stateJSONSet bool
}

func runAsk(cmd *cobra.Command, deps AskDeps, f askFlags) error {
	sources := jeq.Sources{
		Request: f.requestSet, Questions: f.questionsSet, StateText: f.stateSet,
		StateFile: f.stateFileSet, StateJSON: f.stateJSONSet,
	}
	// These checks are deliberately before readers and before every env lookup.
	if err := jeq.CheckSources(sources); err != nil {
		return askError(err)
	}
	if err := jeq.CheckStdin(
		f.requestSet && f.request == "-", f.questionsSet && f.questions == "-",
		(f.stateFileSet && f.stateFile == "-") || (f.stateJSONSet && f.stateJSON == "-")); err != nil {
		return askError(err)
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
		questionsDoc, err = read(f.questions, true)
		if err == nil {
			switch {
			case f.stateSet:
				stateInput = jeq.StateInput{Kind: jeq.SourceStateText, Text: f.state}
			case f.stateFileSet:
				var data []byte
				data, err = read(f.stateFile, true)
				stateInput = jeq.StateInput{Kind: jeq.SourceStateText, Text: string(data)}
			case f.stateJSONSet:
				var data []byte
				data, err = read(f.stateJSON, true)
				stateInput = jeq.StateInput{Kind: jeq.SourceStateJSON, JSON: data}
			}
		}
	}
	if err != nil {
		return askError(err)
	}

	resolvedModel := ""
	if !f.requestSet {
		var modelErr *jeq.Error
		resolvedModel, _, modelErr = ResolveConfiguredModelWithSource(f.model, strings.TrimSpace(deps.Getenv("JEQ_CONFIG")), deps.Getenv, deps.ReadFile, deps.ReadOptionalFile)
		if modelErr != nil {
			return askError(modelErr)
		}
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

	rootURL, rootErr := ResolveBaseURL(deps.Getenv)
	if rootErr != nil {
		return rootErr
	}

	apiKey := deps.Getenv("TYPESAFE_API_KEY")
	if strings.TrimSpace(apiKey) == "" {
		return jeq.NewError(jeq.CodeAuthMissing, "TYPESAFE_API_KEY is not set").WithRecovery("export TYPESAFE_API_KEY with the account key")
	}
	traceMetadata(cmd, req.Model, "", "json", "", req.Questions)
	client := deps.NewClient(rootURL, timeout, apiKey, f.maxRetries, func(line string) {
		_, _ = fmt.Fprintln(cmd.ErrOrStderr(), line)
	})
	attachTrace(cmd, client)
	resp, evalErr := client.Evaluate(cmd.Context(), req)
	if evalErr != nil {
		return askError(evalErr)
	}
	if deps.Renderer != nil {
		if err := deps.Renderer.RenderSuccess(cmd.OutOrStdout(), resp); err != nil {
			return askError(jeq.WrapError(jeq.CodeResponseInvalid, err, "rendering success document").WithRecovery("retry the request; if it persists, report the renderer failure"))
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
