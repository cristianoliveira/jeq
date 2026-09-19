package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/cristianoliveira/gev/internal/domain/contract"
	"github.com/cristianoliveira/gev/internal/domain/gev"
)

type fakeEvaluator struct {
	response contract.Response
	err      *gev.Error
	calls    int
	request  contract.Request
}

func (fake *fakeEvaluator) Evaluate(_ context.Context, request contract.Request) (contract.Response, *gev.Error) {
	fake.calls++
	fake.request = request
	if fake.err != nil {
		return contract.Response{}, fake.err
	}
	return fake.response, nil
}

func testQuestions() map[string]contract.Question {
	return map[string]contract.Question{
		"route": {Type: contract.TypeChoice, Instructions: json.RawMessage(`"route"`), Criteria: json.RawMessage(`{"a":"A","b":"B"}`)},
	}
}

func testResponse() contract.Response {
	confidence := 0.91
	choice := "a"
	return contract.Response{
		Model: "jev-test",
		Answers: map[string]contract.Answer{
			"route": {Type: contract.TypeChoice, Choice: &choice, Confidence: &confidence, Probs: map[string]float64{"a": 0.91, "b": 0.09}},
		},
		Usage: contract.Usage{InputTokens: 12, OutputTokens: 4},
		Extra: map[string]json.RawMessage{"server_extra": json.RawMessage(`{"kept":true}`)},
	}
}

func run(t *testing.T, record, pointer, name string, fake *fakeEvaluator) ([]byte, *gev.Error) {
	t.Helper()
	return Enrich(context.Background(), []byte(record), Config{Name: name, Pointer: pointer, Model: "jev-test", Questions: testQuestions()}, fake)
}

func decodeOutput(t *testing.T, output []byte) map[string]json.RawMessage {
	t.Helper()
	var members map[string]json.RawMessage
	if err := json.Unmarshal(output, &members); err != nil {
		t.Fatal(err)
	}
	return members
}

func TestPointersPreserveRawStateAndCallOnce(t *testing.T) {
	cases := []struct {
		name    string
		record  string
		pointer string
		want    string
	}{
		{name: "root object", record: `{"payload":{"x":1},"unicode":"café"}`, pointer: "", want: `{"payload":{"x":1},"unicode":"café"}`},
		{name: "nested object", record: `{"payload":{"x":1}}`, pointer: "/payload", want: `{"x":1}`},
		{name: "escaped members", record: `{"a/b":{"~key":"value"}}`, pointer: "/a~1b/~0key", want: `"value"`},
		{name: "array element", record: `{"items":["first",{"second":true}]}`, pointer: "/items/1", want: `{"second":true}`},
		{name: "unsafe exponent", record: `{"value":123456789012345678901234567890e+10,"payload":{"ok":true}}`, pointer: "/payload", want: `{"ok":true}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fake := &fakeEvaluator{response: testResponse()}
			output, err := run(t, tc.record, tc.pointer, "step", fake)
			if err != nil || fake.calls != 1 {
				t.Fatalf("err=%v calls=%d", err, fake.calls)
			}
			if got := string(fake.request.State); got != tc.want {
				t.Fatalf("state=%s want=%s", got, tc.want)
			}
			members := decodeOutput(t, output)
			if tc.name == "unsafe exponent" && string(members["value"]) != `123456789012345678901234567890e+10` {
				t.Fatalf("unsafe number=%s", members["value"])
			}
			gevRaw := decodeOutput(t, members["_gev"])
			responseRaw := decodeOutput(t, gevRaw["step"])
			if string(responseRaw["server_extra"]) != `{"kept":true}` {
				t.Fatalf("response extra=%s", responseRaw["server_extra"])
			}
		})
	}
}

func TestStringObjectAndNullStates(t *testing.T) {
	fake := &fakeEvaluator{response: testResponse()}
	if _, err := run(t, `{"text":"hello"}`, "/text", "text", fake); err != nil {
		t.Fatalf("string: %v", err)
	}
	fake = &fakeEvaluator{response: testResponse()}
	if _, err := run(t, `{"value":null}`, "/value", "null", fake); err == nil || fake.calls != 0 {
		t.Fatalf("null err=%v calls=%d", err, fake.calls)
	}
	fake = &fakeEvaluator{response: testResponse()}
	if _, err := run(t, `{"items":[1,2]}`, "/items", "array", fake); err != nil || fake.calls != 1 {
		t.Fatalf("array state err=%v calls=%d", err, fake.calls)
	}
}

func TestEnvelopeCollisionsNamesAndDuplicateKeys(t *testing.T) {
	cases := []struct{ name, record, pointer string }{
		{"non-object root", `[]`, ""},
		{"nested duplicate", `{"payload":{"x":1,"x":2}}`, "/payload"},
		{"existing evidence duplicate", `{"_gev":{"a":1,"a":2}}`, "/"},
		{"existing evidence wrong type", `{"_gev":[]}`, ""},
		{"collision", `{"_gev":{"step":{}}}`, ""},
		{"invalid pointer", `{"x":1}`, "x"},
		{"bad escape", `{"x":1}`, "/x~2"},
		{"missing pointer", `{"x":1}`, "/missing"},
		{"noncanonical index", `{"x":[1]}`, "/x/01"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fake := &fakeEvaluator{response: testResponse()}
			name := "step"
			if tc.name == "collision" {
				name = "step"
			}
			if _, err := run(t, tc.record, tc.pointer, name, fake); err == nil || fake.calls != 0 || err.Code != gev.CodeInputInvalid {
				t.Fatalf("err=%v calls=%d", err, fake.calls)
			}
		})
	}
}

func TestNamesAndChainedEnrichment(t *testing.T) {
	for _, name := range []string{"", "-bad", "bad space", strings.Repeat("a", 65)} {
		fake := &fakeEvaluator{response: testResponse()}
		if _, err := run(t, `{"x":1}`, "/x", name, fake); err == nil || fake.calls != 0 {
			t.Fatalf("name=%q err=%v calls=%d", name, err, fake.calls)
		}
	}
	firstFake := &fakeEvaluator{response: testResponse()}
	first, err := run(t, `{"payload":{"id":1}}`, "/payload", "first", firstFake)
	if err != nil {
		t.Fatal(err)
	}
	secondFake := &fakeEvaluator{response: testResponse()}
	second, err := run(t, string(first), "", "second", secondFake)
	if err != nil || secondFake.calls != 1 {
		t.Fatalf("second err=%v calls=%d", err, secondFake.calls)
	}
	members := decodeOutput(t, second)
	gevRaw := decodeOutput(t, members["_gev"])
	if gevRaw["first"] == nil || gevRaw["second"] == nil {
		t.Fatalf("evidence=%s", members["_gev"])
	}
}

func TestEvaluatorErrorPropagatesUnchanged(t *testing.T) {
	sentinel := gev.NewError(gev.CodeAuthRejected, "denied")
	fake := &fakeEvaluator{err: sentinel}
	_, err := run(t, `{"x":{"ok":true}}`, "/x", "step", fake)
	if err != sentinel || fake.calls != 1 {
		t.Fatalf("err=%v calls=%d", err, fake.calls)
	}
	if errors.Is(err, context.Canceled) {
		t.Fatal("unexpected cancellation")
	}
}
