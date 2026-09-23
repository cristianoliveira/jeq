package examples_test

import "testing"

func TestLayaEvaluationHarnessUsesControlledLocalResponses(t *testing.T) {
	result := runScript(t, "examples/laya-evaluation/test.sh", "", "", nil)
	if result.exit != 0 || result.stdout != "PASS laya evaluation harness\n" || result.stderr != "" {
		t.Fatalf("exit=%d stdout=%q stderr=%q", result.exit, result.stdout, result.stderr)
	}
}
