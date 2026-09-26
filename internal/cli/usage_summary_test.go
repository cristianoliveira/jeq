package cli_test

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

	"github.com/cristianoliveira/jeq/internal/cli"
	"github.com/cristianoliveira/jeq/internal/domain/contract"
	"github.com/cristianoliveira/jeq/internal/domain/jeq"
	"github.com/cristianoliveira/jeq/internal/trace"
)

type usageClient struct {
	mu       sync.Mutex
	observer trace.Observer
	statuses []int
	calls    int
	evaluate func(contract.Request) (contract.Response, *jeq.Error)
}

func (c *usageClient) SetTraceObserver(observer trace.Observer) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.observer = observer
}

func (c *usageClient) Evaluate(_ context.Context, request contract.Request) (contract.Response, *jeq.Error) {
	c.mu.Lock()
	c.calls++
	observer := c.observer
	statuses := append([]int(nil), c.statuses...)
	c.mu.Unlock()
	if len(statuses) == 0 {
		statuses = []int{200}
	}
	for attempt, status := range statuses {
		if observer != nil {
			observer.Attempt(attempt+1, status)
		}
	}
	return c.evaluate(request)
}

func (c *usageClient) Models(context.Context) (contract.Models, *jeq.Error) {
	return contract.Models{}, jeq.NewError(jeq.CodeResponseInvalid, "unexpected models call")
}

func (c *usageClient) callCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.calls
}

type usageRenderer struct{}

func (usageRenderer) RenderSuccess(writer io.Writer, response contract.Response) error {
	encoded, err := response.Encode()
	if err != nil {
		return err
	}
	_, err = writer.Write(append(encoded, '\n'))
	return err
}

func (usageRenderer) RenderValue(writer io.Writer, value any) error {
	encoded, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = writer.Write(append(encoded, '\n'))
	return err
}

func (usageRenderer) RenderRaw(writer io.Writer, raw []byte) error {
	_, err := writer.Write(append(raw, '\n'))
	return err
}

type usageSummaryDocument struct {
	Schema                   string   `json:"schema"`
	Command                  string   `json:"command"`
	AttemptedRequests        int64    `json:"attempted_requests"`
	SuccessfulRequests       int64    `json:"successful_requests"`
	ProcessedRecords         int64    `json:"processed_records"`
	DecodedAnswers           int64    `json:"decoded_answers"`
	InputTokens              int64    `json:"input_tokens"`
	OutputTokens             int64    `json:"output_tokens"`
	AnswersPer1000InputToken *float64 `json:"answers_per_1000_input_tokens"`
	ElapsedMilliseconds      int64    `json:"elapsed_ms"`
	ModelVersions            []string `json:"resolved_model_versions"`
}

func runUsageCommand(t *testing.T, args []string, input string, client *usageClient) (int, string, string) {
	t.Helper()
	stdin := strings.NewReader(input)
	deps := cli.AskDeps{
		ReadFile: func(string, int64) ([]byte, *jeq.Error) {
			return nil, jeq.NewError(jeq.CodeInputInvalid, "unexpected file read")
		},
		ReadStdin: func(reader io.Reader, limit int64, forbidEmpty bool) ([]byte, *jeq.Error) {
			data, err := io.ReadAll(io.LimitReader(reader, limit))
			if err != nil {
				return nil, jeq.WrapError(jeq.CodeInputInvalid, err, "reading test input")
			}
			if forbidEmpty && len(bytes.TrimSpace(data)) == 0 {
				return nil, jeq.NewError(jeq.CodeInputInvalid, "empty test input")
			}
			return data, nil
		},
		NewClient: func(string, time.Duration, string, int, func(string)) cli.APIClient { return client },
		Getenv: func(key string) string {
			if key == "TYPESAFE_API_KEY" {
				return "API_SECRET"
			}
			return ""
		},
		Stdin: stdin,
	}
	var stdout, stderr bytes.Buffer
	code := cli.RunWithDeps(args, &stdout, &stderr, usageRenderer{}, deps)
	return code, stdout.String(), stderr.String()
}

func parseUsageSummary(t *testing.T, stderr string) usageSummaryDocument {
	t.Helper()
	var summary usageSummaryDocument
	lines := strings.Split(strings.TrimSpace(stderr), "\n")
	require.NotEmpty(t, lines)
	assert.Contains(t, lines[len(lines)-1], `"schema":"jeq.usage.v1"`, "final stderr line must be the machine-readable usage summary")
	require.NoError(t, json.Unmarshal([]byte(lines[len(lines)-1]), &summary))
	return summary
}

func usageAnswer(name string, kind contract.QuestionType) contract.Answer {
	switch kind {
	case contract.TypeChoice:
		choice := name
		return contract.Answer{Type: kind, Choice: &choice, Confidence: floatPointer(0.8), Probs: map[string]float64{name: 1}}
	case contract.TypeScore:
		return contract.Answer{Type: kind, Score: floatPointer(0.8), Legend: map[string]string{"high": "High"}, Confidence: floatPointer(0.8)}
	default:
		return contract.Answer{Type: contract.TypeNoul, Noul: floatPointer(0.8), Confidence: floatPointer(0.8)}
	}
}

func floatPointer(value float64) *float64 { return &value }

func stringPointer(value string) *string { return &value }

func responseWithUsage(model string, input, output int64, answers map[string]contract.Answer) contract.Response {
	return contract.Response{Model: model, Usage: contract.Usage{InputTokens: input, OutputTokens: output}, Answers: answers}
}

func TestUsageSummaryIsOptInAndKeepsAskStdoutUnchanged(t *testing.T) {
	questions := `{"questions":{"first":{"type":"noul","instructions":"PRIVATE_PROMPT"},"second":{"type":"noul","instructions":"second"}}}`
	state := `{"note":"PRIVATE_STATE"}`
	makeClient := func() *usageClient {
		return &usageClient{statuses: []int{429, 200}, evaluate: func(contract.Request) (contract.Response, *jeq.Error) {
			return responseWithUsage("jev-1.13.0", 0, 7, map[string]contract.Answer{"first": usageAnswer("first", contract.TypeNoul), "second": usageAnswer("second", contract.TypeNoul)}), nil
		}}
	}
	baseArgs := []string{"ask", "--questions-json", questions, "--state-json", state, "--model", "jev-latest"}
	plainCode, plainOut, plainErr := runUsageCommand(t, baseArgs, "", makeClient())
	measuredCode, measuredOut, measuredErr := runUsageCommand(t, append(baseArgs, "--usage-summary"), "", makeClient())
	assert.Equal(t, 0, plainCode)
	assert.Equal(t, plainCode, measuredCode)
	assert.Equal(t, plainOut, measuredOut, "usage opt-in must not change ask stdout")
	assert.Empty(t, plainErr, "summary is opt-in")
	summary := parseUsageSummary(t, measuredErr)
	assert.Equal(t, "jeq ask", summary.Command)
	assert.Equal(t, int64(2), summary.AttemptedRequests)
	assert.Equal(t, int64(1), summary.SuccessfulRequests)
	assert.Equal(t, int64(1), summary.ProcessedRecords)
	assert.Equal(t, int64(2), summary.DecodedAnswers)
	assert.Zero(t, summary.InputTokens)
	assert.Equal(t, int64(7), summary.OutputTokens)
	assert.Nil(t, summary.AnswersPer1000InputToken)
	assert.GreaterOrEqual(t, summary.ElapsedMilliseconds, int64(0))
	assert.Equal(t, []string{"jev-1.13.0"}, summary.ModelVersions)
	assert.NotContains(t, measuredErr, "PRIVATE_PROMPT")
	assert.NotContains(t, measuredErr, "PRIVATE_STATE")
	assert.NotContains(t, measuredErr, "TYPESAFE_API_KEY")
	assert.NotContains(t, measuredErr, "API_SECRET")
}

func TestUsageSummaryCountsMapResponsesAcrossOrderedPrefixFailure(t *testing.T) {
	input := "{\"id\":0,\"state\":\"zero\"}\n{\"id\":1,\"state\":\"one\"}\n{\"id\":2,\"state\":\"two\"}\n{\"id\":3,\"state\":\"three\"}\n"
	laterCompleted := make(chan struct{}, 3)
	client := &usageClient{evaluate: func(request contract.Request) (contract.Response, *jeq.Error) {
		var state string
		if err := json.Unmarshal(request.State, &state); err != nil {
			return contract.Response{}, jeq.WrapError(jeq.CodeRequestInvalid, err, "test state")
		}
		if state == "zero" {
			for i := 0; i < 3; i++ {
				<-laterCompleted
			}
			return contract.Response{}, jeq.NewError(jeq.CodeResponseInvalid, "first record failed")
		}
		defer func() { laterCompleted <- struct{}{} }()
		inputTokens := int64(5)
		if state == "two" {
			inputTokens = 0
		}
		return responseWithUsage("jev-"+state, inputTokens, 2, map[string]contract.Answer{"risk": usageAnswer("risk", contract.TypeNoul)}), nil
	}}
	args := []string{"map", "--as", "risk", "--input", "ndjson", "--state-pointer", "/state", "--questions-json", `{"questions":{"risk":{"type":"noul","instructions":"risk"}}}`, "--model", "jev-latest", "--usage-summary"}
	code, stdout, stderr := runUsageCommand(t, args, input, client)
	assert.Equal(t, 1, code)
	assert.Empty(t, stdout)
	assert.Equal(t, 4, client.callCount())
	summary := parseUsageSummary(t, stderr)
	assert.Equal(t, "jeq map", summary.Command)
	assert.Equal(t, int64(4), summary.AttemptedRequests)
	assert.Equal(t, int64(3), summary.SuccessfulRequests)
	assert.Equal(t, int64(4), summary.ProcessedRecords)
	assert.Equal(t, int64(3), summary.DecodedAnswers)
	assert.Equal(t, int64(10), summary.InputTokens)
	assert.Equal(t, int64(6), summary.OutputTokens)
	require.NotNil(t, summary.AnswersPer1000InputToken)
	assert.Equal(t, float64(300), *summary.AnswersPer1000InputToken)
	assert.Equal(t, []string{"jev-one", "jev-three", "jev-two"}, summary.ModelVersions)
}

func TestUsageSummaryCoversRateRankAndReduce(t *testing.T) {
	question := `{"questions":{"risk":{"type":"noul","instructions":"risk"}}}`
	cases := []struct {
		name, input string
		args        []string
		response    contract.Response
		processed   int64
		requests    int64
		answers     int64
	}{
		{
			name: "rate", input: "{\"state\":\"one\"}\n{\"state\":\"two\"}\n",
			args:     []string{"rate", "--as", "risk", "--input", "ndjson", "--state-pointer", "/state", "--instruction", "risk", "--level", "low", "--level", "high", "--model", "jev-latest", "--usage-summary"},
			response: responseWithUsage("jev-score", 4, 2, map[string]contract.Answer{"risk": usageAnswer("risk", contract.TypeScore)}), processed: 2, requests: 2, answers: 2,
		},
		{
			name: "rank", input: "{\"id\":\"a\",\"criteria\":\"A\"}\n{\"id\":\"b\",\"criteria\":\"B\"}\n",
			args:     []string{"rank", "--as", "route", "--input", "ndjson", "--state", "request", "--instruction", "route", "--id-pointer", "/id", "--criteria-pointer", "/criteria", "--usage-summary"},
			response: responseWithUsage("jev-choice", 8, 3, map[string]contract.Answer{"route": {Type: contract.TypeChoice, Choice: stringPointer("b"), Confidence: floatPointer(0.8), Probs: map[string]float64{"a": 0.2, "b": 0.8}}}), processed: 2, requests: 1, answers: 1,
		},
		{
			name: "reduce", input: "[{\"id\":1},{\"id\":2},{\"id\":3}]",
			args:     []string{"reduce", "--as", "coherent", "--questions-json", question, "--model", "jev-latest", "--usage-summary"},
			response: responseWithUsage("jev-reduce", 12, 5, map[string]contract.Answer{"risk": usageAnswer("risk", contract.TypeNoul)}), processed: 3, requests: 1, answers: 1,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client := &usageClient{evaluate: func(contract.Request) (contract.Response, *jeq.Error) { return tc.response, nil }}
			code, _, stderr := runUsageCommand(t, tc.args, tc.input, client)
			assert.Equal(t, 0, code, "stderr=%q", stderr)
			assert.Equal(t, tc.requests, int64(client.callCount()))
			summary := parseUsageSummary(t, stderr)
			assert.Equal(t, tc.requests, summary.AttemptedRequests)
			assert.Equal(t, tc.requests, summary.SuccessfulRequests)
			assert.Equal(t, tc.processed, summary.ProcessedRecords)
			assert.Equal(t, tc.answers, summary.DecodedAnswers)
			assert.Equal(t, tc.response.Usage.InputTokens*tc.requests, summary.InputTokens)
			assert.Equal(t, tc.response.Usage.OutputTokens*tc.requests, summary.OutputTokens)
			assert.NotNil(t, summary.AnswersPer1000InputToken)
		})
	}
}

func TestUsageSummaryFlagIsDocumentedAndIgnoredForNonPaidCommands(t *testing.T) {
	client := &usageClient{evaluate: func(contract.Request) (contract.Response, *jeq.Error) {
		return contract.Response{}, jeq.NewError(jeq.CodeResponseInvalid, "unexpected evaluation")
	}}
	code, help, stderr := runUsageCommand(t, []string{"ask", "--help"}, "", client)
	assert.Equal(t, 0, code)
	assert.Contains(t, help, "--usage-summary")
	assert.Empty(t, stderr)
	code, output, stderr := runUsageCommand(t, []string{"examples", "--usage-summary"}, "", client)
	assert.Equal(t, 0, code)
	assert.NotEmpty(t, output)
	assert.Empty(t, stderr)
	assert.Zero(t, client.callCount())
}

func TestUsageSummaryExcludesFailedAndMalformedResponses(t *testing.T) {
	for _, tc := range []struct {
		name     string
		err      *jeq.Error
		wantCode int
	}{
		{name: "malformed", err: jeq.NewError(jeq.CodeResponseInvalid, "malformed provider response"), wantCode: 1},
		{name: "cancelled", err: jeq.NewError(jeq.CodeInterrupted, "request cancelled"), wantCode: 130},
	} {
		t.Run(tc.name, func(t *testing.T) {
			newClient := func() *usageClient {
				return &usageClient{evaluate: func(contract.Request) (contract.Response, *jeq.Error) { return contract.Response{}, tc.err }}
			}
			args := []string{"ask", "--questions-json", `{"questions":{"risk":{"type":"noul","instructions":"risk"}}}`, "--state", "private", "--model", "jev-latest"}
			plainCode, _, plainStderr := runUsageCommand(t, args, "", newClient())
			code, _, stderr := runUsageCommand(t, append(args, "--usage-summary"), "", newClient())
			assert.Equal(t, tc.wantCode, plainCode, "plain stderr=%q", plainStderr)
			assert.Equal(t, plainCode, code, "summary stderr=%q", stderr)
			summary := parseUsageSummary(t, stderr)
			assert.Equal(t, int64(1), summary.AttemptedRequests)
			assert.Zero(t, summary.SuccessfulRequests)
			assert.Zero(t, summary.DecodedAnswers)
			assert.Zero(t, summary.InputTokens)
			assert.Zero(t, summary.OutputTokens)
			assert.Nil(t, summary.AnswersPer1000InputToken, "failed response must not contribute usage")
		})
	}
}
