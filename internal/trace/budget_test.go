package trace

import (
	"bytes"
	"strings"
	"testing"
)

func TestDetailBudgetSuppressesTenThousandOperations(t *testing.T) {
	var out bytes.Buffer
	cfg := New(true, "bounded", &out)
	for i := 0; i < 10000; i++ {
		cfg.EmitOperation("jeq map", "operation.started", "evaluation", "started", "", i+1, 10000)
		cfg.EmitOperation("jeq map", "operation.completed", "evaluation", "success", "", i+1, 10000)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) > 260 {
		t.Fatalf("events=%d", len(lines))
	}
	if !strings.Contains(out.String(), `"event":"events.suppressed"`) {
		t.Fatal("suppression event missing")
	}
}
