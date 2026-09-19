// Package gev holds gev's own rules: stable error codes, ask modes, sources,
// and input composition. It is the center ring of ADR 0002: pure Go,
// standard library only — no Cobra, HTTP, or TOON here.
package gev

import "strings"

// Code is a stable, machine-matchable failure class.
// Values are an automation contract: never rename, never reuse, append only.
type Code string

// Stable error codes.
const (
	CodeAuthMissing     Code = "GEV_AUTH_MISSING"
	CodeRequestInvalid  Code = "GEV_REQUEST_INVALID"
	CodeRateLimited     Code = "GEV_RATE_LIMITED"
	CodeResponseInvalid Code = "GEV_RESPONSE_INVALID"
	CodeSourceConflict  Code = "GEV_SOURCE_CONFLICT"
	CodeInputInvalid    Code = "GEV_INPUT_INVALID"
)

// Codes returns every stable code in contractual order.
func Codes() []Code {
	return []Code{
		CodeAuthMissing,
		CodeRequestInvalid,
		CodeRateLimited,
		CodeResponseInvalid,
		CodeSourceConflict,
		CodeInputInvalid,
	}
}

// Error couples a stable code with a human message and an internal cause.
// The cause stays internal: machine-facing output renders the code, prose
// serves humans debugging a run.
type Error struct {
	Code    Code
	Message string

	cause error
}

// NewError builds a coded error without a cause.
func NewError(code Code, message string) *Error {
	return &Error{Code: code, Message: message}
}

// WrapError builds a coded error that preserves an internal cause.
func WrapError(code Code, cause error, message string) *Error {
	return &Error{Code: code, Message: message, cause: cause}
}

// Error renders "CODE: message" and appends the cause when present.
func (e *Error) Error() string {
	var b strings.Builder
	b.WriteString(string(e.Code))
	b.WriteString(": ")
	b.WriteString(e.Message)
	if e.cause != nil {
		b.WriteString(": ")
		b.WriteString(e.cause.Error())
	}
	return b.String()
}

// Unwrap exposes the internal cause for errors.Is and errors.As.
func (e *Error) Unwrap() error { return e.cause }
