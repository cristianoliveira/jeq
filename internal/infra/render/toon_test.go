package render_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/cristianoliveira/gev/internal/domain/contract"
	"github.com/cristianoliveira/gev/internal/domain/gev"
	"github.com/cristianoliveira/gev/internal/fixtures"
	"github.com/cristianoliveira/gev/internal/infra/render"
	toon "github.com/toon-format/toon-go"
)

func TestTOONResponseRoundTripCorpus(t *testing.T) {
	for _, fixture := range []string{"response_200_full.json", "response_unknown_fields.json"} {
		t.Run(fixture, func(t *testing.T) {
			response, err := contract.DecodeResponse(fixtures.MustContract(t, fixture))
			if err != nil {
				t.Fatal(err)
			}
			var first, second bytes.Buffer
			if err := (render.TOON{}).RenderSuccess(&first, response); err != nil {
				t.Fatal(err)
			}
			if err := (render.TOON{}).RenderSuccess(&second, response); err != nil {
				t.Fatal(err)
			}
			if first.String() != second.String() {
				t.Fatal("TOON output is not deterministic")
			}
			if !strings.HasSuffix(first.String(), "\n") || strings.Count(first.String(), "\n") == 0 {
				t.Fatalf("missing trailing newline: %q", first.String())
			}
			var want, got any
			dec := json.NewDecoder(strings.NewReader(string(fixtures.MustContract(t, fixture))))
			dec.UseNumber()
			if err := dec.Decode(&want); err != nil {
				t.Fatal(err)
			}
			if err := toon.Unmarshal([]byte(strings.TrimSuffix(first.String(), "\n")), &got); err != nil {
				t.Fatal(err)
			}
			if !semanticJSONEqual(want, got) {
				t.Fatalf("semantic round trip mismatch\nwant=%#v\ngot=%#v", want, got)
			}
		})
	}
}

func TestTOONErrorDocumentsRoundTrip(t *testing.T) {
	for _, code := range gev.Codes() {
		var out bytes.Buffer
		err := gev.NewError(code, "message with Unicode ✓, comma, colon: and quotes \"x\"").WithRecovery("recover: retry or correct input")
		if renderErr := (render.TOON{}).RenderError(&out, err); renderErr != nil {
			t.Fatalf("%s: %v", code, renderErr)
		}
		if !strings.HasSuffix(out.String(), "\n") {
			t.Errorf("%s lacks trailing newline", code)
		}
		var decoded map[string]any
		if decodeErr := toon.Unmarshal([]byte(strings.TrimSuffix(out.String(), "\n")), &decoded); decodeErr != nil {
			t.Fatalf("%s: %v", code, decodeErr)
		}
		if decoded["code"] != string(code) || decoded["recovery"] == nil {
			t.Errorf("decoded=%#v", decoded)
		}
	}
}

func TestTOONRejectsUnsafeNumericPrecision(t *testing.T) {
	// The candidate must fail loudly rather than silently rounding a value
	// outside IEEE-754's exact integer range.
	response := contract.Response{
		Model: "m", Answers: map[string]contract.Answer{}, Usage: contract.Usage{},
		Extra: map[string]json.RawMessage{"n": json.RawMessage(`9007199254740993`)},
	}
	var out bytes.Buffer
	err := (render.TOON{}).RenderSuccess(&out, response)
	if err == nil || !strings.Contains(err.Error(), "semantics") {
		t.Fatalf("error = %v, want explicit semantic-loss failure", err)
	}
	if out.Len() != 0 {
		t.Fatalf("partial TOON output leaked: %q", out.String())
	}
}

func semanticJSONEqual(a, b any) bool {
	ab, _ := json.Marshal(a)
	bb, _ := json.Marshal(b)
	var x, y any
	_ = json.Unmarshal(ab, &x)
	_ = json.Unmarshal(bb, &y)
	return string(mustJSON(x)) == string(mustJSON(y))
}
func mustJSON(v any) []byte { b, _ := json.Marshal(v); return b }
