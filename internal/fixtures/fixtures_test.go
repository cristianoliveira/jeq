// Package fixtures_test keeps the loading helpers honest.
package fixtures_test

import (
	"strings"
	"testing"

	"github.com/cristianoliveira/jeq/internal/fixtures"
)

func TestContractFixturesLoad(t *testing.T) {
	names, err := fixtures.ContractFixtures()
	if err != nil {
		t.Fatal(err)
	}
	if len(names) < 10 {
		t.Fatalf("expected the contract corpus, got %d files: %v", len(names), names)
	}
}

func TestInvalidManifestParses(t *testing.T) {
	manifest, err := fixtures.InvalidManifest()
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range manifest {
		if !strings.HasPrefix(entry.Code, "JEQ_") {
			t.Errorf("fixture %s: code %q is not a jeq code", entry.Fixture, entry.Code)
		}
		if entry.Rule == "" {
			t.Errorf("fixture %s: missing rule", entry.Fixture)
		}
	}
}
