package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/cristianoliveira/jeq/internal/domain/contract"
	"github.com/cristianoliveira/jeq/internal/domain/jeq"
	"github.com/spf13/cobra"
)

type versionDocument struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Commit  string `json:"commit"`
}

func depsForRoot(deps ...AskDeps) AskDeps {
	if len(deps) == 0 {
		return AskDeps{}
	}
	return deps[0]
}

// NewVersionCmdWithDeps creates the plain-text build-information command.
func NewVersionCmdWithDeps(_ AskDeps) *cobra.Command {
	return &cobra.Command{
		Use: "version", Short: "Print build information", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			doc := versionDocument{Name: "jeq", Version: Version, Commit: Commit}
			return writeVersion(cmd.OutOrStdout(), doc)
		},
	}
}

// NewModelsCmd creates the authenticated models discovery command.
func NewModelsCmd(deps AskDeps) *cobra.Command {
	var timeoutText string
	cmd := &cobra.Command{
		Use: "models", Short: "List models available to the account", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			timeout, parseErr := time.ParseDuration(timeoutText)
			if parseErr != nil || timeout <= 0 {
				return jeq.NewError(jeq.CodeInputInvalid, fmt.Sprintf("invalid --timeout %q", timeoutText)).WithRecovery("set --timeout to a positive Go duration, for example 10s")
			}
			provider, providerErr := ResolveProvider(deps.Getenv, deps.ReadFile, deps.ReadOptionalFile)
			if providerErr != nil {
				return providerErr
			}
			baseURL, apiKey := provider.BaseURL, provider.APIKey
			client := deps.NewClient(baseURL, timeout, apiKey, DefaultMaxRetries, func(line string) { _, _ = fmt.Fprintln(cmd.ErrOrStderr(), line) })
			attachTrace(cmd, client)
			modelsClient, ok := client.(interface {
				Models(context.Context) (contract.Models, *jeq.Error)
			})
			if !ok {
				return jeq.NewError(jeq.CodeResponseInvalid, "configured client cannot list models").WithRecovery("run jeq through its standard composition root")
			}
			models, callErr := modelsClient.Models(cmd.Context())
			if callErr != nil {
				return callErr
			}
			return writeModels(cmd.OutOrStdout(), models)
		},
	}
	cmd.Flags().StringVar(&timeoutText, "timeout", DefaultTimeout.String(), "request timeout")
	return cmd
}
