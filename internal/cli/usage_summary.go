package cli

import (
	"context"
	"encoding/json"
	"io"
	"sort"
	"sync"
	"time"

	"github.com/cristianoliveira/jeq/internal/domain/contract"
	"github.com/cristianoliveira/jeq/internal/domain/jeq"
	"github.com/cristianoliveira/jeq/internal/domain/pipeline"
	"github.com/cristianoliveira/jeq/internal/trace"
)

const usageSummarySchema = "jeq.usage.v1"

type usageSummaryKey struct{}

type usageSummary struct {
	mu                 sync.Mutex
	command            string
	startedAt          time.Time
	attemptedRequests  int64
	successfulRequests int64
	processedRecords   int64
	decodedAnswers     int64
	inputTokens        int64
	outputTokens       int64
	modelVersions      map[string]struct{}
}

type usageSummaryDocument struct {
	Schema                    string   `json:"schema"`
	Command                   string   `json:"command"`
	AttemptedRequests         int64    `json:"attempted_requests"`
	SuccessfulRequests        int64    `json:"successful_requests"`
	ProcessedRecords          int64    `json:"processed_records"`
	DecodedAnswers            int64    `json:"decoded_answers"`
	InputTokens               int64    `json:"input_tokens"`
	OutputTokens              int64    `json:"output_tokens"`
	AnswersPer1000InputTokens *float64 `json:"answers_per_1000_input_tokens"`
	ElapsedMilliseconds       int64    `json:"elapsed_ms"`
	ResolvedModelVersions     []string `json:"resolved_model_versions"`
}

func withUsageSummary(ctx context.Context, summary *usageSummary) context.Context {
	return context.WithValue(ctx, usageSummaryKey{}, summary)
}

func usageSummaryFrom(ctx context.Context) *usageSummary {
	summary, _ := ctx.Value(usageSummaryKey{}).(*usageSummary)
	return summary
}

func newUsageSummary(command string, now time.Time) *usageSummary {
	return &usageSummary{command: command, startedAt: now, modelVersions: make(map[string]struct{})}
}

func isUsageSummaryCommand(name string) bool {
	switch name {
	case "ask", "map", "rate", "rank", "reduce":
		return true
	default:
		return false
	}
}

func (s *usageSummary) Attempt(_, _ int) {
	s.mu.Lock()
	s.attemptedRequests++
	s.mu.Unlock()
}

func (*usageSummary) Retrying(_, _, _ int) {}

func (s *usageSummary) recordResponse(response contract.Response) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.successfulRequests++
	s.inputTokens += response.Usage.InputTokens
	s.outputTokens += response.Usage.OutputTokens
	if response.Model != "" {
		s.modelVersions[response.Model] = struct{}{}
	}
}

func (s *usageSummary) addProcessedRecords(count int64) {
	s.mu.Lock()
	s.processedRecords += count
	s.mu.Unlock()
}

func (s *usageSummary) addDecodedAnswers(count int64) {
	s.mu.Lock()
	s.decodedAnswers += count
	s.mu.Unlock()
}

func (s *usageSummary) write(out io.Writer, finishedAt time.Time) {
	if s == nil || out == nil {
		return
	}
	s.mu.Lock()
	document := usageSummaryDocument{
		Schema: usageSummarySchema, Command: s.command,
		AttemptedRequests: s.attemptedRequests, SuccessfulRequests: s.successfulRequests,
		ProcessedRecords: s.processedRecords, DecodedAnswers: s.decodedAnswers,
		InputTokens: s.inputTokens, OutputTokens: s.outputTokens,
		ElapsedMilliseconds: finishedAt.Sub(s.startedAt).Milliseconds(),
	}
	if document.ElapsedMilliseconds < 0 {
		document.ElapsedMilliseconds = 0
	}
	for version := range s.modelVersions {
		document.ResolvedModelVersions = append(document.ResolvedModelVersions, version)
	}
	s.mu.Unlock()
	sort.Strings(document.ResolvedModelVersions)
	if document.InputTokens > 0 {
		ratio := float64(document.DecodedAnswers) * 1000 / float64(document.InputTokens)
		document.AnswersPer1000InputTokens = &ratio
	}
	encoded, err := json.Marshal(document)
	if err == nil {
		_, _ = out.Write(append(encoded, '\n'))
	}
}

type combinedTraceObserver []trace.Observer

func (observers combinedTraceObserver) Attempt(attempt, status int) {
	for _, observer := range observers {
		observer.Attempt(attempt, status)
	}
}

func (observers combinedTraceObserver) Retrying(attempt, budget, status int) {
	for _, observer := range observers {
		observer.Retrying(attempt, budget, status)
	}
}

type usageEvaluator struct {
	delegate pipeline.Evaluator
	summary  *usageSummary
	response contract.Response
	observed bool
}

func newUsageEvaluator(ctx context.Context, evaluator pipeline.Evaluator) *usageEvaluator {
	return &usageEvaluator{delegate: evaluator, summary: usageSummaryFrom(ctx)}
}

func (e *usageEvaluator) Evaluate(ctx context.Context, request contract.Request) (contract.Response, *jeq.Error) {
	response, err := e.delegate.Evaluate(ctx, request)
	if err == nil && e.summary != nil {
		e.summary.recordResponse(response)
		e.response = response
		e.observed = true
	}
	return response, err
}

func (e *usageEvaluator) recordAttachedAnswers() {
	if e.summary != nil && e.observed {
		e.summary.addDecodedAnswers(int64(len(e.response.Answers)))
	}
	e.response = contract.Response{}
	e.observed = false
}
