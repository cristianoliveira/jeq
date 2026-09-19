package cli

import (
	"fmt"

	"github.com/cristianoliveira/gev/internal/domain/gev"
	"github.com/spf13/cobra"
)

type renderedError struct{ err *gev.Error }

func (e *renderedError) Error() string { return e.err.Error() }
func (e *renderedError) Unwrap() error { return e.err }

func commandOutput(cmd *cobra.Command) (string, error) {
	if flag := cmd.Flags().Lookup("output"); flag != nil {
		return cmd.Flags().GetString("output")
	}
	if flag := cmd.InheritedFlags().Lookup("output"); flag != nil {
		return cmd.InheritedFlags().GetString("output")
	}
	return cmd.Root().PersistentFlags().GetString("output")
}

func unsupportedOutput(output string) *gev.Error {
	return gev.NewError(gev.CodeInputInvalid, fmt.Sprintf("unsupported output format %q", output)).WithRecovery("set --output json")
}
