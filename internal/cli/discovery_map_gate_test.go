package cli

import (
	"bytes"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/cristianoliveira/gev/internal/domain/gev"
)

func TestMapAndGateRejectUnsupportedOutputBeforeDependencies(t *testing.T) {
	for _, command := range []string{"map", "gate"} {
		t.Run(command, func(t *testing.T) {
			reads := 0
			deps := AskDeps{
				ReadStdin: func(_ io.Reader, _ int64, _ bool) ([]byte, *gev.Error) { reads++; return nil, nil },
				ReadFile:  func(string, int64) ([]byte, *gev.Error) { reads++; return nil, nil },
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
			if code := RunWithDeps(args, &out, &stderr, streamRenderer{}, deps); code != 2 || reads != 0 {
				t.Fatalf("code=%d reads=%d out=%q stderr=%q", code, reads, out.String(), stderr.String())
			}
		})
	}
}

func TestHomeDiscoveryIncludesAvailableMapAndGate(t *testing.T) {
	deps := AskDeps{
		ReadFile:  func(string, int64) ([]byte, *gev.Error) { return nil, nil },
		ReadStdin: func(_ io.Reader, _ int64, _ bool) ([]byte, *gev.Error) { return nil, nil },
		Getenv:    func(string) string { return "" },
		NewClient: func(string, time.Duration, string, int, func(string)) APIClient { return nil },
		Renderer:  streamRenderer{},
	}
	var out, stderr bytes.Buffer
	if code := RunWithDeps(nil, &out, &stderr, streamRenderer{}, deps); code != 0 {
		t.Fatalf("code=%d out=%q stderr=%q", code, out.String(), stderr.String())
	}
	if !strings.Contains(out.String(), "Commands:") || !strings.Contains(out.String(), "map") || !strings.Contains(out.String(), "reduce") || !strings.Contains(out.String(), "gate") {
		t.Fatalf("home=%q", out.String())
	}
}
