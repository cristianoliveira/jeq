package blackbox_test

import (
	"strings"
	"testing"
)

func TestBlackBoxVerboseGatePolicySummaryCountsAndExit(t *testing.T) {
	cases := []struct {
		name    string
		value   float64
		exit    int
		summary string
	}{{"pass", 0.9, 0, `"pass":1,"ambiguous":0,"reject":0`}, {"ambiguous", 0.5, 11, `"pass":0,"ambiguous":1,"reject":0`}, {"reject", 0.1, 10, `"pass":0,"ambiguous":0,"reject":1`}}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := runBinary(t, `{"score":`+formatFloat(tc.value)+"}\n", nil, "--verbose", "gate", "--as", "policy", "--input", "ndjson", "--value-pointer", "/score", "--pass-min", "0.8", "--reject-max", "0.2")
			if result.exit != tc.exit || !strings.Contains(result.stderr, `"phase":"offline_policy"`) || !strings.Contains(result.stderr, tc.summary) {
				t.Fatalf("result=%#v", result)
			}
		})
	}
}

func formatFloat(value float64) string {
	if value == 0.9 {
		return "0.9"
	}
	if value == 0.5 {
		return "0.5"
	}
	return "0.1"
}
