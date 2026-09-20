package blackbox_test

import (
	"net/http"
	"strings"
	"testing"
)

func TestBlackBoxVerbosePartialMapFailurePreservesPriorOutput(t *testing.T) {
	response := fixture(t, "response_200_full.json")
	api := newFakeAPI(t, func(w http.ResponseWriter, _ *http.Request, count int) {
		if count == 1 {
			_, _ = w.Write(response)
			return
		}
		http.Error(w, "private provider detail", 500)
	})
	input := `{"description":"first"}` + "\n" + `{"description":"second"}` + "\n"
	result := runBinary(t, input, map[string]string{"TYPESAFE_BASE_URL": api.server.URL, "TYPESAFE_API_KEY": "partial-secret", "JEQ_TRACE_ID": "partial"}, "--verbose", "map", "--as", "risk", "--input", "ndjson", "--state-pointer", "/description", "--questions-json", `{"questions":{"risk":{"type":"noul","instructions":"private prompt"}}}`)
	if result.exit != 1 || !strings.Contains(result.stdout, `"first"`) || strings.Contains(result.stdout, "second") {
		t.Fatalf("exit=%d stdout=%q", result.exit, result.stdout)
	}
	if !strings.Contains(result.stderr, `"operation_index":2`) || strings.Contains(result.stderr, "private") || strings.Contains(result.stderr, "partial-secret") {
		t.Fatalf("trace=%s", result.stderr)
	}
}

func TestBlackBoxFailureTraceNamesCommandPhase(t *testing.T) {
	cases := [][]string{{"reduce", "--as", "x", "--input", "ndjson", "--questions-json", `{"questions":{"x":{"type":"noul","instructions":"x"}}}`}, {"rank", "--as", "x", "--input", "json", "--state", "s", "--instruction", "x", "--id-pointer", "/id", "--criteria-pointer", "/criteria"}, {"gate", "--as", "x", "--input", "ndjson", "--value-pointer", "/missing", "--pass-min", "0.8", "--reject-max", "0.2"}}
	for _, args := range cases {
		result := runBinary(t, "not-json\n", map[string]string{"JEQ_TRACE_ID": "failure"}, append([]string{"--verbose"}, args...)...)
		if result.exit == 0 || !strings.Contains(result.stderr, `"event":"run.failed"`) || !strings.Contains(result.stderr, `"phase":"input_validation"`) {
			t.Fatalf("args=%v exit=%d stderr=%s", args, result.exit, result.stderr)
		}
	}
}
