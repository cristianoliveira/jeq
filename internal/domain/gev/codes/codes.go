// Package codes is the leaf holding the stable error-code registry and
// the coded error type. It imports nothing internal, so both contract and
// gev can depend on it without a cycle.
package codes

import "strings"

// Code is a stable, machine-matchable failure class.
// Values are an automation contract: never rename, never reuse, append only.
// Format is locked: GEV_<AREA>_<REASON>, uppercase A-Z, 0-9, underscore.
type Code string

// Stable error codes, in contractual (registry) order.
const (
	CodeAuthMissing     Code = "GEV_AUTH_MISSING"
	CodeAuthRejected    Code = "GEV_AUTH_REJECTED"
	CodeRequestInvalid  Code = "GEV_REQUEST_INVALID"  // local document validation only (exit 2)
	CodeRequestRejected Code = "GEV_REQUEST_REJECTED" // server 422: the server rejected fields gev cannot check locally
	CodeSourceConflict  Code = "GEV_SOURCE_CONFLICT"
	CodeInputInvalid    Code = "GEV_INPUT_INVALID"
	CodeRateLimited     Code = "GEV_RATE_LIMITED"
	CodeServerError     Code = "GEV_SERVER_ERROR"
	CodeResponseInvalid Code = "GEV_RESPONSE_INVALID"
	CodeNetworkError    Code = "GEV_NETWORK_ERROR"
	CodeTimeout         Code = "GEV_TIMEOUT"
	CodeInterrupted     Code = "GEV_INTERRUPTED"
)

// Codes returns every stable code in contractual order.
func Codes() []Code {
	return []Code{
		CodeAuthMissing,
		CodeAuthRejected,
		CodeRequestInvalid,
		CodeRequestRejected,
		CodeSourceConflict,
		CodeInputInvalid,
		CodeRateLimited,
		CodeServerError,
		CodeResponseInvalid,
		CodeNetworkError,
		CodeTimeout,
		CodeInterrupted,
	}
}

// Error couples a stable code with a human message and one internal cause.
// The cause stays internal: machine-facing output renders the code, prose
// serves humans debugging a run.
type Error struct {
	Code    Code
	Message string
	// Recovery is one actionable instruction for the caller. Renderers must
	// emit it; it never contains secrets or raw dependency prose.
	Recovery string

	cause error
}

// NewError builds a coded error without a cause.
func NewError(code Code, message string) *Error {
	return &Error{Code: code, Message: message}
}

// WithRecovery attaches the actionable instruction and returns the error
// for fluent construction.
func (e *Error) WithRecovery(recovery string) *Error {
	e.Recovery = recovery
	return e
}

// WrapError builds a coded error that preserves one internal cause.
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

// Unwrap exposes the internal cause for errors.Is and errors.As
// (the %w-equivalent for structured errors).
func (e *Error) Unwrap() error { return e.cause }
