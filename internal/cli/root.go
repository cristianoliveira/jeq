// Package cli is the shell ring: Cobra commands, flags, and exit mapping.
// It depends on domain ports only; infra is injected by main.
package cli

import "github.com/spf13/cobra"

// NewRootCmd builds a fresh gev command tree.
func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:          "gev",
		Short:        "Agent-first CLI for TypeSafe System One",
		SilenceUsage: true,
	}
	root.AddCommand(NewVersionCmd())
	return root
}
