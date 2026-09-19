package cli_test

import (
	"bytes"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/cristianoliveira/jeq/internal/cli"
	"github.com/cristianoliveira/jeq/internal/domain/jeq"
)

func TestBareActionsEqualNativeHelpWithoutDependencyWork(t *testing.T) {
	commands := []string{"ask", "validate", "map", "reduce", "gate"}
	for _, command := range commands {
		t.Run(command, func(t *testing.T) {
			var envReads, fileReads, clientCreates, stdinReads int
			deps := cli.AskDeps{
				Getenv:    func(string) string { envReads++; return "secret" },
				ReadFile:  func(string, int64) ([]byte, *jeq.Error) { fileReads++; return nil, nil },
				ReadStdin: func(io.Reader, int64, bool) ([]byte, *jeq.Error) { stdinReads++; return nil, nil },
				NewClient: func(string, time.Duration, string, int, func(string)) cli.APIClient { clientCreates++; return nil },
			}
			var bare, help, stderr bytes.Buffer
			if code := cli.RunWithDeps([]string{command}, &bare, &stderr, nil, deps); code != 0 {
				t.Fatalf("bare exit=%d stderr=%q", code, stderr.String())
			}
			stderr.Reset()
			if code := cli.RunWithDeps([]string{command, "--help"}, &help, &stderr, nil, deps); code != 0 {
				t.Fatalf("help exit=%d stderr=%q", code, stderr.String())
			}
			if bare.String() != help.String() || bare.Len() == 0 || stderr.Len() != 0 {
				t.Fatalf("bare/help mismatch or stderr: bare=%q help=%q stderr=%q", bare.String(), help.String(), stderr.String())
			}
			if envReads != 0 || fileReads != 0 || clientCreates != 0 || stdinReads != 0 {
				t.Fatalf("dependency work: env=%d files=%d clients=%d stdin=%d", envReads, fileReads, clientCreates, stdinReads)
			}
		})
	}
}

func TestIncompleteRecipeInvocationUsesNativeHelp(t *testing.T) {
	var bare, help, stderr bytes.Buffer
	for _, id := range []string{"validate-native", "ask-native", "map-gate", "reduce-gate", "map-reduce-gate"} {
		bare.Reset()
		help.Reset()
		stderr.Reset()
		if code := cli.Run([]string{"examples", id}, &bare, &stderr, nil); code != 0 {
			t.Fatalf("%s bare exit=%d stderr=%q", id, code, stderr.String())
		}
		if code := cli.Run([]string{"examples", id, "--help"}, &help, &stderr, nil); code != 0 || bare.String() != help.String() {
			t.Fatalf("%s help mismatch: %q != %q", id, bare.String(), help.String())
		}
		if strings.Contains(bare.String(), "Next:") {
			t.Fatalf("%s has obsolete navigation text", id)
		}
	}
}
