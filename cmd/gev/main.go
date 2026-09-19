// Command gev is the agent-first CLI for TypeSafe System One.
// main is the composition root: it wires infra adapters into cli and runs.
package main

import (
	"os"

	"github.com/cristianoliveira/gev/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr))
}
