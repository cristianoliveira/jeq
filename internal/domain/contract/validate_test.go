package contract_test

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/cristianoliveira/gev/internal/domain/contract"
	"github.com/cristianoliveira/gev/internal/fixtures"
)

func TestValidateLocalRules(t *testing.T) {
	// Every failing fixture must fail locally with exactly the rule and
	// stable code declared in invalid_manifest.json.
	manifest, err := fixtures.InvalidManifest()
	if err != nil {
		t.Fatal(err)
	}

	for _, entry := range manifest {
		t.Run(entry.Fixture, func(t *testing.T) {
			raw := fixtures.MustContract(t, entry.Fixture)

			violations := contract.Validate(raw)
			if len(violations) == 0 {
				t.Fatalf("expected a violation of rule %q, got none", entry.Rule)
			}

			for _, v := range violations {
				if v.Rule == entry.Rule {
					if string(v.Error.Code) != entry.Code {
						t.Errorf("rule %q produced code %q, want %q", entry.Rule, v.Error.Code, entry.Code)
					}
					return
				}
			}
			t.Errorf("expected a violation of rule %q, got rules %v", entry.Rule, ruleNames(violations))
		})
	}
}

func TestValidateAcceptsValidDocuments(t *testing.T) {
	for _, name := range []string{"request_full.json", "unknown_field.json"} {
		t.Run(name, func(t *testing.T) {
			raw := fixtures.MustContract(t, name)
			if violations := contract.Validate(raw); len(violations) != 0 {
				t.Errorf("valid document rejected: %v", ruleNames(violations))
			}
		})
	}
}

func TestValidateEveryRuleHasRecoveryText(t *testing.T) {
	for _, rule := range contract.Rules() {
		if rule.Recovery == "" {
			t.Errorf("rule %q has no recovery instruction", rule.Name)
		}
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
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range names {
		raw, err := fixtures.Contract(name)
		if err != nil {
			t.Fatal(err)
		}
		_ = contract.Validate(raw)
		_, _ = contract.DecodeRequest(raw)
	}

	if hits.Load() != 0 {
		t.Errorf("validation made %d network requests; the local gate must run pre-network", hits.Load())
	}
}

func ruleNames(vs []contract.Violation) []string {
	names := make([]string, 0, len(vs))
	for _, v := range vs {
		names = append(names, v.Rule)
	}
	return names
}
