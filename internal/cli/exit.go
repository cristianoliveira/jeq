package cli

import (
	"context"
	"errors"

	"github.com/cristianoliveira/gev/internal/domain/gev"
)

// ExitCode maps an error to the process exit code:
//
//	 0  success
//	 1  invocation-side failure (GEV_INPUT_INVALID, GEV_SOURCE_CONFLICT,
//	    unknown errors): fix the command line
//	 2  API-side failure (GEV_AUTH_MISSING, GEV_REQUEST_INVALID,
//	    GEV_RATE_LIMITED, GEV_RESPONSE_INVALID): fix credentials or back off
//	130 interrupted (canceled context; SIGINT maps here)
//
// A stable code always wins over its cause's class.
func ExitCode(err error) int {
	if err == nil {
		return 0
	}

	var coded *gev.Error
	if errors.As(err, &coded) {
		switch coded.Code {
		case gev.CodeAuthMissing, gev.CodeRequestInvalid, gev.CodeRateLimited, gev.CodeResponseInvalid:
			return 2
		case gev.CodeInputInvalid, gev.CodeSourceConflict:
			return 1
		}
	}

	if errors.Is(err, context.Canceled) {
		return 130
	}

	return 1
}
