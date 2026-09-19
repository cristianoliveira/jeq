// Command gev is the agent-first CLI for TypeSafe System One.
// main is the composition root: it wires infra adapters into cli and runs.
package main

import (
	"os"

	"github.com/cristianoliveira/gev/internal/cli"
)

func main() {
	if err := cli.NewRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}
