package gev

import "fmt"

// Source identifies one ask input source.
type Source int

// Input sources. SourceFile kinds are resolved to content by the shell
// layer; the domain only sees presence plus resolved values.
const (
	SourceNone Source = iota
	SourceRequest
	SourceQuestions
	SourceStateText
	SourceStateFile
	SourceStateJSON
)

// Sources records which ask input sources are present on the command line.
type Sources struct {
	Request   bool
	Questions bool
	StateText bool
	StateFile bool
	StateJSON bool
}

func (s Sources) stateCount() int {
	n := 0
	for _, b := range []bool{s.StateText, s.StateFile, s.StateJSON} {
		if b {
			n++
		}
	}
	return n
}

// CheckSources validates the ask mode matrix (ADR 0001 Request modes):
// native mode is --request alone; composed mode is --questions plus exactly
// one state source. It is pure and must run before any I/O.
func CheckSources(s Sources) *Error {
	stateCount := s.stateCount()

	if s.Request && (s.Questions || stateCount > 0) {
		return NewError(CodeSourceConflict,
			"--request is a complete document and is mutually exclusive with --questions and state sources")
	}
	if stateCount > 1 {
		return NewError(CodeSourceConflict,
			fmt.Sprintf("exactly one state source is allowed, got %d; the sources never merge", stateCount))
	}
	if !s.Request && !s.Questions {
		return NewError(CodeInputInvalid,
			"nothing to ask: provide --request, or --questions with exactly one state source")
	}
	if s.Questions && stateCount == 0 {
		return NewError(CodeInputInvalid,
			"--questions needs exactly one state source: --state, --state-file, or --state-json")
	}
	return nil
}
