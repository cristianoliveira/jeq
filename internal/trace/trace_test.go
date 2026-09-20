package trace

import (
	"bytes"
	"strings"
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
