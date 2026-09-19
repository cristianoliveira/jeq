package gev_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/cristianoliveira/gev/internal/domain/gev"
)

func TestStableErrorCodes(t *testing.T) {
	// Code values are the automation contract; they must never drift.
	tests := []struct {
		name string
		code gev.Code
		want string
	}{
		{"auth missing", gev.CodeAuthMissing, "GEV_AUTH_MISSING"},
		{"request invalid", gev.CodeRequestInvalid, "GEV_REQUEST_INVALID"},
		{"rate limited", gev.CodeRateLimited, "GEV_RATE_LIMITED"},
		{"response invalid", gev.CodeResponseInvalid, "GEV_RESPONSE_INVALID"},
		{"source conflict", gev.CodeSourceConflict, "GEV_SOURCE_CONFLICT"},
		{"input invalid", gev.CodeInputInvalid, "GEV_INPUT_INVALID"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.code) != tt.want {
				t.Errorf("code = %q, want %q", tt.code, tt.want)
			}
		})
	}
}

func TestCodesRegistry(t *testing.T) {
	// The registry is exhaustive and ordered; nothing may be added or dropped silently.
	want := []gev.Code{
		gev.CodeAuthMissing,
		gev.CodeRequestInvalid,
		gev.CodeRateLimited,
		gev.CodeResponseInvalid,
		gev.CodeSourceConflict,
		gev.CodeInputInvalid,
	}

	got := gev.Codes()
	if len(got) != len(want) {
		t.Fatalf("Codes() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Codes()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestErrorRendering(t *testing.T) {
	tests := []struct {
		name string
		err  *gev.Error
		want string
	}{
		{
			name: "coded error without cause",
			err:  gev.NewError(gev.CodeInputInvalid, "question is required"),
			want: "GEV_INPUT_INVALID: question is required",
		},
		{
			name: "coded error keeps the internal cause visible",
			err:  gev.WrapError(gev.CodeSourceConflict, errors.New("two state sources"), "cannot merge"),
			want: "GEV_SOURCE_CONFLICT: cannot merge: two state sources",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.want {
				t.Errorf("Error() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestErrorUnwrapping(t *testing.T) {
	cause := fmt.Errorf("connection refused")

	t.Run("errors.As finds the coded error through wrapping", func(t *testing.T) {
		wrapped := fmt.Errorf("ask: %w", gev.NewError(gev.CodeRateLimited, "slow down"))
		var coded *gev.Error
		if !errors.As(wrapped, &coded) {
			t.Fatal("errors.As did not find the coded error")
		}
		if coded.Code != gev.CodeRateLimited {
			t.Errorf("code = %q, want %q", coded.Code, gev.CodeRateLimited)
		}
	})

	t.Run("errors.Is finds the internal cause", func(t *testing.T) {
		wrapped := gev.WrapError(gev.CodeRequestInvalid, cause, "bad payload")
		if !errors.Is(wrapped, cause) {
			t.Error("errors.Is did not find the internal cause")
		}
	})

	t.Run("error without cause unwraps to nil", func(t *testing.T) {
		if unwrapped := errors.Unwrap(gev.NewError(gev.CodeInputInvalid, "x")); unwrapped != nil {
			t.Errorf("Unwrap() = %v, want nil", unwrapped)
		}
	})
}
