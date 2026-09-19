package fixtures_test

import (
	"testing"

	"github.com/cristianoliveira/gev/internal/domain/contract"
	"github.com/cristianoliveira/gev/internal/domain/gev"
	"github.com/cristianoliveira/gev/internal/fixtures"
)

// TestFixtureCodeCoverage is the D1-14 meta-test: every failing fixture in
// the contract corpus declares a rule and a stable code from the D0
// registry, and every non-declared fixture document must validate clean.
func TestFixtureCodeCoverage(t *testing.T) {
	manifest, err := fixtures.InvalidManifest()
	if err != nil {
		t.Fatal(err)
	}

	registered := map[string]bool{}
	for _, code := range gev.Codes() {
		registered[string(code)] = true
	}

	declared := map[string]string{} // fixture -> code
	for _, entry := range manifest {
		if !registered[entry.Code] {
			t.Errorf("fixture %s: code %q is not in the D0 registry", entry.Fixture, entry.Code)
			continue
		}
		if prev, dup := declared[entry.Fixture]; dup {
			t.Errorf("fixture %s declared twice (%s, %s)", entry.Fixture, prev, entry.Code)
		}
		declared[entry.Fixture] = entry.Code
	}

	names, err := fixtures.ContractFixtures()
	if err != nil {
		t.Fatal(err)
	}
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
			if err != nil {
				t.Fatal(err)
			}
			if vs := contract.Validate(raw); len(vs) != 0 {
				t.Errorf("fixture %s is not declared as failing but violates: %v", name, vs[0].Rule)
			}
			continue
		}
		if code == "" {
			t.Errorf("fixture %s declares no stable code", name)
		}
	}
}
