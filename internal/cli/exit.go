package cli

import (
	"context"
	"errors"

	"github.com/cristianoliveira/jeq/internal/domain/jeq"
)

// ExitCode maps an error to the process exit code (ADR 0001 § Errors):
//
//	 0  success, regardless of confidence or probability
//	 1  authentication, API, network, timeout, or response failure:
//	    JEQ_AUTH_MISSING, JEQ_AUTH_REJECTED, JEQ_REQUEST_REJECTED
//	    (server 422), JEQ_RATE_LIMITED, JEQ_SERVER_ERROR,
//	    JEQ_RESPONSE_INVALID, JEQ_NETWORK_ERROR, JEQ_TIMEOUT;
//	    unknown failures default here
//	 2  usage or locally invalid input: JEQ_REQUEST_INVALID (local document
//	    validation only), JEQ_INPUT_INVALID, JEQ_SOURCE_CONFLICT; also
//	    unknown commands and flags
//	130 interrupted: JEQ_INTERRUPTED or canceled context (SIGINT maps here)
//
// A stable code always wins over its cause's class.
func ExitCode(err error) int {
	if err == nil {
		return 0
	}

	var coded *jeq.Error
	if errors.As(err, &coded) {
		switch coded.Code {
		case jeq.CodeAuthMissing, jeq.CodeAuthRejected, jeq.CodeRequestRejected,
			jeq.CodeRateLimited, jeq.CodeServerError, jeq.CodeResponseInvalid,
			jeq.CodeNetworkError, jeq.CodeTimeout:
			return 1
		case jeq.CodeRequestInvalid, jeq.CodeInputInvalid, jeq.CodeSourceConflict:
			return 2
		case jeq.CodeInterrupted:
			return 130
		}
	}

	var usage *UsageError
	if errors.As(err, &usage) {
		return 2
	}

	if errors.Is(err, context.Canceled) {
		return 130
	}

	return 1
}
