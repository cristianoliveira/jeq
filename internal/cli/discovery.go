package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/cristianoliveira/gev/internal/domain/contract"
	"github.com/cristianoliveira/gev/internal/domain/gev"
	"github.com/spf13/cobra"
)

type versionDocument struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Commit  string `json:"commit"`
}

type homeDocument struct {
	Identity        string   `json:"identity"`
	Purpose         string   `json:"purpose"`
	CredentialReady bool     `json:"credential_ready"`
	DefaultModel    string   `json:"default_model"`
	Commands        []string `json:"commands"`
	NextStep        string   `json:"next_step"`
}

func depsForRoot(deps ...AskDeps) AskDeps {
	if len(deps) == 0 {
		return AskDeps{}
	}
	return deps[0]
}

func outputRenderer(cmd *cobra.Command, deps AskDeps) (ValueRenderer, error) {
	format, err := cmd.Flags().GetString("output")
	if err != nil {
		return nil, gev.WrapError(gev.CodeInputInvalid, err, "reading --output")
	}
	if format != "json" {
		return nil, gev.NewError(gev.CodeInputInvalid,
			fmt.Sprintf("unsupported output format %q", format)).WithRecovery("set --output json")
	}
	if renderer, ok := deps.Renderer.(ValueRenderer); ok {
		return renderer, nil
	}
	return nil, gev.NewError(gev.CodeResponseInvalid, "no value renderer configured").WithRecovery("run gev through its standard composition root")
}

func renderHome(cmd *cobra.Command, deps AskDeps) error {
	if deps.Getenv == nil {
		deps.Getenv = func(string) string { return "" }
	}
	renderer, err := outputRenderer(cmd, deps)
	if err != nil {
		return err
	}
	commands := []string{"version", "help"}
	if deps.valid() {
		commands = append([]string{"ask", "map"}, commands...)
	}
	if deps.ReadStdin != nil && deps.Renderer != nil {
		commands = append([]string{"gate"}, commands...)
	}
	if deps.modelsReady() {
		commands = append([]string{"models"}, commands...)
	}
	if deps.sourceReady() {
		commands = append([]string{"validate"}, commands...)
	}
	doc := homeDocument{
		Identity:        "gev",
		Purpose:         "agent-first TypeSafe System One client",
		CredentialReady: strings.TrimSpace(deps.Getenv("TYPESAFE_API_KEY")) != "",
		DefaultModel:    ResolveModel("", deps.Getenv),
		Commands:        commands,
		NextStep:        "run gev ask --help",
	}
	return renderer.RenderValue(cmd.OutOrStdout(), doc)
}

// NewVersionCmdWithDeps is the renderer-aware version command. NewVersionCmd
// remains the small JSON-only constructor used by its package-level contract
// test.
func NewVersionCmdWithDeps(deps AskDeps) *cobra.Command {
	return &cobra.Command{
		Use: "version", Short: "Print build information",
		RunE: func(cmd *cobra.Command, _ []string) error {
			renderer, err := outputRenderer(cmd, deps)
			if err != nil {
				return err
			}
			return renderer.RenderValue(cmd.OutOrStdout(), versionDocument{Name: "gev", Version: Version, Commit: Commit})
		},
	}
}

// NewModelsCmd creates the authenticated models discovery command.
func NewModelsCmd(deps AskDeps) *cobra.Command {
	var baseURL, timeoutText string
	cmd := &cobra.Command{
		Use: "models", Short: "List models available to the account",
		RunE: func(cmd *cobra.Command, _ []string) error {
			renderer, err := outputRenderer(cmd, deps)
			if err != nil {
				return err
			}
			timeout, parseErr := time.ParseDuration(timeoutText)
			if parseErr != nil || timeout <= 0 {
				return gev.NewError(gev.CodeInputInvalid, fmt.Sprintf("invalid --timeout %q", timeoutText)).WithRecovery("set --timeout to a positive Go duration, for example 10s")
			}
			apiKey := deps.Getenv("TYPESAFE_API_KEY")
			if strings.TrimSpace(apiKey) == "" {
				return gev.NewError(gev.CodeAuthMissing, "TYPESAFE_API_KEY is not set").WithRecovery("export TYPESAFE_API_KEY with the account key")
			}
			if baseURL == "" {
				baseURL = deps.Getenv("TYPESAFE_BASE_URL")
			}
			if baseURL == "" {
				baseURL = DefaultBaseURL
			}
			if parsed, parseErr := url.Parse(baseURL); parseErr != nil || parsed.Scheme == "" || parsed.Host == "" {
				return gev.NewError(gev.CodeInputInvalid, "--base-url must be an absolute URL").WithRecovery("set --base-url to an https:// or http:// API root")
			}
			client := deps.NewClient(baseURL, timeout, apiKey, DefaultMaxRetries, func(line string) { _, _ = fmt.Fprintln(cmd.ErrOrStderr(), line) })
			modelsClient, ok := client.(interface {
				Models(context.Context) (contract.Models, *gev.Error)
			})
			if !ok {
				return gev.NewError(gev.CodeResponseInvalid, "configured client cannot list models").WithRecovery("run gev through its standard composition root")
			}
			models, callErr := modelsClient.Models(cmd.Context())
			if callErr != nil {
				return callErr
			}
			raw, encodeErr := models.Encode()
			if encodeErr != nil {
				return gev.WrapError(gev.CodeResponseInvalid, encodeErr, "encoding models document").WithRecovery("retry the request; if it persists, report the response shape")
			}
			value, decodeErr := decodeDocument(raw)
			if decodeErr != nil {
				return gev.WrapError(gev.CodeResponseInvalid, decodeErr, "preparing models document").WithRecovery("retry the request; if it persists, report the response shape")
			}
			return renderer.RenderValue(cmd.OutOrStdout(), value)
		},
	}
	cmd.Flags().StringVar(&baseURL, "base-url", "", "TypeSafe API root")
	cmd.Flags().StringVar(&timeoutText, "timeout", DefaultTimeout.String(), "request timeout")
	return cmd
}

func decodeDocument(raw []byte) (any, error) {
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, err
	}
	return value, nil
}
