package trace

import (
	"bytes"
	"encoding/json"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTraceIsVersionedBoundedAndEphemeral(t *testing.T) {
	var out bytes.Buffer
	cfg := New(true, "review-42", &out)
	cfg.Emit("jeq map", "run.started", "preflight", "started", "")
	line := strings.TrimSpace(out.String())
	for _, forbidden := range []string{"secret", "prompt", "Authorization"} {
		assert.NotContains(t, line, forbidden)
	}
	assert.Contains(t, line, `"schema":"jeq.trace.v1"`)
	assert.Contains(t, line, `"sequence":1`)
}

func TestTraceConcurrentObserversKeepOrderedCompleteLines(t *testing.T) {
	var out bytes.Buffer
	cfg := New(true, "concurrent", &out)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			for j := 0; j < 30; j++ {
				cfg.EmitOperation("jeq map", "operation.started", "evaluation", "started", "", i*30+j, 480)
				cfg.Attempt(j+1, 2)
				cfg.Retrying(j+1, 2, 529)
			}
		}(i)
	}
	close(start)
	wg.Wait()
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	require.GreaterOrEqual(t, len(lines), 65)
	require.LessOrEqual(t, len(lines), 257)
	previous, suppressed := uint64(0), 0
	for _, line := range lines {
		var event struct {
			Schema   string `json:"schema"`
			Sequence uint64 `json:"sequence"`
			Event    string `json:"event"`
		}
		require.NoError(t, json.Unmarshal([]byte(line), &event))
		assert.Equal(t, Schema, event.Schema)
		require.Greater(t, event.Sequence, previous, "trace sequences must increase")
		previous = event.Sequence
		if event.Event == "events.suppressed" {
			suppressed++
		}
	}
	require.Equal(t, 1, suppressed)
}

func TestTraceIDValidationAndPrecedence(t *testing.T) {
	id, ok := ResolveID("flag", "env")
	require.True(t, ok)
	assert.Equal(t, "flag", id)
	_, ok = ResolveID(strings.Repeat("x", 129), "")
	assert.False(t, ok, "oversized id must be rejected")
	_, ok = ResolveID("bad space", "")
	assert.False(t, ok, "unsafe id must be rejected")
}

func TestNoopTraceEmitsNothing(t *testing.T) {
	var out bytes.Buffer
	New(false, "", &out).Emit("jeq", "run.started", "", "", "")
	assert.Zero(t, out.Len(), "no-op trace must not emit output")
}
