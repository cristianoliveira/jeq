// Package fixtures loads the sanitized shared fixtures under
// internal/fixtures (ADR 0002). Files are embedded so tests never depend
// on the working directory.
package fixtures

import (
	"embed"
	"encoding/json"
	"fmt"
)

// TB is the subset of testing.TB the helpers need, kept local so this
// package never imports testing outside tests.
type TB interface {
	Fatalf(format string, args ...any)
}

//go:embed contract
var files embed.FS

// Contract returns the raw bytes of a fixture from the contract/ directory.
func Contract(name string) ([]byte, error) {
	return files.ReadFile("contract/" + name)
}

// MustContract is Contract for tests: it fails the test on any error.
func MustContract(tb TB, name string) []byte {
	data, err := Contract(name)
	if err != nil {
		tb.Fatalf("loading fixture %s: %v", name, err)
	}
	return data
}

// ContractFixtures lists every file name in the contract/ directory.
func ContractFixtures() ([]string, error) {
	entries, err := files.ReadDir("contract")
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names, nil
}

// InvalidFixture declares, in invalid_manifest.json, one failing fixture and
// the local rule plus stable code it must produce.
type InvalidFixture struct {
	Fixture string `json:"fixture"`
	Rule    string `json:"rule"`
	Code    string `json:"code"`
}

// InvalidManifest parses invalid_manifest.json.
func InvalidManifest() ([]InvalidFixture, error) {
	raw, err := Contract("invalid_manifest.json")
	if err != nil {
		return nil, err
	}
	var manifest []InvalidFixture
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return nil, fmt.Errorf("parsing invalid_manifest.json: %w", err)
	}
	return manifest, nil
}
