package trace

import (
	"bytes"
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
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			cfg.EmitOperation("jeq map", "operation.started", "evaluation", "started", "", i, 8)
			cfg.Attempt(i+1, 200)
			cfg.Retrying(i+1, 2, 529)
		}(i)
	}
	wg.Wait()
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) == 0 {
		t.Fatal("no trace events")
	}
	for _, line := range lines {
		if !strings.HasSuffix(line, "}") || !strings.Contains(line, `"schema":"jeq.trace.v1"`) {
			t.Fatalf("torn trace line: %q", line)
		}
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
