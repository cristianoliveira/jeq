// Package fixtures_test keeps the loading helpers honest.
package fixtures_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cristianoliveira/jeq/internal/fixtures"
)

func TestContractFixturesLoad(t *testing.T) {
	names, err := fixtures.ContractFixtures()
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(names), 10, "expected the contract corpus, got %d files: %v", len(names), names)
}

func TestInvalidManifestParses(t *testing.T) {
	manifest, err := fixtures.InvalidManifest()
	require.NoError(t, err)
	for _, entry := range manifest {
		assert.True(t, strings.HasPrefix(entry.Code, "JEQ_"), "fixture %s: code %q is not a jeq code", entry.Fixture, entry.Code)
		assert.NotEmpty(t, entry.Rule, "fixture %s: missing rule", entry.Fixture)
	}
}
