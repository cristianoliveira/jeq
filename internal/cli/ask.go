package cli

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/cristianoliveira/gev/internal/domain/contract"
	"github.com/cristianoliveira/gev/internal/domain/gev"
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
	Evaluate(context.Context, contract.Request) (contract.Response, *gev.Error)
}

// AskDeps contains every side effect ask needs. The composition root supplies
// production adapters; tests supply bounded readers and an httptest-backed
// client. This keeps source I/O and HTTP out of the shell policy.
type AskDeps struct {
	ReadFile         func(path string, limit int64) ([]byte, *gev.Error)
	ReadOptionalFile func(path string, limit int64) ([]byte, *gev.Error, bool)
	ReadStdin        func(stdin io.Reader, limit int64, forbidEmpty bool) ([]byte, *gev.Error)
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
		model, baseURL, timeoutText                     string
		maxRetries                                      int
	)

	cmd := &cobra.Command{
		Use:   "ask",
		Short: "Send one System One request",
		Example: `  gev ask --request request.json
  gev examples ask-native`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runAsk(cmd, deps, askFlags{
				request: request, questions: questions, state: state, stateFile: stateFile, stateJSON: stateJSON,
				model: model, baseURL: baseURL, timeout: timeoutText, maxRetries: maxRetries,
				requestSet: cmd.Flags().Changed("request"), questionsSet: cmd.Flags().Changed("questions"),
				stateSet: cmd.Flags().Changed("state"), stateFileSet: cmd.Flags().Changed("state-file"), stateJSONSet: cmd.Flags().Changed("state-json"),
				baseURLSet: cmd.Flags().Changed("base-url"),
			})
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&request, "request", "", "complete native request JSON file or -")
	flags.StringVar(&questions, "questions", "", "questions JSON file or -")
	flags.StringVar(&state, "state", "", "literal state text")
	flags.StringVar(&stateFile, "state-file", "", "state text file or -")
	flags.StringVar(&stateJSON, "state-json", "", "state JSON file or -")
	flags.StringVar(&model, "model", "", "composed-mode model")
	flags.StringVar(&baseURL, "base-url", "", "TypeSafe API root")
	flags.StringVar(&timeoutText, "timeout", DefaultTimeout.String(), "request timeout")
	flags.IntVar(&maxRetries, "max-retries", DefaultMaxRetries, "maximum retries (0-5)")
	return cmd
}

type askFlags struct {
	request, questions, state, stateFile, stateJSON                            string
	model, baseURL, timeout                                                    string
	maxRetries                                                                 int
	requestSet, questionsSet, stateSet, stateFileSet, stateJSONSet, baseURLSet bool
}

func runAsk(cmd *cobra.Command, deps AskDeps, f askFlags) error {
	sources := gev.Sources{
		Request: f.requestSet, Questions: f.questionsSet, StateText: f.stateSet,
		StateFile: f.stateFileSet, StateJSON: f.stateJSONSet,
	}
	// These checks are deliberately before readers and before every env lookup.
	if err := gev.CheckSources(sources); err != nil {
		return askError(err)
	}
	if err := gev.CheckStdin(
		f.requestSet && f.request == "-", f.questionsSet && f.questions == "-",
		(f.stateFileSet && f.stateFile == "-") || (f.stateJSONSet && f.stateJSON == "-")); err != nil {
		return askError(err)
	}

	read := func(path string, forbidEmpty bool) ([]byte, *gev.Error) {
		if path == "-" {
			if deps.Stdin == nil {
				return nil, gev.NewError(gev.CodeInputInvalid, "stdin is unavailable; pipe the selected document").WithRecovery("provide an explicit file or stdin stream")
			}
			return deps.ReadStdin(deps.Stdin, SourceLimit, forbidEmpty)
		}
		return deps.ReadFile(path, SourceLimit)
	}

	var requestDoc, questionsDoc []byte
	var stateInput gev.StateInput
	var err *gev.Error
	if f.requestSet {
		requestDoc, err = read(f.request, true)
	} else {
		questionsDoc, err = read(f.questions, true)
		if err == nil {
			switch {
			case f.stateSet:
				stateInput = gev.StateInput{Kind: gev.SourceStateText, Text: f.state}
			case f.stateFileSet:
				var data []byte
				data, err = read(f.stateFile, true)
				stateInput = gev.StateInput{Kind: gev.SourceStateText, Text: string(data)}
			case f.stateJSONSet:
				var data []byte
				data, err = read(f.stateJSON, true)
				stateInput = gev.StateInput{Kind: gev.SourceStateJSON, JSON: data}
			}
		}
	}
	if err != nil {
		return askError(err)
	}

	resolvedModel := ""
	if !f.requestSet {
		resolvedModel = ResolveModel(f.model, deps.Getenv)
	}
	req, composeErr := gev.Compose(gev.ComposeInput{
		RequestDoc: requestDoc, QuestionsDoc: questionsDoc, State: stateInput, Model: resolvedModel,
	})
	if composeErr != nil {
		return askError(composeErr)
	}

	// Validate local configuration before credential lookup and client creation.
	// This keeps malformed flags in the usage class and never starts I/O.
	if f.maxRetries < 0 || f.maxRetries > MaxRetriesLimit {
		return gev.NewError(gev.CodeInputInvalid,
			fmt.Sprintf("--max-retries must be between 0 and %d, got %d", MaxRetriesLimit, f.maxRetries)).WithRecovery("set --max-retries to an integer from 0 through 5")
	}
	timeout, parseErr := time.ParseDuration(f.timeout)
	if parseErr != nil || timeout <= 0 {
		return gev.NewError(gev.CodeInputInvalid, fmt.Sprintf("invalid --timeout %q", f.timeout)).WithRecovery("set --timeout to a positive Go duration, for example 10s")
	}
	if f.baseURLSet && strings.TrimSpace(f.baseURL) == "" {
		return gev.NewError(gev.CodeInputInvalid, "--base-url cannot be empty").WithRecovery("set --base-url to an API root URL")
	}

	apiKey := deps.Getenv("TYPESAFE_API_KEY")
	if strings.TrimSpace(apiKey) == "" {
		return gev.NewError(gev.CodeAuthMissing, "TYPESAFE_API_KEY is not set").WithRecovery("export TYPESAFE_API_KEY with the account key")
	}
	rootURL := f.baseURL
	if !f.baseURLSet {
		rootURL = deps.Getenv("TYPESAFE_BASE_URL")
		if rootURL == "" {
			rootURL = DefaultBaseURL
		}
	}
	client := deps.NewClient(rootURL, timeout, apiKey, f.maxRetries, func(line string) {
		_, _ = fmt.Fprintln(cmd.ErrOrStderr(), line)
	})
	resp, evalErr := client.Evaluate(cmd.Context(), req)
	if evalErr != nil {
		return askError(evalErr)
	}
	if deps.Renderer != nil {
		if err := deps.Renderer.RenderSuccess(cmd.OutOrStdout(), resp); err != nil {
			return askError(gev.WrapError(gev.CodeResponseInvalid, err, "rendering success document").WithRecovery("retry the request; if it persists, report the renderer failure"))
		}
		return nil
	}
	return gev.NewError(gev.CodeResponseInvalid, "no output renderer configured").WithRecovery("run gev through its standard composition root")
}

func askError(err *gev.Error) *gev.Error {
	if err == nil || err.Recovery != "" {
		return err
	}
	return err.WithRecovery("inspect the error message and correct the input or retry")
}
