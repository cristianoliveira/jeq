// Package gev holds gev's own rules: ask modes, source matrix, and input
// composition (ADR 0002). It is pure Go: no Cobra or HTTP here.
//
// The stable error-code registry lives in the codes leaf so the contract
// package can use it without an import cycle; gev re-exports it below as
// the single public surface for the rest of the CLI.
package gev

import "github.com/cristianoliveira/gev/internal/domain/gev/codes"

// Code is a stable, machine-matchable failure class.
type Code = codes.Code

// Stable error codes, re-exported from the registry leaf.
const (
	CodeAuthMissing     = codes.CodeAuthMissing
	CodeAuthRejected    = codes.CodeAuthRejected
	CodeRequestInvalid  = codes.CodeRequestInvalid
	CodeRequestRejected = codes.CodeRequestRejected
	CodeSourceConflict  = codes.CodeSourceConflict
	CodeInputInvalid    = codes.CodeInputInvalid
	CodeRateLimited     = codes.CodeRateLimited
	CodeServerError     = codes.CodeServerError
	CodeResponseInvalid = codes.CodeResponseInvalid
	CodeNetworkError    = codes.CodeNetworkError
	CodeTimeout         = codes.CodeTimeout
	CodeInterrupted     = codes.CodeInterrupted
)

// Error couples a stable code with a human message and one internal cause.
type Error = codes.Error

// Codes returns every stable code in contractual order.
func Codes() []Code { return codes.Codes() }

// NewError builds a coded error without a cause.
func NewError(code Code, message string) *Error { return codes.NewError(code, message) }

// WrapError builds a coded error that preserves one internal cause.
func WrapError(code Code, cause error, message string) *Error {
	return codes.WrapError(code, cause, message)
}
