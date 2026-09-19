// Package cli is the shell ring: Cobra commands, flags, and exit mapping.
// It depends on domain ports only; infra is injected by main.
package cli

import "github.com/spf13/cobra"

// NewRootCmd builds a fresh gev command tree.
func NewRootCmd(deps ...AskDeps) *cobra.Command {
	root := &cobra.Command{
		Use:           "gev",
		Short:         "Agent-first CLI for TypeSafe System One",
		SilenceUsage:  true,
		SilenceErrors: true, // errors render as structured documents, not Cobra prose
		RunE: func(cmd *cobra.Command, _ []string) error {
			return renderHome(cmd, depsForRoot(deps...))
		},
	}
	root.PersistentFlags().String("output", "json", "output format (json)")
	// Flag parse failures are usage failures (exit 2), not generic errors.
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return NewUsageError(err)
	})
	root.AddCommand(NewVersionCmdWithDeps(depsForRoot(deps...)))
	if len(deps) > 0 {
		root.AddCommand(NewGateCmd(deps[0]))
	}
	if len(deps) > 0 && deps[0].valid() {
		root.AddCommand(NewAskCmd(deps[0]))
		root.AddCommand(NewMapCmd(deps[0]))
	}
	if len(deps) > 0 && deps[0].modelsReady() {
		root.AddCommand(NewModelsCmd(deps[0]))
	}
	if len(deps) > 0 && deps[0].sourceReady() {
		root.AddCommand(NewValidateCmd(deps[0]))
	}
	return root
}
