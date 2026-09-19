// Package cli is the shell ring: Cobra commands, flags, and exit mapping.
// It depends on domain ports only; infra is injected by main.
package cli

import "github.com/spf13/cobra"

// NewRootCmd builds a fresh gev command tree.
func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "gev",
		Short:         "Agent-first CLI for TypeSafe System One",
		SilenceUsage:  true,
		SilenceErrors: true, // errors render as structured documents, not Cobra prose
	}
	// Flag parse failures are usage failures (exit 2), not generic errors.
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return NewUsageError(err)
	})
	root.AddCommand(NewVersionCmd())
	return root
}
