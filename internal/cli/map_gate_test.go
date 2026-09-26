package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cristianoliveira/jeq/internal/domain/contract"
	"github.com/cristianoliveira/jeq/internal/domain/jeq"
)

type streamRenderer struct{}

func (streamRenderer) RenderSuccess(w io.Writer, response contract.Response) error {
	data, err := response.Encode()
	if err != nil {
		return err
	}
	_, err = w.Write(append(data, '\n'))
	return err
}

func (streamRenderer) RenderError(w io.Writer, err *jeq.Error) error {
	return json.NewEncoder(w).Encode(map[string]any{"code": err.Code, "message": err.Message, "recovery": err.Recovery})
}

func (streamRenderer) RenderValue(w io.Writer, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = w.Write(append(data, '\n'))
	return err
}

func (streamRenderer) RenderRaw(w io.Writer, raw []byte) error {
	_, err := w.Write(append(raw, '\n'))
	return err
}

type streamClient struct {
	mu       sync.Mutex
	calls    int
	requests []contract.Request
}

func (c *streamClient) Evaluate(_ context.Context, request contract.Request) (contract.Response, *jeq.Error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls++
	c.requests = append(c.requests, request)
	return contract.Response{Model: "m", Answers: map[string]contract.Answer{}, Usage: contract.Usage{}}, nil
}

func streamDeps(client *streamClient, input string, questions string) (AskDeps, *bytes.Buffer) {
	stdin := bytes.NewBufferString(input)
	return AskDeps{
		ReadStdin: func(_ io.Reader, _ int64, _ bool) ([]byte, *jeq.Error) { return stdin.Bytes(), nil },
		ReadFile:  func(_ string, _ int64) ([]byte, *jeq.Error) { return []byte(questions), nil },
		NewClient: func(string, time.Duration, string, int, func(string)) APIClient { return client },
		Getenv: func(key string) string {
			if key == "TYPESAFE_API_KEY" {
				return "test-key"
			}
			return ""
		},
		Stdin: stdin,
	}, stdin
}

func TestMapNDJSONMakesOneRequestPerRecordAndPreservesOrder(t *testing.T) {
	client := &streamClient{}
	deps, _ := streamDeps(client, `{"state":"one"}
{"state":"two"}
`, `{"questions":{"q":{"type":"noul","instructions":"is it?"}}}`)
	var out, stderr bytes.Buffer
	code := RunWithDeps([]string{"map", "--as", "route", "--input", "ndjson", "--questions", "questions.json", "--model", "m"}, &out, &stderr, streamRenderer{}, deps)
	assert.Equal(t, 0, code, "stderr=%q", stderr.String())
	assert.Equal(t, 2, client.calls)
	assert.Equal(t, 2, strings.Count(out.String(), "\n"))
	assert.Empty(t, stderr.String())
	assert.Contains(t, out.String(), `"state":"one"`)
	assert.Contains(t, out.String(), `"state":"two"`)
}

func TestMapNativeRequestPointerUsesRecordRequest(t *testing.T) {
	client := &streamClient{}
	record := `{"request":{"state":"s","model":"native","questions":{"q":{"type":"noul","instructions":"is it?"}}},"id":1}`
	deps, _ := streamDeps(client, record, "")
	var out, stderr bytes.Buffer
	code := RunWithDeps([]string{"map", "--as", "native", "--request-pointer", "/request"}, &out, &stderr, streamRenderer{}, deps)
	assert.Equal(t, 0, code, "stderr=%q", stderr.String())
	assert.Equal(t, 1, client.calls)
	require.Len(t, client.requests, 1)
	assert.Equal(t, "native", client.requests[0].Model)
	assert.Empty(t, stderr.String())
}

func TestGateAggregateStatusDoesNotRenderSecondError(t *testing.T) {
	deps, _ := streamDeps(nil, `{"score":0.9}
{"score":0.1}
`, "")
	var out, stderr bytes.Buffer
	code := RunWithDeps([]string{"gate", "--as", "policy", "--input", "ndjson", "--value-pointer", "/score", "--pass-min", "0.8", "--reject-max", "0.2"}, &out, &stderr, streamRenderer{}, deps)
	assert.Equal(t, 10, code)
	assert.Equal(t, 2, strings.Count(out.String(), "\n"))
	assert.NotContains(t, out.String(), `"code"`)
}

func TestMapNDJSONFailureKeepsPriorOutputAndEmitsOneErrorLine(t *testing.T) {
	client := &streamClient{}
	deps, _ := streamDeps(client, `{"state":"ok"}
not-json
`, `{"questions":{"q":{"type":"noul","instructions":"is it?"}}}`)
	var out, stderr bytes.Buffer
	code := RunWithDeps([]string{"map", "--as", "route", "--input", "ndjson", "--questions", "questions.json", "--model", "m"}, &out, &stderr, streamRenderer{}, deps)
	assert.Equal(t, 2, code)
	assert.Equal(t, 1, client.calls)
	assert.Contains(t, stderr.String(), "JEQ_REQUEST_INVALID")
	assert.NotContains(t, out.String(), `"code"`)
}
