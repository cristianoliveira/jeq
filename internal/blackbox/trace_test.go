package blackbox_test

import (
	"strings"
	"testing"
)

func TestBlackBoxVerboseKeepsStdoutSeparateAndCorrelates(t *testing.T) {
	off := runBinary(t, "", nil, "version")
	on := runBinary(t, "", map[string]string{"JEQ_TRACE_ID": "chain-42"}, "--verbose", "version")
	if off.exit != 0 || on.exit != 0 || off.stdout != on.stdout {
		t.Fatalf("stdout changed: off=%q on=%q", off.stdout, on.stdout)
	}
	for _, field := range []string{`"schema":"jeq.trace.v1"`, `"trace_id":"chain-42"`, `"event":"run.started"`, `"event":"run.completed"`} {
		if !strings.Contains(on.stderr, field) {
			t.Fatalf("trace missing %s: %s", field, on.stderr)
		}
	}
	if strings.Contains(on.stdout, "jeq.trace.v1") {
		t.Fatal("trace entered stdout")
	}
}
