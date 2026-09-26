package contract_test

import (
	"encoding/json"
	"math/rand"
	"reflect"
	"testing"
	"testing/quick"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cristianoliveira/jeq/internal/domain/contract"
	"github.com/cristianoliveira/jeq/internal/domain/jeq"
	"github.com/cristianoliveira/jeq/internal/fixtures"
)

func TestDecodeRequestStrict(t *testing.T) {
	tests := []struct {
		name     string
		doc      string
		wantCode jeq.Code
	}{
		{
			name:     "truncated document is rejected",
			doc:      `{"state":"a","model"`,
			wantCode: jeq.CodeRequestInvalid,
		},
		{
			name:     "trailing comma is rejected",
			doc:      `{"state":"a","model":"m","questions":{},}`,
			wantCode: jeq.CodeRequestInvalid,
		},
		{
			name:     "duplicate top-level key is rejected, not last-wins",
			doc:      `{"state":"a","state":"b","model":"m","questions":{"q":{"type":"noul","instructions":"i"}}}`,
			wantCode: jeq.CodeRequestInvalid,
		},
		{
			name:     "duplicate nested key is rejected",
			doc:      `{"state":"a","model":"m","questions":{"q":{"type":"noul","instructions":"i","instructions":"j"}}}`,
			wantCode: jeq.CodeRequestInvalid,
		},
		{
			name:     "UTF-8 BOM is rejected",
			doc:      "\xEF\xBB\xBF{\"state\":\"a\"}",
			wantCode: jeq.CodeRequestInvalid,
		},
		{
			name:     "invalid UTF-8 is rejected",
			doc:      "{\"state\":\"\xff\xfe\"}",
			wantCode: jeq.CodeRequestInvalid,
		},
		{
			name:     "known-field type mismatch",
			doc:      `{"state":"s","model":"m","questions":{"f":{"type":"score","instructions":"rate it","criteria":"Calm"}}}`,
			wantCode: jeq.CodeRequestInvalid,
		},
		{
			name:     "model as number is a type mismatch",
			doc:      `{"state":"s","model":7,"questions":{"q":{"type":"noul","instructions":"i"}}}`,
			wantCode: jeq.CodeRequestInvalid,
		},
		{
			name:     "questions as array is a type mismatch",
			doc:      `{"state":"s","model":"m","questions":[]}`,
			wantCode: jeq.CodeRequestInvalid,
		},
		{
			name:     "trailing data after the document is rejected",
			doc:      `{"state":"a","model":"m","questions":{}} {"x":1}`,
			wantCode: jeq.CodeRequestInvalid,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := contract.DecodeRequest([]byte(tt.doc))
			require.NotNil(t, err)
			assert.Equal(t, tt.wantCode, err.Code, "message: %s", err.Message)
		})
	}
}

func TestDecodeRequestTypeMismatchNamesFieldPath(t *testing.T) {
	_, err := contract.DecodeRequest([]byte(`{"state":"s","model":"m","questions":{"f":{"type":"score","instructions":"rate it","criteria":"Calm"}}}`))
	require.NotNil(t, err)
	assert.Contains(t, err.Message, "questions.f.criteria")
}

func TestDecodeRequestHappyPath(t *testing.T) {
	raw := fixtures.MustContract(t, "request_full.json")
	req, err := contract.DecodeRequest(raw)
	require.Nil(t, err)
	assert.Equal(t, "jev-latest", req.Model)
	require.Len(t, req.Questions, 3)
	assert.Equal(t, contract.TypeScore, req.Questions["frustration"].Type)
}

func TestUnknownFieldPassthrough(t *testing.T) {
	// Given a request with unknown fields at the top level and inside questions
	raw := fixtures.MustContract(t, "unknown_field.json")

	// When it decodes and re-encodes
	req, decErr := contract.DecodeRequest(raw)
	require.Nil(t, decErr, "unknown fields must never be rejected")
	out, encErr := req.Encode()
	require.NoError(t, encErr)

	// Then the unknown fields survive byte-semantically
	var round map[string]any
	require.NoError(t, json.Unmarshal(out, &round))
	require.Contains(t, round, "x_extra")
	questions, ok := round["questions"].(map[string]any)
	require.True(t, ok)
	q, ok := questions["is_urgent"].(map[string]any)
	require.True(t, ok)
	assert.Contains(t, q, "vendor_meta")

	// And re-decoding the output is an idempotent fixpoint
	req2, decErr := contract.DecodeRequest(out)
	require.Nil(t, decErr)
	out2, _ := req2.Encode()
	req3, _ := contract.DecodeRequest(out2)
	assert.Equal(t, req2, req3)
}

func TestLosslessResponse(t *testing.T) {
	raw := fixtures.MustContract(t, "response_unknown_fields.json")

	resp, err := contract.DecodeResponse(raw)
	require.Nil(t, err, "response decode failed")

	out, encErr := resp.Encode()
	require.NoError(t, encErr)

	var round map[string]any
	require.NoError(t, json.Unmarshal(out, &round))
	require.Contains(t, round, "x_served_by")
	answers, ok := round["answers"].(map[string]any)
	require.True(t, ok)
	dept, ok := answers["department"].(map[string]any)
	require.True(t, ok)
	assert.Contains(t, dept, "x_rank")
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
			require.NotNil(t, err)
			assert.Equal(t, jeq.CodeResponseInvalid, err.Code)
		})
	}
}

func TestDecodeResponseHappyPath(t *testing.T) {
	raw := fixtures.MustContract(t, "response_unknown_fields.json")
	resp, err := contract.DecodeResponse(raw)
	require.Nil(t, err, "decode failed")
	assert.Equal(t, "jev-1.13.0", resp.Model)
	assert.Len(t, resp.Answers, 3)
	assert.Equal(t, int64(312), resp.Usage.InputTokens)
	assert.Equal(t, int64(48), resp.Usage.OutputTokens)
	require.NotNil(t, resp.Answers["is_urgent"].Noul)
	assert.Equal(t, 0.92, *resp.Answers["is_urgent"].Noul)
	require.NotNil(t, resp.Answers["department"].Confidence)
	assert.Equal(t, 0.82, *resp.Answers["department"].Confidence)
}

func TestDecodeModels(t *testing.T) {
	raw := fixtures.MustContract(t, "models.json")
	models, err := contract.DecodeModels(raw)
	require.Nil(t, err, "decode failed")
	require.Len(t, models.Models, 2)
	assert.Equal(t, "jev-latest", models.Models[0].Name)

	// Unknown top-level fields survive the round trip.
	out, encErr := models.Encode()
	require.NoError(t, encErr)
	assert.Contains(t, string(out), "x_account")
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
		require.Nil(t, err, "%s: decode failed", name)
		out1, encErr := req.Encode()
		require.NoError(t, encErr, "%s: encode failed", name)
		out2, _ := req.Encode()
		assert.Equal(t, string(out1), string(out2), "%s: encode is not deterministic", name)
		req2, err := contract.DecodeRequest(out1)
		require.Nil(t, err, "%s: re-decode failed", name)
		assert.Equal(t, req, req2, "%s: round trip changed the document", name)
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
	assert.NoError(t, quick.Check(property, cfg), "round-trip property")
}
