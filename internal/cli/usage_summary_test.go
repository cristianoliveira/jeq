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
	if len(lines) == 0 || !strings.Contains(lines[len(lines)-1], `"schema":"jeq.usage.v1"`) {
		t.Fatalf("final stderr line is not the machine-readable usage summary: %q", stderr)
	}
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &summary); err != nil {
		t.Fatalf("decode usage summary: %v", err)
	}
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
	if plainCode != 0 || measuredCode != plainCode || measuredOut != plainOut {
		t.Fatalf("opt-in changed ask result: codes=%d/%d stdout-equal=%t", plainCode, measuredCode, measuredOut == plainOut)
	}
	if plainErr != "" {
		t.Fatalf("summary was not opt-in: stderr=%q", plainErr)
	}
	summary := parseUsageSummary(t, measuredErr)
	if summary.Command != "jeq ask" || summary.AttemptedRequests != 2 || summary.SuccessfulRequests != 1 || summary.ProcessedRecords != 1 || summary.DecodedAnswers != 2 || summary.InputTokens != 0 || summary.OutputTokens != 7 || summary.AnswersPer1000InputToken != nil || summary.ElapsedMilliseconds < 0 {
		t.Fatalf("unexpected summary: %+v", summary)
	}
	if len(summary.ModelVersions) != 1 || summary.ModelVersions[0] != "jev-1.13.0" {
		t.Fatalf("model versions = %#v", summary.ModelVersions)
	}
	if strings.Contains(measuredErr, "PRIVATE_PROMPT") || strings.Contains(measuredErr, "PRIVATE_STATE") || strings.Contains(measuredErr, "TYPESAFE_API_KEY") || strings.Contains(measuredErr, "API_SECRET") {
		t.Fatalf("summary leaked request or credential material: %q", measuredErr)
	}
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
	if code != 1 || stdout != "" || client.callCount() != 4 {
		t.Fatalf("code=%d calls=%d stdout=%q stderr=%q", code, client.callCount(), stdout, stderr)
	}
	summary := parseUsageSummary(t, stderr)
	if summary.Command != "jeq map" || summary.AttemptedRequests != 4 || summary.SuccessfulRequests != 3 || summary.ProcessedRecords != 4 || summary.DecodedAnswers != 3 || summary.InputTokens != 10 || summary.OutputTokens != 6 || summary.AnswersPer1000InputToken == nil || *summary.AnswersPer1000InputToken != 300 {
		t.Fatalf("unexpected summary: %+v", summary)
	}
	if strings.Join(summary.ModelVersions, ",") != "jev-one,jev-three,jev-two" {
		t.Fatalf("model versions = %#v", summary.ModelVersions)
	}
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
			if code != 0 || int64(client.callCount()) != tc.requests {
				t.Fatalf("code=%d calls=%d stderr=%q", code, client.callCount(), stderr)
			}
			summary := parseUsageSummary(t, stderr)
			if summary.AttemptedRequests != tc.requests || summary.SuccessfulRequests != tc.requests || summary.ProcessedRecords != tc.processed || summary.DecodedAnswers != tc.answers || summary.InputTokens != tc.response.Usage.InputTokens*tc.requests || summary.OutputTokens != tc.response.Usage.OutputTokens*tc.requests || summary.AnswersPer1000InputToken == nil {
				t.Fatalf("unexpected summary: %+v", summary)
			}
		})
	}
}

func TestUsageSummaryFlagIsDocumentedAndIgnoredForNonPaidCommands(t *testing.T) {
	client := &usageClient{evaluate: func(contract.Request) (contract.Response, *jeq.Error) {
		return contract.Response{}, jeq.NewError(jeq.CodeResponseInvalid, "unexpected evaluation")
	}}
	code, help, stderr := runUsageCommand(t, []string{"ask", "--help"}, "", client)
	if code != 0 || !strings.Contains(help, "--usage-summary") || stderr != "" {
		t.Fatalf("help code=%d flag-present=%t stderr=%q", code, strings.Contains(help, "--usage-summary"), stderr)
	}
	code, output, stderr := runUsageCommand(t, []string{"examples", "--usage-summary"}, "", client)
	if code != 0 || output == "" || stderr != "" || client.callCount() != 0 {
		t.Fatalf("non-paid command code=%d calls=%d stdout-empty=%t stderr=%q", code, client.callCount(), output == "", stderr)
	}
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
			if plainCode != tc.wantCode || code != plainCode {
				t.Fatalf("exit codes plain=%d summary=%d, want %d; stderr=%q / %q", plainCode, code, tc.wantCode, plainStderr, stderr)
			}
			summary := parseUsageSummary(t, stderr)
			if summary.AttemptedRequests != 1 || summary.SuccessfulRequests != 0 || summary.DecodedAnswers != 0 || summary.InputTokens != 0 || summary.OutputTokens != 0 || summary.AnswersPer1000InputToken != nil {
				t.Fatalf("failed response contributed usage: %+v", summary)
			}
		})
	}
}
