package blackbox_test

import (
	"strings"
	"testing"
)

func TestBlackBoxHelpOmitsRemovedInfrastructureFlags(t *testing.T) {
	cases := []struct {
		command string
		flags   []string
	}{{"ask", []string{"--base-url"}}, {"models", []string{"--base-url"}}, {"map", []string{"--base-url", "--config"}}, {"reduce", []string{"--base-url", "--config"}}, {"rank", []string{"--base-url", "--config"}}, {"rate", []string{"--base-url", "--config"}}}
	for _, tc := range cases {
		t.Run(tc.command, func(t *testing.T) {
			result := runBinary(t, "", nil, tc.command, "--help")
			if result.exit != 0 {
				t.Fatalf("exit=%d stderr=%q", result.exit, result.stderr)
			}
			for _, flag := range tc.flags {
				if strings.Contains(result.stdout, flag) {
					t.Fatalf("help contains removed flag %q: %s", flag, result.stdout)
				}
			}
		})
	}
}
