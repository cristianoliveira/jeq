package blackbox_test

import (
	"strings"
	"testing"
)

func TestBlackBoxVerboseStateSelectionFailuresAreTyped(t *testing.T) {
	cases := [][]string{{"map", "--as", "q", "--input", "ndjson", "--state-pointer", "/missing", "--questions-json", `{"questions":{"q":{"type":"noul","instructions":"private prompt"}}}`}, {"rank", "--as", "q", "--input", "json", "--state", "private state", "--instruction", "private prompt", "--id-pointer", "/candidate-id", "--criteria-pointer", "/missing"}}
	for _, args := range cases {
		input := `{"candidate-id":"secret-candidate","state":"secret-state"}` + "\n"
		if args[0] == "rank" {
			input = `[{"candidate-id":"secret-candidate","criteria":"private criteria"}]`
		}
		result := runBinary(t, input, map[string]string{"JEQ_TRACE_ID": "state", "TYPESAFE_API_KEY": "secret-key"}, append([]string{"--verbose"}, args...)...)
		if result.exit == 0 || !strings.Contains(result.stderr, `"phase":"state_selection"`) || strings.Contains(result.stderr, "secret") || strings.Contains(result.stderr, "private") {
			t.Fatalf("args=%v stderr=%s", args, result.stderr)
		}
	}
}
