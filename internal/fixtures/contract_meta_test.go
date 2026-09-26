package fixtures_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cristianoliveira/jeq/internal/domain/contract"
	"github.com/cristianoliveira/jeq/internal/domain/jeq"
	"github.com/cristianoliveira/jeq/internal/fixtures"
)

// TestFixtureCodeCoverage is the D1-14 meta-test: every failing fixture in
// the contract corpus declares a rule and a stable code from the D0
// registry, and every non-declared fixture document must validate clean.
func TestFixtureCodeCoverage(t *testing.T) {
	manifest, err := fixtures.InvalidManifest()
	require.NoError(t, err)

	registered := map[string]bool{}
	for _, code := range jeq.Codes() {
		registered[string(code)] = true
	}

	declared := map[string]string{} // fixture -> code
	for _, entry := range manifest {
		if !assert.True(t, registered[entry.Code], "fixture %s: code %q is not in the D0 registry", entry.Fixture, entry.Code) {
			continue
		}
		prev, dup := declared[entry.Fixture]
		assert.False(t, dup, "fixture %s declared twice (%s, %s)", entry.Fixture, prev, entry.Code)
		declared[entry.Fixture] = entry.Code
	}

	names, err := fixtures.ContractFixtures()
	require.NoError(t, err)
	// Non-request documents (server responses, model lists) are not inputs
	// to the local request gate.
	nonRequest := map[string]bool{
		"models.json":                  true,
		"response_200_full.json":       true,
		"response_unknown_fields.json": true,
	}
	for _, name := range names {
		switch name {
		case "invalid_manifest.json", "error_codes.golden":
			continue
		}
		if nonRequest[name] {
			continue
		}
		code, isFailure := declared[name]
		if !isFailure {
			// Valid documents must stay valid; otherwise the corpus lies.
			raw, err := fixtures.Contract(name)
			require.NoError(t, err)
			vs := contract.Validate(raw)
			assert.Empty(t, vs, "fixture %s is not declared as failing but violates a rule", name)
			continue
		}
		assert.NotEmpty(t, code, "fixture %s declares no stable code", name)
	}
}
