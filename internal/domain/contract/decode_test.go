package contract_test

import (
	"encoding/json"
	"math/rand"
	"reflect"
	"strings"
	"testing"
	"testing/quick"

	"github.com/cristianoliveira/gev/internal/domain/contract"
	"github.com/cristianoliveira/gev/internal/domain/gev"
	"github.com/cristianoliveira/gev/internal/fixtures"
)

func TestDecodeRequestStrict(t *testing.T) {
	tests := []struct {
		name     string
		doc      string
		wantCode gev.Code
	}{
		{
			name:     "truncated document is rejected",
			doc:      `{"state":"a","model"`,
			wantCode: gev.CodeRequestInvalid,
		},
		{
			name:     "trailing comma is rejected",
			doc:      `{"state":"a","model":"m","questions":{},}`,
			wantCode: gev.CodeRequestInvalid,
		},
		{
			name:     "duplicate top-level key is rejected, not last-wins",
			doc:      `{"state":"a","state":"b","model":"m","questions":{"q":{"type":"noul","instructions":"i"}}}`,
			wantCode: gev.CodeRequestInvalid,
		},
		{
			name:     "duplicate nested key is rejected",
			doc:      `{"state":"a","model":"m","questions":{"q":{"type":"noul","instructions":"i","instructions":"j"}}}`,
			wantCode: gev.CodeRequestInvalid,
		},
		{
			name:     "UTF-8 BOM is rejected",
			doc:      "\xEF\xBB\xBF{\"state\":\"a\"}",
			wantCode: gev.CodeRequestInvalid,
		},
		{
			name:     "invalid UTF-8 is rejected",
			doc:      "{\"state\":\"\xff\xfe\"}",
			wantCode: gev.CodeRequestInvalid,
		},
		{
			name:     "known-field type mismatch",
			doc:      `{"state":"s","model":"m","questions":{"f":{"type":"score","instructions":"rate it","criteria":"Calm"}}}`,
			wantCode: gev.CodeRequestInvalid,
		},
		{
			name:     "model as number is a type mismatch",
			doc:      `{"state":"s","model":7,"questions":{"q":{"type":"noul","instructions":"i"}}}`,
			wantCode: gev.CodeRequestInvalid,
		},
		{
			name:     "questions as array is a type mismatch",
			doc:      `{"state":"s","model":"m","questions":[]}`,
			wantCode: gev.CodeRequestInvalid,
		},
		{
			name:     "trailing data after the document is rejected",
			doc:      `{"state":"a","model":"m","questions":{}} {"x":1}`,
			wantCode: gev.CodeRequestInvalid,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := contract.DecodeRequest([]byte(tt.doc))
			if err == nil {
				t.Fatal("expected decode error, got nil")
			}
			if err.Code != tt.wantCode {
				t.Errorf("code = %q, want %q (message: %s)", err.Code, tt.wantCode, err.Message)
			}
		})
	}
}

func TestDecodeRequestTypeMismatchNamesFieldPath(t *testing.T) {
	_, err := contract.DecodeRequest([]byte(`{"state":"s","model":"m","questions":{"f":{"type":"score","instructions":"rate it","criteria":"Calm"}}}`))
	if err == nil {
		t.Fatal("expected decode error")
	}
	if !strings.Contains(err.Message, "questions.f.criteria") {
		t.Errorf("message %q does not name the field path questions.f.criteria", err.Message)
	}
}

func TestDecodeRequestHappyPath(t *testing.T) {
	raw := fixtures.MustContract(t, "request_full.json")
	req, err := contract.DecodeRequest(raw)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if req.Model != "jev-latest" {
		t.Errorf("model = %q", req.Model)
	}
	if len(req.Questions) != 3 {
		t.Fatalf("questions = %d, want 3", len(req.Questions))
	}
	if req.Questions["frustration"].Type != contract.TypeScore {
		t.Errorf("frustration type = %q", req.Questions["frustration"].Type)
	}
}

func TestUnknownFieldPassthrough(t *testing.T) {
	// Given a request with unknown fields at the top level and inside questions
	raw := fixtures.MustContract(t, "unknown_field.json")

	// When it decodes and re-encodes
	req, decErr := contract.DecodeRequest(raw)
	if decErr != nil {
		t.Fatalf("unknown fields must never be rejected: %v", decErr)
	}
	out, encErr := req.Encode()
	if encErr != nil {
		t.Fatal(encErr)
	}

	// Then the unknown fields survive byte-semantically
	var round map[string]any
	if err := json.Unmarshal(out, &round); err != nil {
		t.Fatal(err)
	}
	if _, ok := round["x_extra"]; !ok {
		t.Error("top-level unknown field x_extra was dropped")
	}
	questions, _ := round["questions"].(map[string]any)
	q, _ := questions["is_urgent"].(map[string]any)
	if _, ok := q["vendor_meta"]; !ok {
		t.Error("unknown question field vendor_meta was dropped")
	}

	// And re-decoding the output is an idempotent fixpoint
	req2, decErr := contract.DecodeRequest(out)
	if decErr != nil {
		t.Fatalf("re-decode failed: %v", decErr)
	}
	out2, _ := req2.Encode()
	req3, _ := contract.DecodeRequest(out2)
	if !reflect.DeepEqual(req2, req3) {
		t.Error("decode→encode is not idempotent")
	}
}

func TestLosslessResponse(t *testing.T) {
	raw := fixtures.MustContract(t, "response_unknown_fields.json")

	resp, err := contract.DecodeResponse(raw)
	if err != nil {
		t.Fatalf("response decode failed: %v", err)
	}

	out, encErr := resp.Encode()
	if encErr != nil {
		t.Fatal(encErr)
	}

	var round map[string]any
	if err := json.Unmarshal(out, &round); err != nil {
		t.Fatal(err)
	}
	if _, ok := round["x_served_by"]; !ok {
		t.Error("unknown top-level response field x_served_by was dropped")
	}
	answers, _ := round["answers"].(map[string]any)
	dept, _ := answers["department"].(map[string]any)
	if _, ok := dept["x_rank"]; !ok {
		t.Error("unknown answer field x_rank was dropped")
	}
}

func TestDecodeResponseMissingKnownFieldIsResponseInvalid(t *testing.T) {
	tests := []struct {
		name string
		doc  string
	}{
		{"missing model", `{"answers":{},"usage":{"input_tokens":1,"output_tokens":2}}`},
		{"missing answers", `{"model":"m","usage":{"input_tokens":1,"output_tokens":2}}`},
		{"missing usage", `{"model":"m","answers":{}}`},
		{"answer missing type", `{"model":"m","answers":{"q":{}},"usage":{"input_tokens":1,"output_tokens":2}}`},
		{"probability as string", `{"model":"m","answers":{"q":{"type":"noul","noul":"0.9"}},"usage":{"input_tokens":1,"output_tokens":2}}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := contract.DecodeResponse([]byte(tt.doc))
			if err == nil || err.Code != gev.CodeResponseInvalid {
				t.Errorf("expected %q, got %v", gev.CodeResponseInvalid, err)
			}
		})
	}
}

func TestDecodeResponseHappyPath(t *testing.T) {
	raw := fixtures.MustContract(t, "response_unknown_fields.json")
	resp, err := contract.DecodeResponse(raw)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if resp.Model != "jev-1.13.0" {
		t.Errorf("model = %q", resp.Model)
	}
	if len(resp.Answers) != 3 {
		t.Errorf("answers = %d, want 3", len(resp.Answers))
	}
	if resp.Usage.InputTokens != 312 || resp.Usage.OutputTokens != 48 {
		t.Errorf("usage = %d/%d", resp.Usage.InputTokens, resp.Usage.OutputTokens)
	}
	if got := resp.Answers["is_urgent"].Noul; got == nil || *got != 0.92 {
		t.Errorf("noul answer = %v", resp.Answers["is_urgent"].Noul)
	}
	if got := resp.Answers["department"].Confidence; got == nil || *got != 0.82 {
		t.Errorf("choice confidence = %v", resp.Answers["department"].Confidence)
	}
}

func TestDecodeModels(t *testing.T) {
	raw := fixtures.MustContract(t, "models.json")
	models, err := contract.DecodeModels(raw)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if len(models.Models) != 2 {
		t.Fatalf("models = %d, want 2", len(models.Models))
	}
	if models.Models[0].Name != "jev-latest" {
		t.Errorf("name = %q", models.Models[0].Name)
	}

	// Unknown top-level fields survive the round trip.
	out, encErr := models.Encode()
	if encErr != nil {
		t.Fatal(encErr)
	}
	if !strings.Contains(string(out), "x_account") {
		t.Error("unknown models field x_account was dropped")
	}
}

// jsonValue generates valid JSON values for the round-trip property.
type jsonValue string

func (jsonValue) Generate(r *rand.Rand, _ int) reflect.Value {
	values := []string{
		`1`, `2.5`, `"text"`, `null`, `true`, `false`,
		`{"a":[1,"b"]}`, `[{"x":null},"y"]`, `{"deep":{"deeper":[0.5]}}`,
	}
	return reflect.ValueOf(jsonValue(values[r.Intn(len(values))]))
}

// TestRoundTripProperty: for the valid corpus plus generated unknown-field
// decorations, decode→encode is deterministic and decode(encode(x)) == x.
func TestRoundTripProperty(t *testing.T) {
	for _, name := range []string{"request_full.json", "unknown_field.json"} {
		raw := fixtures.MustContract(t, name)
		req, err := contract.DecodeRequest(raw)
		if err != nil {
			t.Fatalf("%s: decode failed: %v", name, err)
		}
		out1, encErr := req.Encode()
		if encErr != nil {
			t.Fatalf("%s: encode failed: %v", name, encErr)
		}
		out2, _ := req.Encode()
		if string(out1) != string(out2) {
			t.Errorf("%s: encode is not deterministic", name)
		}
		req2, err := contract.DecodeRequest(out1)
		if err != nil {
			t.Fatalf("%s: re-decode failed: %v", name, err)
		}
		if !reflect.DeepEqual(req, req2) {
			t.Errorf("%s: round trip changed the document", name)
		}
	}

	property := func(value jsonValue) bool {
		doc := []byte(`{"state":"s","model":"m","questions":{"q":{"type":"noul","instructions":"i"}},"x_prop":` + value + `}`)
		req, err := contract.DecodeRequest(doc)
		if err != nil {
			return false
		}
		out, encErr := req.Encode()
		if encErr != nil {
			return false
		}
		req2, err := contract.DecodeRequest(out)
		if err != nil {
			return false
		}
		return reflect.DeepEqual(req, req2)
	}
	cfg := &quick.Config{MaxCount: 200, Rand: rand.New(rand.NewSource(1))}
	if err := quick.Check(property, cfg); err != nil {
		t.Errorf("round-trip property failed: %v", err)
	}
}
