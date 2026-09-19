package gev_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/cristianoliveira/gev/internal/domain/gev"
	"github.com/cristianoliveira/gev/internal/fixtures"
)

func TestModeConflictMatrix(t *testing.T) {
	// The full ADR 0001 matrix: native = --request alone; composed =
	// --questions plus exactly one state source. Everything else fails
	// before any I/O.
	conflict := gev.CodeSourceConflict
	input := gev.CodeInputInvalid

	tests := []struct {
		name     string
		sources  gev.Sources
		wantCode gev.Code // "" means success
	}{
		{"native mode alone", gev.Sources{Request: true}, ""},
		{"composed with state text", gev.Sources{Questions: true, StateText: true}, ""},
		{"composed with state file", gev.Sources{Questions: true, StateFile: true}, ""},
		{"composed with state json", gev.Sources{Questions: true, StateJSON: true}, ""},

		{"request with questions", gev.Sources{Request: true, Questions: true}, conflict},
		{"request with state text", gev.Sources{Request: true, StateText: true}, conflict},
		{"request with state file", gev.Sources{Request: true, StateFile: true}, conflict},
		{"request with state json", gev.Sources{Request: true, StateJSON: true}, conflict},
		{"request with questions and state", gev.Sources{Request: true, Questions: true, StateText: true}, conflict},

		{"state text with state file", gev.Sources{Questions: true, StateText: true, StateFile: true}, conflict},
		{"state text with state json", gev.Sources{Questions: true, StateText: true, StateJSON: true}, conflict},
		{"state file with state json", gev.Sources{Questions: true, StateFile: true, StateJSON: true}, conflict},
		{"all three state sources", gev.Sources{Questions: true, StateText: true, StateFile: true, StateJSON: true}, conflict},
		{"state sources without questions", gev.Sources{StateText: true, StateJSON: true}, conflict},

		{"nothing provided", gev.Sources{}, input},
		{"state alone without questions", gev.Sources{StateText: true}, input},
		{"questions without any state source", gev.Sources{Questions: true}, input},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := gev.CheckSources(tt.sources)
			if tt.wantCode == "" {
				if err != nil {
					t.Fatalf("expected success, got %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected %q, got success", tt.wantCode)
			}
			if err.Code != tt.wantCode {
				t.Errorf("code = %q, want %q", err.Code, tt.wantCode)
			}
		})
	}
}

const questionsDoc = `{"questions":{"is_urgent":{"type":"noul","instructions":"Does this convey urgency?"}}}`

func TestComposeComposedMode(t *testing.T) {
	// Given composed mode with a text state source and an explicit model
	out, err := gev.Compose(gev.ComposeInput{
		QuestionsDoc: []byte(questionsDoc),
		State:        gev.StateInput{Kind: gev.SourceStateText, Text: "Help! My payouts have been failing."},
		Model:        "jev-1.13.0",
	})
	if err != nil {
		t.Fatalf("compose failed: %v", err)
	}

	// Then the request carries the explicit model and the state as a JSON string
	if out.Model != "jev-1.13.0" {
		t.Errorf("model = %q, want the explicit model", out.Model)
	}
	if string(out.State) != `"Help! My payouts have been failing."` {
		t.Errorf("state = %s, want a JSON string", out.State)
	}
	if len(out.Questions) != 1 {
		t.Errorf("questions = %d, want 1", len(out.Questions))
	}
}

func TestComposeStateJSONSource(t *testing.T) {
	out, err := gev.Compose(gev.ComposeInput{
		QuestionsDoc: []byte(questionsDoc),
		State:        gev.StateInput{Kind: gev.SourceStateJSON, JSON: []byte(`{"messages":[{"role":"user"}]}`)},
		Model:        "jev-latest",
	})
	if err != nil {
		t.Fatalf("compose failed: %v", err)
	}
	var state map[string]any
	if err := json.Unmarshal(out.State, &state); err != nil {
		t.Fatalf("state is not structured JSON: %v", err)
	}
}

func TestComposeStateJSONSourceRejectsScalars(t *testing.T) {
	_, err := gev.Compose(gev.ComposeInput{
		QuestionsDoc: []byte(questionsDoc),
		State:        gev.StateInput{Kind: gev.SourceStateJSON, JSON: []byte(`42`)},
		Model:        "jev-latest",
	})
	if err == nil || err.Code != gev.CodeRequestInvalid {
		t.Errorf("expected %q for scalar state JSON, got %v", gev.CodeRequestInvalid, err)
	}
}

func TestComposeQuestionsDocUnknownFieldsPassThrough(t *testing.T) {
	// Given a questions document with unrecognized top-level fields (OQ-1)
	doc := `{"questions":{"q":{"type":"noul","instructions":"i"}},"x_profile":"team-a"}`

	// When composed
	out, err := gev.Compose(gev.ComposeInput{
		QuestionsDoc: []byte(doc),
		State:        gev.StateInput{Kind: gev.SourceStateText, Text: "s"},
		Model:        "jev-latest",
	})
	if err != nil {
		t.Fatalf("compose failed: %v", err)
	}

	// Then the unknown field rides into the outgoing request
	encoded, encErr := out.Encode()
	if encErr != nil {
		t.Fatal(encErr)
	}
	if !strings.Contains(string(encoded), "x_profile") {
		t.Errorf("unknown questions-doc field x_profile was dropped: %s", encoded)
	}
}

func TestComposeComposedEmptyStateFailsBeforeIO(t *testing.T) {
	_, err := gev.Compose(gev.ComposeInput{
		QuestionsDoc: []byte(questionsDoc),
		State:        gev.StateInput{Kind: gev.SourceStateText, Text: "   "},
		Model:        "jev-latest",
	})
	if err == nil || err.Code != gev.CodeInputInvalid {
		t.Errorf("expected %q for empty resolved state, got %v", gev.CodeInputInvalid, err)
	}
}

func TestComposeEmptyModelFails(t *testing.T) {
	_, err := gev.Compose(gev.ComposeInput{
		QuestionsDoc: []byte(questionsDoc),
		State:        gev.StateInput{Kind: gev.SourceStateText, Text: "s"},
	})
	if err == nil || err.Code != gev.CodeInputInvalid {
		t.Errorf("expected %q for missing model, got %v", gev.CodeInputInvalid, err)
	}
}

func TestComposeInvalidQuestionsDocFailsLocally(t *testing.T) {
	raw := fixtures.MustContract(t, "missing_instructions.json")
	// A native document passed as the questions doc has no "questions" key
	// mapping: composition rejects it as an invalid questions document.
	_, err := gev.Compose(gev.ComposeInput{
		QuestionsDoc: raw,
		State:        gev.StateInput{Kind: gev.SourceStateText, Text: "s"},
		Model:        "jev-latest",
	})
	if err == nil {
		t.Fatal("expected a local failure for a malformed questions document")
	}
}

func TestComposeNativeMode(t *testing.T) {
	raw := fixtures.MustContract(t, "request_full.json")
	out, err := gev.Compose(gev.ComposeInput{RequestDoc: raw})
	if err != nil {
		t.Fatalf("compose failed: %v", err)
	}
	if out.Model != "jev-latest" {
		t.Errorf("model = %q, want the document's model", out.Model)
	}

	// Unknown fields survive the native passthrough.
	encoded, encErr := out.Encode()
	if encErr != nil {
		t.Fatal(encErr)
	}
	if !strings.Contains(string(encoded), "x_trace") {
		t.Error("native unknown field x_trace was dropped")
	}
}

func TestComposeNativeModeInvalidDocumentFailsLocally(t *testing.T) {
	raw := fixtures.MustContract(t, "dup_key.json")
	_, err := gev.Compose(gev.ComposeInput{RequestDoc: raw})
	if err == nil || err.Code != gev.CodeRequestInvalid {
		t.Errorf("expected %q, got %v", gev.CodeRequestInvalid, err)
	}
}

func TestComposeNeverTouchesNetwork(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
	}))
	defer srv.Close()

	inputs := []gev.ComposeInput{
		{RequestDoc: fixtures.MustContract(t, "request_full.json")},
		{RequestDoc: fixtures.MustContract(t, "dup_key.json")},
		{QuestionsDoc: []byte(questionsDoc), State: gev.StateInput{Kind: gev.SourceStateText, Text: "s"}, Model: "jev-latest"},
		{QuestionsDoc: []byte(questionsDoc), State: gev.StateInput{Kind: gev.SourceStateJSON, JSON: []byte(`{}`)}, Model: "jev-latest"},
	}
	for _, in := range inputs {
		_, _ = gev.Compose(in)
	}
	if hits.Load() != 0 {
		t.Errorf("composition made %d network requests; it must be pure", hits.Load())
	}
}

func TestComposeComposedPrettyEmptyStateJSONFails(t *testing.T) {
	// F-D1-1: a pretty-printed empty object is semantically empty and must
	// fail exactly like the compact form, regardless of whitespace.
	for _, state := range []string{`{}`, "{\n}", `[]`, "[\n]"} {
		_, err := gev.Compose(gev.ComposeInput{
			QuestionsDoc: []byte(questionsDoc),
			State:        gev.StateInput{Kind: gev.SourceStateJSON, JSON: []byte(state)},
			Model:        "jev-latest",
		})
		if err == nil {
			t.Errorf("pretty state %q must fail as semantically empty", state)
		}
	}
}
