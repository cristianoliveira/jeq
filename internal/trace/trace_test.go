package trace

import (
	"bytes"
	"encoding/json"
	"strings"
	"sync"
	"testing"
)

func TestTraceIsVersionedBoundedAndEphemeral(t *testing.T) {
	var out bytes.Buffer
	cfg := New(true, "review-42", &out)
	cfg.Emit("jeq map", "run.started", "preflight", "started", "")
	line := strings.TrimSpace(out.String())
	for _, forbidden := range []string{"secret", "prompt", "Authorization"} {
		if strings.Contains(line, forbidden) {
			t.Fatalf("trace leaked %q: %s", forbidden, line)
		}
	}
	if !strings.Contains(line, `"schema":"jeq.trace.v1"`) || !strings.Contains(line, `"sequence":1`) {
		t.Fatalf("unexpected event: %s", line)
	}
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
	if len(lines) < 65 || len(lines) > 257 {
		t.Fatalf("event count=%d", len(lines))
	}
	previous, suppressed := uint64(0), 0
	for _, line := range lines {
		var event struct {
			Schema   string `json:"schema"`
			Sequence uint64 `json:"sequence"`
			Event    string `json:"event"`
		}
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatalf("invalid line: %v", err)
		}
		if event.Schema != Schema || event.Sequence <= previous {
			t.Fatalf("sequence order: previous=%d current=%d", previous, event.Sequence)
		}
		previous = event.Sequence
		if event.Event == "events.suppressed" {
			suppressed++
		}
	}
	if suppressed != 1 {
		t.Fatalf("suppression events=%d", suppressed)
	}
}

func TestTraceIDValidationAndPrecedence(t *testing.T) {
	if id, ok := ResolveID("flag", "env"); !ok || id != "flag" {
		t.Fatal("flag precedence failed")
	}
	if _, ok := ResolveID(strings.Repeat("x", 129), ""); ok {
		t.Fatal("oversized id accepted")
	}
	if _, ok := ResolveID("bad space", ""); ok {
		t.Fatal("unsafe id accepted")
	}
}

func TestNoopTraceEmitsNothing(t *testing.T) {
	var out bytes.Buffer
	New(false, "", &out).Emit("jeq", "run.started", "", "", "")
	if out.Len() != 0 {
		t.Fatal("no-op trace emitted output")
	}
}
