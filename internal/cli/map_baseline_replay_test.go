package cli_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianoliveira/jeq/internal/domain/contract"
	"github.com/cristianoliveira/jeq/internal/domain/jeq"
)

type baselineReplayManifest struct {
	FixtureSHA256 string                   `json:"fixture_sha256"`
	Model         string                   `json:"model"`
	Repetitions   int                      `json:"repetitions"`
	Workloads     []baselineReplayWorkload `json:"workloads"`
}

type baselineReplayWorkload struct {
	ID                        string  `json:"id"`
	Requests                  int     `json:"requests"`
	InputTokens               int64   `json:"input_tokens"`
	OutputTokens              int64   `json:"output_tokens"`
	AttachedAnswers           int64   `json:"attached_answers"`
	AnswersPer1000InputTokens float64 `json:"answers_per_1000_input_tokens"`
	RequestHashesByRecord     []struct {
		RecordID string `json:"record_id"`
		SHA256   string `json:"sha256"`
	} `json:"request_hashes_by_record"`
}

type mapShapeFixtures struct {
	Model     string             `json:"model"`
	Workloads []mapShapeWorkload `json:"workloads"`
}

type mapShapeWorkload struct {
	ID              string                     `json:"id"`
	Questions       map[string]json.RawMessage `json:"questions"`
	SharedReference json.RawMessage            `json:"shared_reference"`
	Records         []mapShapeRecord           `json:"records"`
}

type mapShapeRecord struct {
	ID    string          `json:"id"`
	State json.RawMessage `json:"state"`
}

func TestMapUsageSummaryReplaysPerRecordBaselineWithoutProviderCalls(t *testing.T) {
	fixturePath := filepath.Join("..", "..", "examples", "map-shapes-evaluation", "fixtures.json")
	manifestPath := filepath.Join("..", "..", "examples", "map-shapes-evaluation", "per-record-baseline-replay.json")
	fixtures, fixtureBytes := loadExpandedMapFixtures(t, fixturePath)
	manifestBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read sanitized replay manifest: %v", err)
	}
	var manifest baselineReplayManifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		t.Fatalf("decode sanitized replay manifest: %v", err)
	}
	fixtureDigest := sha256.Sum256(fixtureBytes)
	if manifest.FixtureSHA256 != hex.EncodeToString(fixtureDigest[:]) || manifest.Model != fixtures.Model {
		t.Fatalf("replay manifest does not match pinned fixtures/model")
	}
	if len(manifest.Workloads) != len(fixtures.Workloads) {
		t.Fatalf("manifest workload count=%d fixture workload count=%d", len(manifest.Workloads), len(fixtures.Workloads))
	}

	for workloadIndex, workload := range fixtures.Workloads {
		t.Run(workload.ID, func(t *testing.T) {
			replay := manifest.Workloads[workloadIndex]
			if replay.ID != workload.ID || replay.Requests != len(workload.Records)*manifest.Repetitions {
				t.Fatalf("replay metadata does not match workload %q", workload.ID)
			}
			questionDoc := providerQuestionDocument(t, workload.Questions)
			perRequestInputs := splitAggregateTokens(replay.InputTokens, replay.Requests)
			perRequestOutputs := splitAggregateTokens(replay.OutputTokens, replay.Requests)
			wantHashes := make(map[string]string, len(replay.RequestHashesByRecord))
			for _, item := range replay.RequestHashesByRecord {
				wantHashes[item.RecordID] = item.SHA256
			}

			var aggregate usageSummaryDocument
			callIndex := 0
			for repetition := 0; repetition < manifest.Repetitions; repetition++ {
				for _, record := range workload.Records {
					usageInput := perRequestInputs[callIndex]
					usageOutput := perRequestOutputs[callIndex]
					callIndex++
					var captured contract.Request
					client := &usageClient{evaluate: func(request contract.Request) (contract.Response, *jeq.Error) {
						captured = request
						answers := make(map[string]contract.Answer, len(request.Questions))
						for questionID, question := range request.Questions {
							answers[questionID] = usageAnswer(questionID, question.Type)
						}
						return responseWithUsage(fixtures.Model, usageInput, usageOutput, answers), nil
					}}
					input := perRecordReplayInput(t, workload, record)
					args := []string{
						"map", "--as", "baseline", "--input", "json",
						"--state-pointer", "/state", "--questions-json", string(questionDoc),
						"--model", fixtures.Model, "--usage-summary",
					}
					code, _, stderr := runUsageCommand(t, args, string(input), client)
					if code != 0 {
						t.Fatalf("production map replay failed with exit %d: %q", code, stderr)
					}
					summary := parseUsageSummary(t, stderr)
					if client.callCount() != 1 || summary.AttemptedRequests != 1 || summary.SuccessfulRequests != 1 || summary.ProcessedRecords != 1 || summary.DecodedAnswers != int64(len(workload.Questions)) || summary.InputTokens != usageInput || summary.OutputTokens != usageOutput {
						t.Fatalf("production map summary differs from fake response: %+v", summary)
					}
					encoded, err := captured.Encode()
					if err != nil {
						t.Fatalf("encode captured request: %v", err)
					}
					digest := sha256.Sum256(encoded)
					if got, want := hex.EncodeToString(digest[:]), wantHashes[record.ID]; got != want {
						t.Fatalf("request hash for %s repetition %d = %s, want planner hash %s", record.ID, repetition+1, got, want)
					}
					if strings.Contains(string(encoded), `"expected"`) {
						t.Fatalf("expected labels leaked into production request for %s", record.ID)
					}
					aggregate.AttemptedRequests += summary.AttemptedRequests
					aggregate.SuccessfulRequests += summary.SuccessfulRequests
					aggregate.ProcessedRecords += summary.ProcessedRecords
					aggregate.DecodedAnswers += summary.DecodedAnswers
					aggregate.InputTokens += summary.InputTokens
					aggregate.OutputTokens += summary.OutputTokens
				}
			}

			if aggregate.AttemptedRequests != int64(replay.Requests) || aggregate.SuccessfulRequests != int64(replay.Requests) || aggregate.ProcessedRecords != int64(replay.Requests) || aggregate.DecodedAnswers != replay.AttachedAnswers || aggregate.InputTokens != replay.InputTokens || aggregate.OutputTokens != replay.OutputTokens {
				t.Fatalf("production --usage-summary does not match deterministic replay aggregate: %+v", aggregate)
			}
			wantRatio := float64(replay.AttachedAnswers) * 1000 / float64(replay.InputTokens)
			gotRatio := float64(aggregate.DecodedAnswers) * 1000 / float64(aggregate.InputTokens)
			if gotRatio != wantRatio {
				t.Fatalf("answers per 1,000 tokens=%v want=%v", gotRatio, wantRatio)
			}
			if math.Abs(gotRatio-replay.AnswersPer1000InputTokens) >= 0.0000005 {
				t.Fatalf("manifest answers per 1,000 tokens=%v want=%v", replay.AnswersPer1000InputTokens, gotRatio)
			}
		})
	}
}

func loadExpandedMapFixtures(t *testing.T, path string) (mapShapeFixtures, []byte) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read map-shape fixtures: %v", err)
	}
	var generic any
	if err := json.Unmarshal(raw, &generic); err != nil {
		t.Fatalf("decode map-shape fixtures: %v", err)
	}
	expanded, err := json.Marshal(expandMapFixtureValue(generic))
	if err != nil {
		t.Fatalf("expand map-shape fixtures: %v", err)
	}
	var fixtures mapShapeFixtures
	if err := json.Unmarshal(expanded, &fixtures); err != nil {
		t.Fatalf("decode expanded map-shape fixtures: %v", err)
	}
	return fixtures, raw
}

func expandMapFixtureValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		text, hasText := typed["repeat_text"].(string)
		countValue, hasCount := typed["repeat_count"].(float64)
		if len(typed) == 2 && hasText && hasCount {
			return strings.Repeat(text, int(countValue))
		}
		for key, child := range typed {
			typed[key] = expandMapFixtureValue(child)
		}
		return typed
	case []any:
		for index, child := range typed {
			typed[index] = expandMapFixtureValue(child)
		}
		return typed
	default:
		return value
	}
}

func providerQuestionDocument(t *testing.T, source map[string]json.RawMessage) []byte {
	t.Helper()
	questions := make(map[string]json.RawMessage, len(source))
	for questionID, raw := range source {
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(raw, &fields); err != nil {
			t.Fatalf("decode fixture question %q: %v", questionID, err)
		}
		delete(fields, "threshold")
		encoded, err := json.Marshal(fields)
		if err != nil {
			t.Fatalf("encode provider question %q: %v", questionID, err)
		}
		questions[questionID] = encoded
	}
	document, err := json.Marshal(map[string]any{"questions": questions})
	if err != nil {
		t.Fatalf("encode provider question document: %v", err)
	}
	return document
}

func perRecordReplayInput(t *testing.T, workload mapShapeWorkload, record mapShapeRecord) []byte {
	t.Helper()
	state := record.State
	if len(workload.SharedReference) > 0 {
		var shared, content any
		if err := json.Unmarshal(workload.SharedReference, &shared); err != nil {
			t.Fatalf("decode shared reference: %v", err)
		}
		if err := json.Unmarshal(record.State, &content); err != nil {
			t.Fatalf("decode record state: %v", err)
		}
		var err error
		state, err = json.Marshal(map[string]any{"shared_reference": shared, "record": content})
		if err != nil {
			t.Fatalf("encode selected state: %v", err)
		}
	}
	encoded, err := json.Marshal(map[string]any{"id": record.ID, "state": state})
	if err != nil {
		t.Fatalf("encode map input: %v", err)
	}
	return encoded
}

func splitAggregateTokens(total int64, count int) []int64 {
	values := make([]int64, count)
	base, remainder := total/int64(count), int(total%int64(count))
	for index := range values {
		values[index] = base
		if index < remainder {
			values[index]++
		}
	}
	return values
}
