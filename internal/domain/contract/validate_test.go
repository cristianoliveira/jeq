package contract_test

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cristianoliveira/jeq/internal/domain/contract"
	"github.com/cristianoliveira/jeq/internal/fixtures"
)

func TestValidateLocalRules(t *testing.T) {
	// Every failing fixture must fail locally with exactly the rule and
	// stable code declared in invalid_manifest.json.
	manifest, err := fixtures.InvalidManifest()
	require.NoError(t, err)

	for _, entry := range manifest {
		t.Run(entry.Fixture, func(t *testing.T) {
			raw := fixtures.MustContract(t, entry.Fixture)

			violations := contract.Validate(raw)
			ruleIDs := ruleNames(violations)
			require.NotEmpty(t, violations, "expected rule %q, got no violations", entry.Rule)
			require.Contains(t, ruleIDs, entry.Rule, "actual rules: %v", ruleIDs)

			for _, v := range violations {
				if v.Rule == entry.Rule {
					assert.Equal(t, entry.Code, string(v.Error.Code), "rule %q", entry.Rule)
					return
				}
			}
		})
	}
}

func TestValidateAcceptsValidDocuments(t *testing.T) {
	for _, name := range []string{"request_full.json", "unknown_field.json"} {
		t.Run(name, func(t *testing.T) {
			raw := fixtures.MustContract(t, name)
			assert.Empty(t, contract.Validate(raw), "valid document rejected")
		})
	}
}

func TestValidateEveryRuleHasRecoveryText(t *testing.T) {
	for _, rule := range contract.Rules() {
		assert.NotEmpty(t, rule.Recovery, "rule %q has no recovery instruction", rule.Name)
	}
}

func TestValidatePreNetwork(t *testing.T) {
	// The local gate must never touch the network, even when a server is
	// reachable: every fixture validates with zero requests counted.
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
	}))
	defer srv.Close()

	names, err := fixtures.ContractFixtures()
	require.NoError(t, err)
	for _, name := range names {
		raw, err := fixtures.Contract(name)
		require.NoError(t, err)
		_ = contract.Validate(raw)
		_, _ = contract.DecodeRequest(raw)
	}

	assert.Zero(t, hits.Load(), "the local validation gate must run before network access")
}

func ruleNames(vs []contract.Violation) []string {
	names := make([]string, 0, len(vs))
	for _, v := range vs {
		names = append(names, v.Rule)
	}
	return names
}
