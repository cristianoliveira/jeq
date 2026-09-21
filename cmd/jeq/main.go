// Command jeq is the agent-first CLI for TypeSafe System One.
// main is the composition root: it wires infra adapters into cli and runs.
package main

import (
	"io"
	"net/http"
	"os"
	"time"

	"github.com/cristianoliveira/jeq/internal/cli"
	"github.com/cristianoliveira/jeq/internal/infra/render"
	"github.com/cristianoliveira/jeq/internal/infra/source"
	"github.com/cristianoliveira/jeq/internal/infra/typesafeapi"
)

// The composition root keeps cli and infra wired together; this compile
// assertion pins that infra/render satisfies the cli port.
var _ cli.Renderer = render.JSON{}

func main() {
	deps := cli.AskDeps{
		ReadFile: func(path string, limit int64) ([]byte, *cli.CodedError) {
			return source.ReadFile(path, limit, source.OSOpen, true)
		},
		ReadOptionalFile: func(path string, limit int64) ([]byte, *cli.CodedError, bool) {
			return source.ReadOptionalFile(path, limit)
		},
		ReadStdin: func(stdin io.Reader, limit int64, forbidEmpty bool) ([]byte, *cli.CodedError) {
			return source.ReadStdin(stdin, limit, func() bool {
				info, err := os.Stdin.Stat()
				return err == nil && info.Mode()&os.ModeCharDevice != 0
			}, forbidEmpty)
		},
		NewClient: func(baseURL string, timeout time.Duration, apiKey string, maxRetries int, diagnostic func(string)) cli.APIClient {
			httpc := &http.Client{Timeout: timeout, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
			c := typesafeapi.New(baseURL, httpc, apiKey)
			if apiKey == "" {
				c.AuthMode = "none"
			}
			c.MaxRetries = maxRetries
			c.Diagnostic = diagnostic
			return c
		},
		Getenv:   os.Getenv,
		Stdin:    os.Stdin,
		Renderer: render.JSON{},
	}
	os.Exit(cli.RunWithDeps(os.Args[1:], os.Stdout, os.Stderr, render.JSON{}, deps))
}
