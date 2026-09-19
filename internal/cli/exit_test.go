package cli_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/cristianoliveira/gev/internal/cli"
	"github.com/cristianoliveira/gev/internal/domain/contract"
	"github.com/cristianoliveira/gev/internal/domain/gev"
)

// Exit code contract per ADR 0001 § Errors:
// 0 success · 1 auth/API/network/timeout/response failure · 2 usage or locally
// invalid input · 130 interrupted.
func TestExitCodeMapping(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{"success", nil, 0},

		// Class 1: authentication, API, network, timeout, response failures.
		{"auth missing is API-side", gev.NewError(gev.CodeAuthMissing, "x"), 1},
		{"server 422 is API-side", gev.NewError(gev.CodeRequestRejected, "x"), 1},
		{"auth rejected is API-side", gev.NewError(gev.CodeAuthRejected, "x"), 1},
		{"rate limited is API-side", gev.NewError(gev.CodeRateLimited, "x"), 1},
		{"server error is API-side", gev.NewError(gev.CodeServerError, "x"), 1},
		{"response invalid is API-side", gev.NewError(gev.CodeResponseInvalid, "x"), 1},
		{"network error is API-side", gev.NewError(gev.CodeNetworkError, "x"), 1},
		{"timeout is API-side", gev.NewError(gev.CodeTimeout, "x"), 1},

		// Class 2: usage or locally invalid input.
		{"request invalid is usage-side", gev.NewError(gev.CodeRequestInvalid, "x"), 2},
		{"input invalid is usage-side", gev.NewError(gev.CodeInputInvalid, "x"), 2},
		{"source conflict is usage-side", gev.NewError(gev.CodeSourceConflict, "x"), 2},

		// Interrupts.
		{"interrupted code", gev.NewError(gev.CodeInterrupted, "x"), 130},
		{"canceled context is interrupt", context.Canceled, 130},
		{"interrupt survives wrapping", fmt.Errorf("run: %w", context.Canceled), 130},
		{"deadline is not an interrupt", context.DeadlineExceeded, 1},

		// Stability rules.
		{
			name: "class survives generic wrapping",
			err:  fmt.Errorf("ask: %w", gev.NewError(gev.CodeRateLimited, "x")),
			want: 1,
		},
		{
			name: "a stable code wins over its interrupt cause",
			err:  gev.WrapError(gev.CodeRateLimited, context.Canceled, "api call"),
			want: 1,
		},
		{"plain errors fall back to API-side default", errors.New("boom"), 1},

		// Usage failures from the shell layer.
		{"unknown command is usage", cli.NewUsageError(errors.New(`unknown command "badsub" for "gev"`)), 2},
		{"flag parse error is usage", cli.NewUsageError(errors.New("unknown flag: --nope")), 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := cli.ExitCode(tt.err); got != tt.want {
				t.Errorf("ExitCode = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestEveryStableCodeHasAnExitClass(t *testing.T) {
	// Guards the mapping against registry drift: a new or renamed code must
	// land here or the gate fails.
	want := map[gev.Code]int{
		gev.CodeAuthMissing:     1,
		gev.CodeAuthRejected:    1,
		gev.CodeRequestRejected: 1,
		gev.CodeRateLimited:     1,
		gev.CodeServerError:     1,
		gev.CodeResponseInvalid: 1,
		gev.CodeNetworkError:    1,
		gev.CodeTimeout:         1,
		gev.CodeRequestInvalid:  2,
		gev.CodeInputInvalid:    2,
		gev.CodeSourceConflict:  2,
		gev.CodeInterrupted:     130,
	}

	codes := gev.Codes()
	if len(codes) != len(want) {
		t.Fatalf("code registry changed: got %v; exit mapping must cover exactly the stable codes", codes)
	}
	for _, code := range codes {
		if got := cli.ExitCode(gev.NewError(code, "probe")); got != want[code] {
			t.Errorf("code %s maps to exit %d, want %d", code, got, want[code])
		}
	}
}

func TestOutputFlagIsUnknownAndUsesPlainStderr(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if got := cli.Run([]string{"version", "--output", "toon"}, &stdout, &stderr, &stubRenderer{}); got != 2 {
		t.Fatalf("output flag exit = %d, want 2", got)
	}
	if stdout.Len() != 0 || !strings.Contains(stderr.String(), "unknown flag") {
		t.Fatalf("stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

func TestRunExitCodesEndToEnd(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want int
	}{
		{"version succeeds", []string{"version"}, 0},
		{"unknown command is usage", []string{"badsubcommand"}, 2},
		{"unknown flag is usage", []string{"--nope"}, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			renderer := &stubRenderer{}
			if got := cli.Run(tt.args, &stdout, &stderr, renderer); got != tt.want {
				t.Errorf("Run(%q) exit = %d, want %d (stderr: %q)", tt.args, got, tt.want, stderr.String())
			}
		})
	}
}

func TestUsageFailuresArePlainStderr(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		wantNamed   string
		wantSuggest string
	}{
		{"unknown command", []string{"badsubcommand"}, `unknown command "badsubcommand"`, ""},
		{"unknown flag", []string{"--nope"}, "unknown flag: --nope", ""},
		{"misspelled command suggests closest", []string{"versionn"}, `unknown command "versionn"`, "did you mean \"version\"?"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			renderer := &stubRenderer{}
			if got := cli.Run(tt.args, &stdout, &stderr, renderer); got != 2 {
				t.Fatalf("exit = %d, want 2", got)
			}

			if stdout.Len() != 0 || !strings.Contains(stderr.String(), "Error:") {
				t.Errorf("stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			if renderer.errDoc == nil {
				t.Fatal("renderer did not receive an error document")
			}
			doc := *renderer.errDoc
			if doc.Code != gev.CodeInputInvalid {
				t.Errorf("code = %q, want %q", doc.Code, gev.CodeInputInvalid)
			}
			if !strings.Contains(doc.Message, tt.wantNamed) {
				t.Errorf("message %q does not name the offending input %q", doc.Message, tt.wantNamed)
			}
			if tt.wantSuggest != "" && !strings.Contains(doc.Recovery, "version") {
				t.Errorf("recovery %q lacks the closest alternative", doc.Recovery)
			}
			if !strings.Contains(doc.Recovery, "--help") {
				t.Errorf("recovery %q does not point at the command list", doc.Recovery)
			}
		})
	}
}

// stubRenderer records the error document passed through the injected port.
type stubRenderer struct {
	errDoc *gev.Error
}

func (r *stubRenderer) RenderSuccess(_ io.Writer, _ contract.Response) error { return nil }
func (r *stubRenderer) RenderValue(_ io.Writer, _ any) error                 { return nil }
func (r *stubRenderer) RenderError(_ io.Writer, e *gev.Error) error {
	r.errDoc = e
	return nil
}
