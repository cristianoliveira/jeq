package pipeline

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cristianoliveira/jeq/internal/domain/contract"
	"github.com/cristianoliveira/jeq/internal/domain/jeq"
)

type fakeEvaluator struct {
	response contract.Response
	err      *jeq.Error
	calls    int
	request  contract.Request
}

func (fake *fakeEvaluator) Evaluate(_ context.Context, request contract.Request) (contract.Response, *jeq.Error) {
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

func run(t *testing.T, record, pointer, name string, fake *fakeEvaluator) ([]byte, *jeq.Error) {
	t.Helper()
	return Enrich(context.Background(), []byte(record), Config{Name: name, Pointer: pointer, Model: "jev-test", Questions: testQuestions()}, fake)
}

func decodeOutput(t *testing.T, output []byte) map[string]json.RawMessage {
	t.Helper()
	var members map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(output, &members))
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
			require.Nil(t, err)
			assert.Equal(t, 1, fake.calls)
			assert.Equal(t, tc.want, string(fake.request.State))
			members := decodeOutput(t, output)
			if tc.name == "unsafe exponent" {
				assert.Equal(t, `123456789012345678901234567890e+10`, string(members["value"]))
			}
			jeqRaw := decodeOutput(t, members["_jeq"])
			responseRaw := decodeOutput(t, jeqRaw["step"])
			assert.Equal(t, `{"kept":true}`, string(responseRaw["server_extra"]))
		})
	}
}

func TestStringObjectAndNullStates(t *testing.T) {
	fake := &fakeEvaluator{response: testResponse()}
	_, err := run(t, `{"text":"hello"}`, "/text", "text", fake)
	require.Nil(t, err, "string state")
	fake = &fakeEvaluator{response: testResponse()}
	_, err = run(t, `{"value":null}`, "/value", "null", fake)
	require.NotNil(t, err)
	assert.Zero(t, fake.calls)
	fake = &fakeEvaluator{response: testResponse()}
	_, err = run(t, `{"items":[1,2]}`, "/items", "array", fake)
	require.Nil(t, err)
	assert.Equal(t, 1, fake.calls)
}

func TestEnvelopeCollisionsNamesAndDuplicateKeys(t *testing.T) {
	cases := []struct{ name, record, pointer string }{
		{"non-object root", `[]`, ""},
		{"nested duplicate", `{"payload":{"x":1,"x":2}}`, "/payload"},
		{"existing evidence duplicate", `{"_jeq":{"a":1,"a":2}}`, "/"},
		{"existing evidence wrong type", `{"_jeq":[]}`, ""},
		{"collision", `{"_jeq":{"step":{}}}`, ""},
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
			_, err := run(t, tc.record, tc.pointer, name, fake)
			require.NotNil(t, err)
			assert.Zero(t, fake.calls)
			assert.Equal(t, jeq.CodeInputInvalid, err.Code)
		})
	}
}

func TestNamesAndChainedEnrichment(t *testing.T) {
	for _, name := range []string{"", "-bad", "bad space", strings.Repeat("a", 65)} {
		fake := &fakeEvaluator{response: testResponse()}
		_, err := run(t, `{"x":1}`, "/x", name, fake)
		require.NotNil(t, err, "name=%q", name)
		assert.Zero(t, fake.calls)
	}
	firstFake := &fakeEvaluator{response: testResponse()}
	first, err := run(t, `{"payload":{"id":1}}`, "/payload", "first", firstFake)
	require.Nil(t, err)
	secondFake := &fakeEvaluator{response: testResponse()}
	second, err := run(t, string(first), "", "second", secondFake)
	require.Nil(t, err)
	assert.Equal(t, 1, secondFake.calls)
	members := decodeOutput(t, second)
	jeqRaw := decodeOutput(t, members["_jeq"])
	assert.NotNil(t, jeqRaw["first"])
	assert.NotNil(t, jeqRaw["second"])
}

func TestEvaluatorErrorPropagatesUnchanged(t *testing.T) {
	sentinel := jeq.NewError(jeq.CodeAuthRejected, "denied")
	fake := &fakeEvaluator{err: sentinel}
	_, err := run(t, `{"x":{"ok":true}}`, "/x", "step", fake)
	assert.Same(t, sentinel, err)
	assert.Equal(t, 1, fake.calls)
	assert.NotErrorIs(t, err, context.Canceled)
}
