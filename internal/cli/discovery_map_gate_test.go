package cli

import (
	"bytes"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/cristianoliveira/jeq/internal/domain/jeq"
)

func TestMapAndGateRejectUnknownOutputFlagBeforeDependencies(t *testing.T) {
	for _, command := range []string{"map", "gate"} {
		t.Run(command, func(t *testing.T) {
			reads := 0
			deps := AskDeps{
				ReadStdin: func(_ io.Reader, _ int64, _ bool) ([]byte, *jeq.Error) { reads++; return nil, nil },
				ReadFile:  func(string, int64) ([]byte, *jeq.Error) { reads++; return nil, nil },
				Getenv:    func(string) string { reads++; return "" },
				NewClient: func(string, time.Duration, string, int, func(string)) APIClient { reads++; return nil },
				Renderer:  streamRenderer{},
			}
			args := []string{command, "--as", "x", "--output", "text"}
			if command == "map" {
				args = append(args, "--questions", "q.json")
			} else {
				args = append(args, "--value-pointer", "/value", "--pass-min", "0.8", "--reject-max", "0.2")
			}
			var out, stderr bytes.Buffer
			code := RunWithDeps(args, &out, &stderr, streamRenderer{}, deps)
			assert.Equal(t, 2, code)
			assert.Zero(t, reads)
		})
	}
}

func TestRootHelpIncludesAvailableMapAndGate(t *testing.T) {
	deps := AskDeps{
		ReadFile:  func(string, int64) ([]byte, *jeq.Error) { return nil, nil },
		ReadStdin: func(_ io.Reader, _ int64, _ bool) ([]byte, *jeq.Error) { return nil, nil },
		Getenv:    func(string) string { return "" },
		NewClient: func(string, time.Duration, string, int, func(string)) APIClient { return nil },
		Renderer:  streamRenderer{},
	}
	var out, stderr bytes.Buffer
	code := RunWithDeps(nil, &out, &stderr, streamRenderer{}, deps)
	assert.Equal(t, 0, code)
	assert.Contains(t, out.String(), "Available Commands:")
	assert.Contains(t, out.String(), "map")
	assert.Contains(t, out.String(), "reduce")
	assert.Contains(t, out.String(), "gate")
}
