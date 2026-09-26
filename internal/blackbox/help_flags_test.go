package blackbox_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBlackBoxHelpOmitsRemovedInfrastructureFlags(t *testing.T) {
	cases := []struct {
		command string
		flags   []string
	}{{"ask", []string{"--base-url"}}, {"models", []string{"--base-url"}}, {"map", []string{"--base-url", "--config"}}, {"reduce", []string{"--base-url", "--config"}}, {"rank", []string{"--base-url", "--config"}}, {"rate", []string{"--base-url", "--config"}}}
	for _, tc := range cases {
		t.Run(tc.command, func(t *testing.T) {
			result := runBinary(t, "", nil, tc.command, "--help")
			require.Equal(t, 0, result.exit)
			for _, flag := range tc.flags {
				assert.NotContains(t, result.stdout, flag)
			}
		})
	}
}
