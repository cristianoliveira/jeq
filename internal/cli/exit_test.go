package cli_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/cristianoliveira/gev/internal/cli"
	"github.com/cristianoliveira/gev/internal/domain/gev"
)

// Exit code contract: 0 success, 1 invocation-side failure, 2 API-side failure,
// 130 interrupted.
func TestExitCodeMapping(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{"success", nil, 0},
		{"input invalid is invocation-side", gev.NewError(gev.CodeInputInvalid, "x"), 1},
		{"source conflict is invocation-side", gev.NewError(gev.CodeSourceConflict, "x"), 1},
		{"auth missing is API-side", gev.NewError(gev.CodeAuthMissing, "x"), 2},
		{"request invalid is API-side", gev.NewError(gev.CodeRequestInvalid, "x"), 2},
		{"rate limited is API-side", gev.NewError(gev.CodeRateLimited, "x"), 2},
		{"response invalid is API-side", gev.NewError(gev.CodeResponseInvalid, "x"), 2},
		{
			name: "class survives generic wrapping",
			err:  fmt.Errorf("ask: %w", gev.NewError(gev.CodeRateLimited, "x")),
			want: 2,
		},
		{"plain errors fall back to invocation-side", errors.New("boom"), 1},
		{"canceled context is interrupt", context.Canceled, 130},
		{"interrupt survives wrapping", fmt.Errorf("run: %w", context.Canceled), 130},
		{"deadline is not an interrupt", context.DeadlineExceeded, 1},
		{
			name: "a stable code wins over its interrupt cause",
			err:  gev.WrapError(gev.CodeRateLimited, context.Canceled, "api call"),
			want: 2,
		},
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
		gev.CodeAuthMissing:     2,
		gev.CodeRequestInvalid:  2,
		gev.CodeRateLimited:     2,
		gev.CodeResponseInvalid: 2,
		gev.CodeSourceConflict:  1,
		gev.CodeInputInvalid:    1,
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
