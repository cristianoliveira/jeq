package render_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/cristianoliveira/gev/internal/domain/contract"
	"github.com/cristianoliveira/gev/internal/domain/gev"
	"github.com/cristianoliveira/gev/internal/fixtures"
	"github.com/cristianoliveira/gev/internal/infra/render"
)

// Regenerate goldens with: nix develop -c go test ./internal/infra/render -update
var updateGolden = flag.Bool("update", false, "regenerate golden fixtures")

func mustDecodeResponse(t *testing.T, name string) contract.Response {
	t.Helper()
	resp, err := contract.DecodeResponse(fixtures.MustContract(t, name))
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return resp
}

func TestRenderSuccessGolden(t *testing.T) {
	resp := mustDecodeResponse(t, "response_200_full.json")

	var out bytes.Buffer
	if err := (render.JSON{}).RenderSuccess(&out, resp); err != nil {
		t.Fatal(err)
	}
	assertGolden(t, "json_200_full.golden", out.Bytes())
}

func TestRenderSuccessIsDeterministic(t *testing.T) {
	resp := mustDecodeResponse(t, "response_200_full.json")

	var one, two bytes.Buffer
	if err := (render.JSON{}).RenderSuccess(&one, resp); err != nil {
		t.Fatal(err)
	}
	if err := (render.JSON{}).RenderSuccess(&two, resp); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(one.Bytes(), two.Bytes()) {
		t.Error("success render is not deterministic")
	}
}

func TestRenderSuccessOneDocumentOneNewline(t *testing.T) {
	resp := mustDecodeResponse(t, "response_200_full.json")

	var out bytes.Buffer
	if err := (render.JSON{}).RenderSuccess(&out, resp); err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(out.String(), "}\n") {
		t.Errorf("output must end with exactly one trailing newline: %q", out.String()[max(0, len(out.String())-5):])
	}
	if strings.Count(out.String(), "\n") != 1 {
		t.Error("success output must be exactly one line")
	}
}

func TestRenderSuccessRoundTrips(t *testing.T) {
	resp := mustDecodeResponse(t, "response_200_full.json")

	var out bytes.Buffer
	if err := (render.JSON{}).RenderSuccess(&out, resp); err != nil {
		t.Fatal(err)
	}
	resp2, err := contract.DecodeResponse(out.Bytes())
	if err != nil {
		t.Fatalf("rendered output does not decode: %v", err)
	}
	if !reflect.DeepEqual(resp, resp2) {
		t.Error("render output does not round-trip to a semantically identical value")
	}
}

func TestRenderErrorExposesOnlyCodeMessageRecovery(t *testing.T) {
	// Given an error whose internal cause carries a secret
	err := gev.WrapError(gev.CodeRateLimited, errors.New("cause with supersecret-material"), "slow down").
		WithRecovery("retry after a delay")

	var out bytes.Buffer
	if err := (render.JSON{}).RenderError(&out, err); err != nil {
		t.Fatal(err)
	}

	// Then the document carries only code/message/recovery
	var doc map[string]any
	if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
		t.Fatalf("error output is not valid JSON: %v", err)
	}
	if len(doc) != 3 {
		t.Fatalf("error document must expose exactly code/message/recovery, got %v", doc)
	}
	for _, key := range []string{"code", "message", "recovery"} {
		if _, ok := doc[key]; !ok {
			t.Errorf("error document missing %q", key)
		}
	}

	// And internal cause text never leaks
	if strings.Contains(out.String(), "supersecret") {
		t.Errorf("internal cause leaked into the error document: %q", out.String())
	}
	if !strings.Contains(out.String(), "GEV_RATE_LIMITED") {
		t.Errorf("error document missing the stable code: %q", out.String())
	}
	if !strings.Contains(out.String(), "retry after a delay") {
		t.Errorf("error document missing the recovery instruction: %q", out.String())
	}
}

func TestRenderErrorGolden(t *testing.T) {
	err := gev.NewError(gev.CodeAuthMissing, "TYPESAFE_API_KEY is not set").
		WithRecovery("export TYPESAFE_API_KEY with the account key")

	var out bytes.Buffer
	if err := (render.JSON{}).RenderError(&out, err); err != nil {
		t.Fatal(err)
	}
	assertGolden(t, "json_error_auth_missing.golden", out.Bytes())
}

func TestRenderEscapingStaysOneLine(t *testing.T) {
	err := gev.NewError(gev.CodeInputInvalid, "bad value \"quoted\" <tag>\nsecond line").
		WithRecovery("fix \"it\"")

	var out bytes.Buffer
	if err := (render.JSON{}).RenderError(&out, err); err != nil {
		t.Fatal(err)
	}
	if strings.Count(out.String(), "\n") != 1 {
		t.Errorf("error output must be one line, got: %q", out.String())
	}
	var doc map[string]any
	if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
		t.Errorf("escaped output is not valid JSON: %v", err)
	}
}

func TestRenderResponseWithEmptyOptionals(t *testing.T) {
	// A minimal noul-only response must render and round-trip cleanly.
	raw := []byte(`{"model":"jev-1.13.0","answers":{"q":{"type":"noul","noul":0}},"usage":{"input_tokens":0,"output_tokens":0}}`)
	resp, err := contract.DecodeResponse(raw)
	if err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if err := (render.JSON{}).RenderSuccess(&out, resp); err != nil {
		t.Fatal(err)
	}
	resp2, err := contract.DecodeResponse(out.Bytes())
	if err != nil || !reflect.DeepEqual(resp, resp2) {
		t.Errorf("minimal response did not round-trip: %v", err)
	}
}

func assertGolden(t *testing.T, name string, got []byte) {
	t.Helper()
	path := filepath.Join("..", "..", "fixtures", "render", name)
	if *updateGolden {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	golden, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading golden %s (run with -update): %v", name, err)
	}
	if !bytes.Equal(got, golden) {
		t.Errorf("rendered output drifted from golden %s\nwant:\n%s\ngot:\n%s", name, golden, got)
	}
}
