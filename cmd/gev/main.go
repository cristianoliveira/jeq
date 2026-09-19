// Command gev is the agent-first CLI for TypeSafe System One.
// main is the composition root: it wires infra adapters into cli and runs.
package main

import (
	"io"
	"net/http"
	"os"
	"time"

	"github.com/cristianoliveira/gev/internal/cli"
	"github.com/cristianoliveira/gev/internal/infra/render"
	"github.com/cristianoliveira/gev/internal/infra/source"
	"github.com/cristianoliveira/gev/internal/infra/typesafeapi"
)

// The composition root keeps cli and infra wired together; this compile
// assertion pins that infra/render satisfies the cli port.
var _ cli.Renderer = render.JSON{}

func main() {
	deps := cli.AskDeps{
		ReadFile: func(path string, limit int64) ([]byte, *cli.CodedError) {
			return source.ReadFile(path, limit, source.OSOpen, true)
		},
		ReadStdin: func(stdin io.Reader, limit int64, forbidEmpty bool) ([]byte, *cli.CodedError) {
			return source.ReadStdin(stdin, limit, func() bool {
				info, err := os.Stdin.Stat()
				return err == nil && info.Mode()&os.ModeCharDevice != 0
			}, forbidEmpty)
		},
		NewClient: func(baseURL string, timeout time.Duration, apiKey string, maxRetries int, diagnostic func(string)) cli.APIClient {
			c := typesafeapi.New(baseURL, &http.Client{Timeout: timeout}, apiKey)
			c.MaxRetries = maxRetries
			c.Diagnostic = diagnostic
			return c
		},
		Getenv:   os.Getenv,
		Stdin:    os.Stdin,
		Renderer: render.TOON{},
		RendererFor: func(format string) cli.Renderer {
			if format == "json" {
				return render.JSON{}
			}
			return render.TOON{}
		},
	}
	os.Exit(cli.RunWithDeps(os.Args[1:], os.Stdout, os.Stderr, render.TOON{}, deps))
}
