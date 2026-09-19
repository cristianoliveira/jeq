package cli

// UsageError marks an invocation problem: unknown command or invalid flag.
// Per ADR 0001 these are usage failures (exit 2), not generic errors.
type UsageError struct {
	err error
}

// NewUsageError wraps a shell-level invocation failure.
func NewUsageError(err error) *UsageError {
	return &UsageError{err: err}
}

// Error renders the underlying invocation failure.
func (e *UsageError) Error() string { return e.err.Error() }

// Unwrap exposes the underlying failure.
func (e *UsageError) Unwrap() error { return e.err }
