// Package cli is the shell ring: Cobra commands, flags, and exit mapping.
// It depends on domain ports only; infra is injected by main.
package cli

import "github.com/spf13/cobra"

// withBareHelp wraps an action so a bare invocation opens native Cobra help.
func withBareHelp(run func(*cobra.Command, []string) error) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 && cmd.Flags().NFlag() == 0 {
			return cmd.Help()
		}
		return run(cmd, args)
	}
}

// NewRootCmd builds a fresh jeq command tree.
func NewRootCmd(deps ...AskDeps) *cobra.Command {
	root := &cobra.Command{
		Use:           "jeq",
		Short:         "Agent-first CLI for TypeSafe System One",
		Example:       "  jeq examples\n  jeq examples map-reduce-gate",
		SilenceUsage:  true,
		SilenceErrors: true, // Run prints Cobra's standard error line once, after policy-exit mapping
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}
	// Flag parse failures are usage failures (exit 2), not generic errors.
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return NewUsageError(err)
	})
	root.AddCommand(NewVersionCmdWithDeps(depsForRoot(deps...)))
	root.AddCommand(NewExamplesCmd(depsForRoot(deps...)))
	if len(deps) > 0 && deps[0].valid() {
		root.AddCommand(NewRankCmd(deps[0]))
		root.AddCommand(NewRateCmd(deps[0]))
	}
	if len(deps) > 0 {
		root.AddCommand(NewGateCmd(deps[0]))
	}
	if len(deps) > 0 && deps[0].valid() {
		root.AddCommand(NewAskCmd(deps[0]))
		root.AddCommand(NewMapCmd(deps[0]))
		root.AddCommand(NewReduceCmd(deps[0]))
	}
	if len(deps) > 0 && deps[0].modelsReady() {
		root.AddCommand(NewModelsCmd(deps[0]))
	}
	if len(deps) > 0 && deps[0].sourceReady() {
		root.AddCommand(NewValidateCmd(deps[0]))
	}
	return root
}
