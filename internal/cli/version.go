package cli

import "github.com/spf13/cobra"

// Version and Commit are overridable at build time via -ldflags.
var (
	Version = "dev"
	Commit  = "unknown"
)

// NewVersionCmd creates the standalone plain-text build-information command.
func NewVersionCmd() *cobra.Command {
	return NewVersionCmdWithDeps(AskDeps{})
}
