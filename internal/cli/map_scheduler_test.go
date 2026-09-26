package cli

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cristianoliveira/jeq/internal/domain/contract"
	"github.com/cristianoliveira/jeq/internal/domain/jeq"
	"github.com/cristianoliveira/jeq/internal/trace"
	"github.com/spf13/cobra"
)

type schedulerReader struct {
	mu    sync.Mutex
	lines []string
	reads int
}

func (r *schedulerReader) Read(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.lines) == 0 {
		return 0, io.EOF
	}
	line := r.lines[0]
	r.lines = r.lines[1:]
	r.reads++
	return copy(p, []byte(line)), nil
}

func (r *schedulerReader) Reads() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.reads
}

type schedulerEvaluator struct {
	started   chan string
	release   <-chan struct{}
	wait      map[string]bool
	fail      map[string]*jeq.Error
	completed chan<- string
	observer  trace.Observer

	mu     sync.Mutex
	calls  map[string]int
	active int
	max    int
}

func (e *schedulerEvaluator) Evaluate(ctx context.Context, request contract.Request) (contract.Response, *jeq.Error) {
	var id string
	if err := json.Unmarshal(request.State, &id); err != nil {
		return contract.Response{}, jeq.NewError(jeq.CodeRequestInvalid, err.Error())
	}
	e.mu.Lock()
	e.calls[id]++
	e.active++
	if e.active > e.max {
		e.max = e.active
	}
	e.mu.Unlock()
	defer func() {
		e.mu.Lock()
		e.active--
		e.mu.Unlock()
	}()

	e.started <- id
	if e.observer != nil {
		e.observer.Attempt(1, 429)
		e.observer.Retrying(1, 1, 429)
		e.observer.Attempt(2, 200)
	}
	if failure := e.fail[id]; failure != nil {
		return contract.Response{}, failure
	}
	if e.wait[id] {
		select {
		case <-e.release:
		case <-ctx.Done():
			return contract.Response{}, jeq.NewError(jeq.CodeInterrupted, "scheduler cancelled")
		}
	}
	if e.completed != nil {
		e.completed <- id
	}
	return contract.Response{Model: "m", Answers: map[string]contract.Answer{}}, nil
}

func (e *schedulerEvaluator) Calls() map[string]int {
	e.mu.Lock()
	defer e.mu.Unlock()
	calls := make(map[string]int, len(e.calls))
	for id, count := range e.calls {
		calls[id] = count
	}
	return calls
}

func (e *schedulerEvaluator) MaxActive() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.max
}

func schedulerRecords(count int) (first []byte, rest *schedulerReader) {
	first = []byte(`{"state":"0"}`)
	lines := make([]string, 0, count-1)
	for i := 1; i < count; i++ {
		lines = append(lines, fmt.Sprintf(`{"state":"%d"}
`, i))
	}
	return first, &schedulerReader{lines: lines}
}

func runScheduler(ctx context.Context, out io.Writer, reader *schedulerReader, first []byte, evaluator *schedulerEvaluator) <-chan error {
	done := make(chan error, 1)
	go func() {
		cmd := &cobra.Command{}
		cmd.SetOut(out)
		done <- processMapStream(ctx, cmd, streamRenderer{}, bufio.NewReader(reader), first,
			mapFlags{input: "ndjson", name: "risk", statePointer: "/state"},
			map[string]contract.Question{"q": {Type: contract.TypeNoul, Instructions: json.RawMessage(`"judge"`)}},
			nil, "m", evaluator)
	}()
	return done
}

func TestMapSchedulerRunsBoundedWorkersAndCommitsOrder(t *testing.T) {
	first, reader := schedulerRecords(8)
	release := make(chan struct{})
	evaluator := &schedulerEvaluator{started: make(chan string, 16), release: release, wait: map[string]bool{"0": true, "1": true, "2": true, "3": true}, calls: map[string]int{}}
	var out bytes.Buffer
	done := runScheduler(context.Background(), &out, reader, first, evaluator)
	for range 4 {
		<-evaluator.started
	}
	assert.Equal(t, 4, evaluator.MaxActive())
	assert.Equal(t, 3, reader.Reads(), "read-ahead records after first dispatch window")
	close(release)
	require.NoError(t, <-done)
	assert.Len(t, evaluator.Calls(), 8)
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	require.Len(t, lines, 8)
	for i, line := range lines {
		assert.Contains(t, line, fmt.Sprintf(`"state":"%d"`, i), "line %d", i)
	}
}

func TestMapSchedulerCommitsOutOfOrderSuccessInInputOrder(t *testing.T) {
	first, reader := schedulerRecords(4)
	release := make(chan struct{})
	completed := make(chan string, 4)
	evaluator := &schedulerEvaluator{started: make(chan string, 8), release: release, wait: map[string]bool{"0": true}, completed: completed, calls: map[string]int{}}
	var out bytes.Buffer
	done := runScheduler(context.Background(), &out, reader, first, evaluator)
	for range 4 {
		<-evaluator.started
	}
	<-completed
	close(release)
	require.NoError(t, <-done)
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	for i, line := range lines {
		assert.Contains(t, line, fmt.Sprintf(`"state":"%d"`, i), "line %d", i)
	}
}

func TestMapSchedulerLaterFailurePreservesPrefixWithinDispatchBound(t *testing.T) {
	first, reader := schedulerRecords(5)
	release := make(chan struct{})
	evaluator := &schedulerEvaluator{
		started: make(chan string, 16),
		release: release,
		wait:    map[string]bool{"0": true, "1": true},
		fail:    map[string]*jeq.Error{"2": jeq.NewError(jeq.CodeServerError, "later failure")},
		calls:   map[string]int{},
	}
	var out bytes.Buffer
	done := runScheduler(context.Background(), &out, reader, first, evaluator)
	for range 4 {
		<-evaluator.started
	}
	close(release)
	err := <-done
	var coded *jeq.Error
	require.True(t, errors.As(err, &coded))
	require.NotNil(t, coded)
	assert.Equal(t, jeq.CodeServerError, coded.Code)
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	require.Len(t, lines, 2)
	assert.Contains(t, lines[0], `"state":"0"`)
	assert.Contains(t, lines[1], `"state":"1"`)
	calls := evaluator.Calls()
	for id := 0; id < 4; id++ {
		assert.Equal(t, 1, calls[fmt.Sprint(id)])
	}
	// Job 3 can complete before job 2's failure reaches the coordinator, allowing
	// one speculative read-ahead and replacement dispatch for candidate 4.
	const initialReadAheadRecords = 3
	reads := reader.Reads()
	speculativeReadAhead := reads - initialReadAheadRecords
	replacementCalls := calls["4"]
	assert.LessOrEqual(t, replacementCalls, 1, "candidate 4 may be dispatched at most once")
	assert.GreaterOrEqual(t, reads, initialReadAheadRecords, "the initial dispatch must read candidates 1 through 3")
	assert.LessOrEqual(t, reads, initialReadAheadRecords+1, "failure may allow at most one speculative read-ahead")
	assert.Equal(t, replacementCalls, speculativeReadAhead, "each speculative read-ahead record must have one replacement call")
}

func TestMapSchedulerCancellationDrainsWorkersWithoutDispatchingMore(t *testing.T) {
	first, reader := schedulerRecords(8)
	release := make(chan struct{})
	evaluator := &schedulerEvaluator{started: make(chan string, 16), release: release, wait: map[string]bool{"0": true, "1": true, "2": true, "3": true}, calls: map[string]int{}}
	ctx, cancel := context.WithCancel(context.Background())
	var out bytes.Buffer
	done := runScheduler(ctx, &out, reader, first, evaluator)
	for range 4 {
		<-evaluator.started
	}
	cancel()
	err := <-done
	var coded *jeq.Error
	require.True(t, errors.As(err, &coded))
	require.NotNil(t, coded)
	assert.Equal(t, jeq.CodeInterrupted, coded.Code)
	assert.Len(t, evaluator.Calls(), 4)
	assert.Equal(t, 3, reader.Reads())
	assert.Empty(t, out.String())
}

type schedulerFailingRenderer struct{}

func (schedulerFailingRenderer) RenderSuccess(io.Writer, contract.Response) error {
	return errors.New("stdout failed")
}
func (schedulerFailingRenderer) RenderError(io.Writer, *jeq.Error) error { return nil }
func (schedulerFailingRenderer) RenderRaw(io.Writer, []byte) error {
	return errors.New("stdout failed")
}

func (schedulerFailingRenderer) RenderValue(io.Writer, any) error { return errors.New("stdout failed") }

func TestMapSchedulerOutputFailureStopsAtDispatchBound(t *testing.T) {
	first, reader := schedulerRecords(8)
	release := make(chan struct{})
	evaluator := &schedulerEvaluator{started: make(chan string, 16), release: release, wait: map[string]bool{"1": true, "2": true, "3": true}, calls: map[string]int{}}
	done := make(chan error, 1)
	go func() {
		cmd := &cobra.Command{}
		done <- processMapStream(context.Background(), cmd, schedulerFailingRenderer{}, bufio.NewReader(reader), first,
			mapFlags{input: "ndjson", name: "risk", statePointer: "/state"},
			map[string]contract.Question{"q": {Type: contract.TypeNoul, Instructions: json.RawMessage(`"judge"`)}}, nil, "m", evaluator)
	}()
	require.Error(t, <-done, "output failure was swallowed")
	assert.Len(t, evaluator.Calls(), mapWorkerLimit)
	assert.Equal(t, mapWorkerLimit-1, reader.Reads())
}

func TestMapSchedulerEmitsOneFinalTraceSummary(t *testing.T) {
	first, reader := schedulerRecords(4)
	evaluator := &schedulerEvaluator{started: make(chan string, 8), calls: map[string]int{}}
	var traceOut bytes.Buffer
	cfg := trace.New(true, "scheduler", &traceOut)
	evaluator.observer = cfg
	ctx := trace.WithContext(context.Background(), cfg)
	var out bytes.Buffer
	require.NoError(t, <-runScheduler(ctx, &out, reader, first, evaluator))
	var summaries []map[string]any
	attempts, retries := 0, 0
	for _, line := range strings.Split(strings.TrimSpace(traceOut.String()), "\n") {
		var event map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &event))
		switch event["event"] {
		case "request.attempted":
			attempts++
		case "request.retrying":
			retries++
		}
		if event["phase"] == "summary" {
			summaries = append(summaries, event)
		}
	}
	assert.Equal(t, 8, attempts, "trace=%s", traceOut.String())
	assert.Equal(t, 4, retries, "trace=%s", traceOut.String())
	require.Len(t, summaries, 1, "trace=%s", traceOut.String())
	assert.Equal(t, float64(4), summaries[0]["seen"])
	assert.Equal(t, float64(4), summaries[0]["succeeded"])
	assert.Equal(t, float64(4), summaries[0]["emitted"])
}
