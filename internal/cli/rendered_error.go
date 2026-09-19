package cli

import "github.com/cristianoliveira/gev/internal/domain/gev"

// renderedError marks an error whose structured receipt was already written
// to the command stream. RunWithDeps must not duplicate that receipt.
type renderedError struct{ err *gev.Error }

func (e *renderedError) Error() string { return e.err.Error() }
func (e *renderedError) Unwrap() error { return e.err }
