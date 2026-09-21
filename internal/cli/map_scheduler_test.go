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
	if got := evaluator.MaxActive(); got != 4 {
		t.Fatalf("max active=%d, want 4", got)
	}
	if got := reader.Reads(); got != 3 {
		t.Fatalf("read-ahead=%d records, want 3 after first dispatch window", got)
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if got := evaluator.Calls(); len(got) != 8 {
		t.Fatalf("calls=%v", got)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 8 {
		t.Fatalf("output lines=%d, want 8", len(lines))
	}
	for i, line := range lines {
		if !strings.Contains(line, fmt.Sprintf(`"state":"%d"`, i)) {
			t.Fatalf("line %d=%s", i, line)
		}
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
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	for i, line := range lines {
		if !strings.Contains(line, fmt.Sprintf(`"state":"%d"`, i)) {
			t.Fatalf("line %d=%s", i, line)
		}
	}
}

func TestMapSchedulerLaterFailurePreservesPrefixAndStopsDispatch(t *testing.T) {
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
	if !errors.As(err, &coded) || coded.Code != jeq.CodeServerError {
		t.Fatalf("error=%v", err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 2 || !strings.Contains(lines[0], `"state":"0"`) || !strings.Contains(lines[1], `"state":"1"`) {
		t.Fatalf("prefix=%q", out.String())
	}
	calls := evaluator.Calls()
	for id := 0; id < 4; id++ {
		if calls[fmt.Sprint(id)] != 1 {
			t.Fatalf("calls=%v", calls)
		}
	}
	if calls["4"] != 0 || reader.Reads() != 3 {
		t.Fatalf("dispatch after failure: calls=%v reads=%d", calls, reader.Reads())
	}
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
	if !errors.As(err, &coded) || coded.Code != jeq.CodeInterrupted {
		t.Fatalf("error=%v", err)
	}
	if got := evaluator.Calls(); len(got) != 4 || reader.Reads() != 3 || out.Len() != 0 {
		t.Fatalf("cancellation calls=%v reads=%d output=%q", got, reader.Reads(), out.String())
	}
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
	if err := <-done; err == nil {
		t.Fatal("output failure was swallowed")
	}
	if calls := evaluator.Calls(); len(calls) != mapWorkerLimit || reader.Reads() != mapWorkerLimit-1 {
		t.Fatalf("work after output failure: calls=%v reads=%d", calls, reader.Reads())
	}
}

func TestMapSchedulerEmitsOneFinalTraceSummary(t *testing.T) {
	first, reader := schedulerRecords(4)
	evaluator := &schedulerEvaluator{started: make(chan string, 8), calls: map[string]int{}}
	var traceOut bytes.Buffer
	cfg := trace.New(true, "scheduler", &traceOut)
	evaluator.observer = cfg
	ctx := trace.WithContext(context.Background(), cfg)
	var out bytes.Buffer
	if err := <-runScheduler(ctx, &out, reader, first, evaluator); err != nil {
		t.Fatal(err)
	}
	var summaries []map[string]any
	attempts, retries := 0, 0
	for _, line := range strings.Split(strings.TrimSpace(traceOut.String()), "\n") {
		var event map[string]any
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatal(err)
		}
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
	if attempts != 8 || retries != 4 {
		t.Fatalf("retry trace attempts=%d retries=%d trace=%s", attempts, retries, traceOut.String())
	}
	if len(summaries) != 1 {
		t.Fatalf("summaries=%d trace=%s", len(summaries), traceOut.String())
	}
	if summaries[0]["seen"] != float64(4) || summaries[0]["succeeded"] != float64(4) || summaries[0]["emitted"] != float64(4) {
		t.Fatalf("summary=%v", summaries[0])
	}
}
