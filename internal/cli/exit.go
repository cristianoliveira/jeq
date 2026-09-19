package cli

import (
	"context"
	"errors"

	"github.com/cristianoliveira/gev/internal/domain/gev"
)

// ExitCode maps an error to the process exit code (ADR 0001 § Errors):
//
//	 0  success, regardless of confidence or probability
//	 1  authentication, API, network, timeout, or response failure:
//	    GEV_AUTH_MISSING, GEV_AUTH_REJECTED, GEV_RATE_LIMITED,
//	    GEV_SERVER_ERROR, GEV_RESPONSE_INVALID, GEV_NETWORK_ERROR,
//	    GEV_TIMEOUT; unknown failures default here
//	 2  usage or locally invalid input: GEV_REQUEST_INVALID (local document
//	    validation only), GEV_INPUT_INVALID, GEV_SOURCE_CONFLICT; also
//	    unknown commands and flags
//	130 interrupted: GEV_INTERRUPTED or canceled context (SIGINT maps here)
//
// A stable code always wins over its cause's class.
func ExitCode(err error) int {
	if err == nil {
		return 0
	}

	var coded *gev.Error
	if errors.As(err, &coded) {
		switch coded.Code {
		case gev.CodeAuthMissing, gev.CodeAuthRejected, gev.CodeRateLimited,
			gev.CodeServerError, gev.CodeResponseInvalid, gev.CodeNetworkError,
			gev.CodeTimeout:
			return 1
		case gev.CodeRequestInvalid, gev.CodeInputInvalid, gev.CodeSourceConflict:
			return 2
		case gev.CodeInterrupted:
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
