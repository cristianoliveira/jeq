package jeq_test

import (
	"errors"
	"fmt"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cristianoliveira/jeq/internal/domain/jeq"
)

func TestStableErrorCodes(t *testing.T) {
	// Code values are the automation contract; they must never drift.
	tests := []struct {
		name string
		code jeq.Code
		want string
	}{
		{"auth missing", jeq.CodeAuthMissing, "JEQ_AUTH_MISSING"},
		{"auth rejected", jeq.CodeAuthRejected, "JEQ_AUTH_REJECTED"},
		{"request invalid", jeq.CodeRequestInvalid, "JEQ_REQUEST_INVALID"},
		{"request rejected", jeq.CodeRequestRejected, "JEQ_REQUEST_REJECTED"},
		{"source conflict", jeq.CodeSourceConflict, "JEQ_SOURCE_CONFLICT"},
		{"input invalid", jeq.CodeInputInvalid, "JEQ_INPUT_INVALID"},
		{"rate limited", jeq.CodeRateLimited, "JEQ_RATE_LIMITED"},
		{"server error", jeq.CodeServerError, "JEQ_SERVER_ERROR"},
		{"response invalid", jeq.CodeResponseInvalid, "JEQ_RESPONSE_INVALID"},
		{"network error", jeq.CodeNetworkError, "JEQ_NETWORK_ERROR"},
		{"timeout", jeq.CodeTimeout, "JEQ_TIMEOUT"},
		{"interrupted", jeq.CodeInterrupted, "JEQ_INTERRUPTED"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, string(tt.code))
		})
	}
}

func TestErrorCodeFormat(t *testing.T) {
	// Locked format: JEQ_<AREA>_<REASON>, uppercase A-Z, 0-9, underscore only.
	re := regexp.MustCompile(`^JEQ_[A-Z0-9_]+$`)

	for _, code := range jeq.Codes() {
		assert.Regexp(t, re, string(code))
	}
}

func TestCodesRegistry(t *testing.T) {
	// The registry is exhaustive and ordered; nothing may be added or dropped silently.
	want := []jeq.Code{
		jeq.CodeAuthMissing,
		jeq.CodeAuthRejected,
		jeq.CodeRequestInvalid,
		jeq.CodeRequestRejected,
		jeq.CodeSourceConflict,
		jeq.CodeInputInvalid,
		jeq.CodeRateLimited,
		jeq.CodeServerError,
		jeq.CodeResponseInvalid,
		jeq.CodeNetworkError,
		jeq.CodeTimeout,
		jeq.CodeInterrupted,
	}

	got := jeq.Codes()
	assert.Equal(t, want, got)
}

func TestErrorRendering(t *testing.T) {
	tests := []struct {
		name string
		err  *jeq.Error
		want string
	}{
		{
			name: "coded error without cause",
			err:  jeq.NewError(jeq.CodeInputInvalid, "question is required"),
			want: "JEQ_INPUT_INVALID: question is required",
		},
		{
			name: "coded error keeps the internal cause visible",
			err:  jeq.WrapError(jeq.CodeSourceConflict, errors.New("two state sources"), "cannot merge"),
			want: "JEQ_SOURCE_CONFLICT: cannot merge: two state sources",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.err.Error())
		})
	}
}

func TestErrorUnwrapping(t *testing.T) {
	cause := fmt.Errorf("connection refused")

	t.Run("errors.As finds the coded error through wrapping", func(t *testing.T) {
		wrapped := fmt.Errorf("ask: %w", jeq.NewError(jeq.CodeRateLimited, "slow down"))
		var coded *jeq.Error
		require.True(t, errors.As(wrapped, &coded))
		require.NotNil(t, coded)
		assert.Equal(t, jeq.CodeRateLimited, coded.Code)
	})

	t.Run("errors.Is finds the internal cause", func(t *testing.T) {
		wrapped := jeq.WrapError(jeq.CodeRequestInvalid, cause, "bad payload")
		assert.ErrorIs(t, wrapped, cause)
	})

	t.Run("error without cause unwraps to nil", func(t *testing.T) {
		assert.Nil(t, errors.Unwrap(jeq.NewError(jeq.CodeInputInvalid, "x")))
	})
}
