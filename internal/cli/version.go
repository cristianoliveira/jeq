package cli

import (
	"encoding/json"

	"github.com/spf13/cobra"
)

// Version and Commit are overridable at build time via -ldflags.
var (
	Version = "dev"
	Commit  = "unknown"
)

type buildInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Commit  string `json:"commit"`
}

// NewVersionCmd prints the build information contract: one JSON line.
func NewVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:          "version",
		Short:        "Print build information as a single JSON line",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return json.NewEncoder(cmd.OutOrStdout()).Encode(buildInfo{
				Name:    "gev",
				Version: Version,
				Commit:  Commit,
			})
		},
	}
}
