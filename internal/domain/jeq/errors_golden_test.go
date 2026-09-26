package jeq_test

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cristianoliveira/jeq/internal/domain/jeq"
)

// Regenerate with: nix develop -c go test ./internal/domain/jeq -run Golden -update
var updateGolden = flag.Bool("update", false, "regenerate golden fixtures")

// TestErrorCodesGoldenSnapshot fails on any registry add, remove, or rename.
// The snapshot is the reviewed contract; changes must be intentional.
func TestErrorCodesGoldenSnapshot(t *testing.T) {
	golden := filepath.Join("..", "..", "fixtures", "contract", "error_codes.golden")

	var b strings.Builder
	for _, code := range jeq.Codes() {
		b.WriteString(string(code))
		b.WriteString("\n")
	}

	if *updateGolden {
		require.NoError(t, os.MkdirAll(filepath.Dir(golden), 0o755))
		require.NoError(t, os.WriteFile(golden, []byte(b.String()), 0o644))
	}

	want, err := os.ReadFile(golden)
	require.NoError(t, err, "reading golden fixture (run with -update to regenerate)")
	assert.Equal(t, string(want), b.String(), "error code registry drifted from the golden snapshot")
}
