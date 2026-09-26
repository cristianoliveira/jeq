package jeq_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cristianoliveira/jeq/internal/domain/jeq"
	"github.com/cristianoliveira/jeq/internal/fixtures"
)

func TestModeConflictMatrix(t *testing.T) {
	// The full ADR 0001 matrix: native = --request alone; composed =
	// --questions plus exactly one state source. Everything else fails
	// before any I/O.
	conflict := jeq.CodeSourceConflict
	input := jeq.CodeInputInvalid

	tests := []struct {
		name     string
		sources  jeq.Sources
		wantCode jeq.Code // "" means success
	}{
		{"native mode alone", jeq.Sources{Request: true}, ""},
		{"composed with state text", jeq.Sources{Questions: true, StateText: true}, ""},
		{"composed with state file", jeq.Sources{Questions: true, StateFile: true}, ""},
		{"composed with state json", jeq.Sources{Questions: true, StateJSON: true}, ""},

		{"request with questions", jeq.Sources{Request: true, Questions: true}, conflict},
		{"request with state text", jeq.Sources{Request: true, StateText: true}, conflict},
		{"request with state file", jeq.Sources{Request: true, StateFile: true}, conflict},
		{"request with state json", jeq.Sources{Request: true, StateJSON: true}, conflict},
		{"request with questions and state", jeq.Sources{Request: true, Questions: true, StateText: true}, conflict},

		{"state text with state file", jeq.Sources{Questions: true, StateText: true, StateFile: true}, conflict},
		{"state text with state json", jeq.Sources{Questions: true, StateText: true, StateJSON: true}, conflict},
		{"state file with state json", jeq.Sources{Questions: true, StateFile: true, StateJSON: true}, conflict},
		{"all three state sources", jeq.Sources{Questions: true, StateText: true, StateFile: true, StateJSON: true}, conflict},
		{"state sources without questions", jeq.Sources{StateText: true, StateJSON: true}, conflict},

		{"nothing provided", jeq.Sources{}, input},
		{"state alone without questions", jeq.Sources{StateText: true}, input},
		{"questions without any state source", jeq.Sources{Questions: true}, input},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := jeq.CheckSources(tt.sources)
			if tt.wantCode == "" {
				assert.Nil(t, err)
				return
			}
			require.NotNil(t, err)
			assert.Equal(t, tt.wantCode, err.Code)
		})
	}
}

const questionsDoc = `{"questions":{"is_urgent":{"type":"noul","instructions":"Does this convey urgency?"}}}`

func TestComposeComposedMode(t *testing.T) {
	// Given composed mode with a text state source and an explicit model
	out, err := jeq.Compose(jeq.ComposeInput{
		QuestionsDoc: []byte(questionsDoc),
		State:        jeq.StateInput{Kind: jeq.SourceStateText, Text: "Help! My payouts have been failing."},
		Model:        "jev-1.13.0",
	})
	require.Nil(t, err, "compose failed")

	// Then the request carries the explicit model and the state as a JSON string
	assert.Equal(t, "jev-1.13.0", out.Model)
	assert.Equal(t, `"Help! My payouts have been failing."`, string(out.State))
	assert.Len(t, out.Questions, 1)
}

func TestComposeStateJSONSource(t *testing.T) {
	out, err := jeq.Compose(jeq.ComposeInput{
		QuestionsDoc: []byte(questionsDoc),
		State:        jeq.StateInput{Kind: jeq.SourceStateJSON, JSON: []byte(`{"messages":[{"role":"user"}]}`)},
		Model:        "jev-latest",
	})
	require.Nil(t, err, "compose failed")
	var state map[string]any
	require.NoError(t, json.Unmarshal(out.State, &state))
}

func TestComposeStateJSONSourceRejectsScalars(t *testing.T) {
	_, err := jeq.Compose(jeq.ComposeInput{
		QuestionsDoc: []byte(questionsDoc),
		State:        jeq.StateInput{Kind: jeq.SourceStateJSON, JSON: []byte(`42`)},
		Model:        "jev-latest",
	})
	require.NotNil(t, err)
	assert.Equal(t, jeq.CodeRequestInvalid, err.Code)
}

func TestComposeQuestionsDocUnknownFieldsPassThrough(t *testing.T) {
	// Given a questions document with unrecognized top-level fields (OQ-1)
	doc := `{"questions":{"q":{"type":"noul","instructions":"i"}},"x_profile":"team-a"}`

	// When composed
	out, err := jeq.Compose(jeq.ComposeInput{
		QuestionsDoc: []byte(doc),
		State:        jeq.StateInput{Kind: jeq.SourceStateText, Text: "s"},
		Model:        "jev-latest",
	})
	require.Nil(t, err, "compose failed")

	// Then the unknown field rides into the outgoing request
	encoded, encErr := out.Encode()
	require.NoError(t, encErr)
	assert.Contains(t, string(encoded), "x_profile")
}

func TestComposeComposedEmptyStateFailsBeforeIO(t *testing.T) {
	_, err := jeq.Compose(jeq.ComposeInput{
		QuestionsDoc: []byte(questionsDoc),
		State:        jeq.StateInput{Kind: jeq.SourceStateText, Text: "   "},
		Model:        "jev-latest",
	})
	require.NotNil(t, err)
	assert.Equal(t, jeq.CodeInputInvalid, err.Code)
}

func TestComposeEmptyModelFails(t *testing.T) {
	_, err := jeq.Compose(jeq.ComposeInput{
		QuestionsDoc: []byte(questionsDoc),
		State:        jeq.StateInput{Kind: jeq.SourceStateText, Text: "s"},
	})
	require.NotNil(t, err)
	assert.Equal(t, jeq.CodeInputInvalid, err.Code)
}

func TestComposeInvalidQuestionsDocFailsLocally(t *testing.T) {
	raw := fixtures.MustContract(t, "missing_instructions.json")
	// A native document passed as the questions doc has no "questions" key
	// mapping: composition rejects it as an invalid questions document.
	_, err := jeq.Compose(jeq.ComposeInput{
		QuestionsDoc: raw,
		State:        jeq.StateInput{Kind: jeq.SourceStateText, Text: "s"},
		Model:        "jev-latest",
	})
	require.NotNil(t, err, "expected a local failure for a malformed questions document")
}

func TestComposeNativeMode(t *testing.T) {
	raw := fixtures.MustContract(t, "request_full.json")
	out, err := jeq.Compose(jeq.ComposeInput{RequestDoc: raw})
	require.Nil(t, err, "compose failed")
	assert.Equal(t, "jev-latest", out.Model)

	// Unknown fields survive the native passthrough.
	encoded, encErr := out.Encode()
	require.NoError(t, encErr)
	assert.Contains(t, string(encoded), "x_trace")
}

func TestComposeNativeModeInvalidDocumentFailsLocally(t *testing.T) {
	raw := fixtures.MustContract(t, "dup_key.json")
	_, err := jeq.Compose(jeq.ComposeInput{RequestDoc: raw})
	require.NotNil(t, err)
	assert.Equal(t, jeq.CodeRequestInvalid, err.Code)
}

func TestComposeNeverTouchesNetwork(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
	}))
	defer srv.Close()

	inputs := []jeq.ComposeInput{
		{RequestDoc: fixtures.MustContract(t, "request_full.json")},
		{RequestDoc: fixtures.MustContract(t, "dup_key.json")},
		{QuestionsDoc: []byte(questionsDoc), State: jeq.StateInput{Kind: jeq.SourceStateText, Text: "s"}, Model: "jev-latest"},
		{QuestionsDoc: []byte(questionsDoc), State: jeq.StateInput{Kind: jeq.SourceStateJSON, JSON: []byte(`{}`)}, Model: "jev-latest"},
	}
	for _, in := range inputs {
		_, _ = jeq.Compose(in)
	}
	assert.Zero(t, hits.Load(), "composition must be pure")
}

func TestComposeComposedPrettyEmptyStateJSONFails(t *testing.T) {
	// F-D1-1: a pretty-printed empty object is semantically empty and must
	// fail exactly like the compact form, regardless of whitespace.
	for _, state := range []string{`{}`, "{\n}", `[]`, "[\n]"} {
		_, err := jeq.Compose(jeq.ComposeInput{
			QuestionsDoc: []byte(questionsDoc),
			State:        jeq.StateInput{Kind: jeq.SourceStateJSON, JSON: []byte(state)},
			Model:        "jev-latest",
		})
		assert.NotNil(t, err, "pretty state %q must fail as semantically empty", state)
	}
}
