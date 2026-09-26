package cli

import (
	"bytes"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/cristianoliveira/jeq/internal/domain/jeq"
)

func TestMapAndReduceRejectQuestionSourceConflictBeforeReading(t *testing.T) {
	for _, command := range []string{"map", "reduce"} {
		t.Run(command, func(t *testing.T) {
			reads := 0
			deps := AskDeps{
				Stdin:     strings.NewReader(`[{"x":1}]`),
				ReadStdin: func(io.Reader, int64, bool) ([]byte, *jeq.Error) { reads++; return []byte(`[{"x":1}]`), nil },
				ReadFile:  func(string, int64) ([]byte, *jeq.Error) { reads++; return []byte(`{"questions":{}}`), nil },
				Getenv:    func(string) string { reads++; return "" },
				NewClient: func(string, time.Duration, string, int, func(string)) APIClient { reads++; return nil },
			}
			args := []string{command, "--as", "x", "--questions", "q.json", "--questions-json", `{"questions":{}}`}
			if command == "map" {
				args = append(args, "--model", "m")
			}
			code := RunWithDeps(args, &bytes.Buffer{}, &bytes.Buffer{}, streamRenderer{}, deps)
			assert.Equal(t, 2, code)
			assert.Zero(t, reads)
		})
	}
}

func TestMapAndReduceRejectOversizedInlineQuestions(t *testing.T) {
	large := `{"questions":{"q":{"type":"noul","instructions":"` + strings.Repeat("x", MapMaxRecordBytes) + `"}}}`
	for _, command := range []string{"map", "reduce"} {
		t.Run(command, func(t *testing.T) {
			args := []string{command, "--as", "x", "--questions-json", large, "--model", "m"}
			if command == "reduce" {
				args = append(args, "--input", "ndjson")
			}
			code := RunWithDeps(args, &bytes.Buffer{}, &bytes.Buffer{}, streamRenderer{}, reduceDeps(&reduceTestClient{}, `{"x":1}`))
			assert.Equal(t, 2, code)
		})
	}
}
