package gev_test

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianoliveira/gev/internal/domain/gev"
)

// Regenerate with: nix develop -c go test ./internal/domain/gev -run Golden -update
var updateGolden = flag.Bool("update", false, "regenerate golden fixtures")

// TestErrorCodesGoldenSnapshot fails on any registry add, remove, or rename.
// The snapshot is the reviewed contract; changes must be intentional.
func TestErrorCodesGoldenSnapshot(t *testing.T) {
	golden := filepath.Join("..", "..", "fixtures", "contract", "error_codes.golden")

	var b strings.Builder
	for _, code := range gev.Codes() {
		b.WriteString(string(code))
		b.WriteString("\n")
	}

	if *updateGolden {
		if err := os.MkdirAll(filepath.Dir(golden), 0o755); err != nil {
			t.Fatalf("creating fixtures dir: %v", err)
		}
		if err := os.WriteFile(golden, []byte(b.String()), 0o644); err != nil {
			t.Fatalf("writing golden fixture: %v", err)
		}
	}

	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("reading golden fixture (run with -update to regenerate): %v", err)
	}

	if b.String() != string(want) {
		t.Errorf("error code registry drifted from the golden snapshot.\nwant:\n%s\ngot:\n%s", want, b.String())
	}
}
