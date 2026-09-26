package render_test

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cristianoliveira/jeq/internal/domain/contract"
	"github.com/cristianoliveira/jeq/internal/fixtures"
	"github.com/cristianoliveira/jeq/internal/infra/render"
)

// Regenerate goldens with: nix develop -c go test ./internal/infra/render -update
var updateGolden = flag.Bool("update", false, "regenerate golden fixtures")

func mustDecodeResponse(t *testing.T, name string) contract.Response {
	t.Helper()
	resp, err := contract.DecodeResponse(fixtures.MustContract(t, name))
	require.Nil(t, err, "%s", name)
	return resp
}

func TestRenderSuccessGolden(t *testing.T) {
	resp := mustDecodeResponse(t, "response_200_full.json")

	var out bytes.Buffer
	require.NoError(t, (render.JSON{}).RenderSuccess(&out, resp))
	assertGolden(t, "json_200_full.golden", out.Bytes())
}

func TestRenderSuccessIsDeterministic(t *testing.T) {
	resp := mustDecodeResponse(t, "response_200_full.json")

	var one, two bytes.Buffer
	require.NoError(t, (render.JSON{}).RenderSuccess(&one, resp))
	require.NoError(t, (render.JSON{}).RenderSuccess(&two, resp))
	assert.Equal(t, one.Bytes(), two.Bytes(), "success render must be deterministic")
}

func TestRenderSuccessOneDocumentOneNewline(t *testing.T) {
	resp := mustDecodeResponse(t, "response_200_full.json")

	var out bytes.Buffer
	require.NoError(t, (render.JSON{}).RenderSuccess(&out, resp))
	assert.True(t, strings.HasSuffix(out.String(), "}\n"), "output must end with exactly one trailing newline: %q", out.String()[max(0, len(out.String())-5):])
	assert.Equal(t, 1, strings.Count(out.String(), "\n"), "success output must be exactly one line")
}

func TestRenderSuccessRoundTrips(t *testing.T) {
	resp := mustDecodeResponse(t, "response_200_full.json")

	var out bytes.Buffer
	require.NoError(t, (render.JSON{}).RenderSuccess(&out, resp))
	resp2, err := contract.DecodeResponse(out.Bytes())
	require.Nil(t, err, "rendered output must decode")
	assert.True(t, reflect.DeepEqual(resp, resp2), "render output must round-trip to a semantically identical value")
}

func TestRenderResponseWithEmptyOptionals(t *testing.T) {
	// A minimal noul-only response must render and round-trip cleanly.
	raw := []byte(`{"model":"jev-1.13.0","answers":{"q":{"type":"noul","noul":0}},"usage":{"input_tokens":0,"output_tokens":0}}`)
	resp, err := contract.DecodeResponse(raw)
	require.Nil(t, err)

	var out bytes.Buffer
	require.NoError(t, (render.JSON{}).RenderSuccess(&out, resp))
	resp2, err := contract.DecodeResponse(out.Bytes())
	require.Nil(t, err, "minimal response must decode")
	assert.True(t, reflect.DeepEqual(resp, resp2), "minimal response must round-trip")
}

func assertGolden(t *testing.T, name string, got []byte) {
	t.Helper()
	path := filepath.Join("..", "..", "fixtures", "render", name)
	if *updateGolden {
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, got, 0o644))
	}
	golden, err := os.ReadFile(path)
	require.NoError(t, err, "reading golden %s (run with -update)", name)
	assert.Equal(t, golden, got, "rendered output drifted from golden %s", name)
}
