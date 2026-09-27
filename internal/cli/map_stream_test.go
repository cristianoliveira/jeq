package cli

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cristianoliveira/jeq/internal/domain/contract"
	"github.com/cristianoliveira/jeq/internal/domain/jeq"
	"github.com/spf13/cobra"
)

type readErrorAfterFirst struct{}

func (r *readErrorAfterFirst) Read([]byte) (int, error) {
	return 0, errors.New("synthetic read failure")
}

type cancelClient struct {
	calls  int
	cancel context.CancelFunc
}

type releaseStreamClient struct{ release <-chan struct{} }

func (c releaseStreamClient) Evaluate(ctx context.Context, _ contract.Request) (contract.Response, *jeq.Error) {
	select {
	case <-c.release:
		return contract.Response{Model: "m"}, nil
	case <-ctx.Done():
		return contract.Response{}, jeq.NewError(jeq.CodeInterrupted, "stream evaluation cancelled")
	}
}

func (c *cancelClient) Evaluate(ctx context.Context, _ contract.Request) (contract.Response, *jeq.Error) {
	c.calls++
	if c.cancel != nil {
		c.cancel()
	}
	if ctx.Err() != nil {
		return contract.Response{}, jeq.NewError(jeq.CodeInterrupted, "cancelled")
	}
	return contract.Response{Model: "m"}, nil
}

type openProducerReader struct {
	phase        int
	eofRequested chan struct{}
	releaseEOF   chan struct{}
	releaseOnce  sync.Once
}

func (r *openProducerReader) Read(p []byte) (int, error) {
	if r.phase == 0 {
		r.phase = 1
		return copy(p, []byte("{\"state\":\"first\"}\n")), nil
	}
	if r.phase == 1 {
		r.phase = 2
		return copy(p, []byte("{\"state\":\"second\"}\n")), nil
	}
	if r.phase == 2 {
		close(r.eofRequested)
		<-r.releaseEOF
		r.phase = 3
	}
	return 0, io.EOF
}

func (r *openProducerReader) releaseProducerEOF() {
	r.releaseOnce.Do(func() { close(r.releaseEOF) })
}

type orderedWriter struct {
	output  *bytes.Buffer
	written chan struct{}
}

func (w *orderedWriter) Write(p []byte) (int, error) {
	n, err := w.output.Write(p)
	select {
	case <-w.written:
	default:
		close(w.written)
	}
	return n, err
}

type blockedPipeReader struct {
	*io.PipeReader
	blocked chan struct{}
}

func (r *blockedPipeReader) Read(p []byte) (int, error) {
	select {
	case <-r.blocked:
	default:
		close(r.blocked)
	}
	return r.PipeReader.Read(p)
}

func (*blockedPipeReader) InterruptsReadOnClose() {}

type closeOnlyReader struct{ closed bool }

func (*closeOnlyReader) Read([]byte) (int, error) { return 0, io.EOF }
func (r *closeOnlyReader) Close() error {
	r.closed = true
	return nil
}

type closeableBlockedReader struct {
	phase     int
	blocked   chan struct{}
	closed    chan struct{}
	closeOnce sync.Once
}

func (r *closeableBlockedReader) Read(p []byte) (int, error) {
	if r.phase == 0 {
		r.phase = 1
		return copy(p, []byte("{\"state\":\"second\"}\n")), nil
	}
	close(r.blocked)
	<-r.closed
	return 0, errors.New("reader closed")
}

func (r *closeableBlockedReader) Close() error {
	r.closeOnce.Do(func() { close(r.closed) })
	return nil
}

type externallyReleasedReader struct {
	phase       int
	blocked     chan struct{}
	release     chan struct{}
	readDone    chan struct{}
	releaseOnce sync.Once
	closeCalled bool
}

func (r *externallyReleasedReader) Read(p []byte) (int, error) {
	if r.phase == 0 {
		r.phase = 1
		return copy(p, []byte("{\"state\":\"second\"}\n")), nil
	}
	close(r.blocked)
	<-r.release
	close(r.readDone)
	return 0, io.EOF
}

func (r *externallyReleasedReader) Close() error {
	r.closeCalled = true
	return nil
}

func (r *externallyReleasedReader) allowReadToFinish() {
	r.releaseOnce.Do(func() { close(r.release) })
}

func mapStreamTestDeps(stdin io.Reader, client APIClient) AskDeps {
	return AskDeps{
		Stdin: stdin,
		ReadStdin: func(io.Reader, int64, bool) ([]byte, *jeq.Error) {
			return nil, nil
		},
		ReadFile: func(string, int64) ([]byte, *jeq.Error) {
			return []byte(`{"questions":{"q":{"type":"noul","instructions":"is it?"}}}`), nil
		},
		NewClient: func(string, time.Duration, string, int, func(string)) APIClient {
			return client
		},
		Getenv: func(key string) string {
			if key == "TYPESAFE_API_KEY" {
				return "test"
			}
			return ""
		},
	}
}

func TestInterruptibleInputCloseRequiresCloseGuarantee(t *testing.T) {
	reader := &closeOnlyReader{}
	assert.Nil(t, interruptibleInputClose(reader))
	assert.False(t, reader.closed, "an arbitrary io.Closer may not interrupt Read")
}

func TestInterruptibleInputCloseRecognizesOSFile(t *testing.T) {
	reader, writer, err := os.Pipe()
	require.NoError(t, err)
	defer func() { _ = writer.Close() }()
	defer func() { _ = reader.Close() }()
	closeInput := interruptibleInputClose(reader)
	require.NotNil(t, closeInput)
	closeInput()
	_, err = reader.Read(make([]byte, 1))
	assert.Error(t, err)
}

func TestInterruptibleInputCloseRejectsNonPollableFile(t *testing.T) {
	file, err := os.CreateTemp(t.TempDir(), "input")
	require.NoError(t, err)
	defer func() { _ = file.Close() }()
	deadlineErr := file.SetReadDeadline(time.Now())
	if deadlineErr == nil {
		require.NoError(t, file.SetReadDeadline(time.Time{}))
	}
	closeInput := interruptibleInputClose(file)
	assert.Equal(t, deadlineErr == nil, closeInput != nil)
}

func TestRunMapCancellationClosesBlockedPipe(t *testing.T) {
	pipeReader, pipeWriter := io.Pipe()
	reader := &blockedPipeReader{PipeReader: pipeReader, blocked: make(chan struct{})}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	deps := mapStreamTestDeps(reader, &streamClient{})
	cmd := NewMapCmd(deps)
	cmd.SetArgs([]string{"--as", "x", "--input", "ndjson", "--questions", "q.json", "--model", "m"})
	done := make(chan error, 1)
	go func() { done <- cmd.ExecuteContext(ctx) }()
	<-reader.blocked
	cancel()
	err := <-done
	_ = pipeWriter
	require.Error(t, err)
	assert.Contains(t, err.Error(), string(jeq.CodeInterrupted))
}

func TestMapNDJSONEmitsBeforeProducerEOF(t *testing.T) {
	written := make(chan struct{})
	var output, stderr bytes.Buffer
	reader := &openProducerReader{eofRequested: make(chan struct{}), releaseEOF: make(chan struct{})}
	t.Cleanup(reader.releaseProducerEOF)
	client := &streamClient{}
	deps := mapStreamTestDeps(reader, client)
	done := make(chan int, 1)
	go func() {
		done <- RunWithDeps([]string{"map", "--as", "x", "--input", "ndjson", "--questions", "q.json", "--model", "m"}, &orderedWriter{output: &output, written: written}, &stderr, streamRenderer{}, deps)
	}()

	select {
	case <-reader.eofRequested:
	case code := <-done:
		t.Fatalf("command exited before the open producer reached EOF: %d", code)
	case <-time.After(5 * time.Second):
		reader.releaseProducerEOF()
		<-done
		t.Fatal("command did not request the producer's next record")
	}
	select {
	case <-written:
	case code := <-done:
		t.Fatalf("command exited before writing output: %d, stderr=%q", code, stderr.String())
	case <-time.After(5 * time.Second):
		reader.releaseProducerEOF()
		<-done
		t.Fatal("command waited for producer EOF before writing the first result")
	}

	reader.releaseProducerEOF()
	assert.Equal(t, 0, <-done, "stderr=%q", stderr.String())
	assert.Equal(t, 2, client.calls)
	assert.Contains(t, output.String(), "first")
}

func TestMapStreamClosesBlockedReadOnCancellation(t *testing.T) {
	reader := &closeableBlockedReader{blocked: make(chan struct{}), closed: make(chan struct{})}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cmd := &cobra.Command{}
	cmd.SetOut(io.Discard)
	done := make(chan error, 1)
	go func() {
		done <- processMapStream(ctx, cmd, streamRenderer{}, bufio.NewReader(reader), []byte(`{"state":"first"}`), mapFlags{input: "ndjson", name: "x", statePointer: "/state"}, map[string]contract.Question{"q": {Type: contract.TypeNoul, Instructions: json.RawMessage(`"is it?"`)}}, nil, "m", &streamClient{}, func() { _ = reader.Close() })
	}()
	<-reader.blocked
	cancel()
	err := <-done
	var coded *jeq.Error
	require.Error(t, err)
	require.ErrorAs(t, err, &coded)
	assert.Equal(t, jeq.CodeInterrupted, coded.Code)
	select {
	case <-reader.closed:
	default:
		t.Fatal("cancellation did not close the blocked input")
	}
}

func TestMapStreamCannotInterruptBlockedNonClosableRead(t *testing.T) {
	reader := &externallyReleasedReader{blocked: make(chan struct{}), release: make(chan struct{}), readDone: make(chan struct{})}
	t.Cleanup(reader.allowReadToFinish)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cmd := &cobra.Command{}
	cmd.SetOut(io.Discard)
	done := make(chan error, 1)
	go func() {
		done <- processMapStream(ctx, cmd, streamRenderer{}, bufio.NewReader(reader), []byte(`{"state":"first"}`), mapFlags{input: "ndjson", name: "x", statePointer: "/state"}, map[string]contract.Question{"q": {Type: contract.TypeNoul, Instructions: json.RawMessage(`"is it?"`)}}, nil, "m", &streamClient{}, interruptibleInputClose(reader))
	}()
	<-reader.blocked
	cancel()
	var err error
	select {
	case err = <-done:
	case <-time.After(5 * time.Second):
		reader.allowReadToFinish()
		<-done
		t.Fatal("cancellation waited for a non-closable blocked Read")
	}
	var coded *jeq.Error
	require.ErrorAs(t, err, &coded)
	assert.Equal(t, jeq.CodeInterrupted, coded.Code)
	assert.False(t, reader.closeCalled, "Close alone does not promise to unblock Read")
	select {
	case <-reader.readDone:
		t.Fatal("the test reader unexpectedly finished without external release")
	default:
	}
	reader.allowReadToFinish()
	<-reader.readDone
}

func TestMapStreamClosesBlockedReadOnWorkerFailure(t *testing.T) {
	reader := &closeableBlockedReader{blocked: make(chan struct{}), closed: make(chan struct{})}
	cmd := &cobra.Command{}
	cmd.SetOut(io.Discard)
	done := make(chan error, 1)
	go func() {
		done <- processMapStream(context.Background(), cmd, streamRenderer{}, bufio.NewReader(reader), []byte(`{"state":"first"}`), mapFlags{input: "ndjson", name: "x", statePointer: "/state"}, map[string]contract.Question{"q": {Type: contract.TypeNoul, Instructions: json.RawMessage(`"is it?"`)}}, nil, "m", &failingStreamClient{}, func() { _ = reader.Close() })
	}()
	<-reader.blocked
	err := <-done
	var coded *jeq.Error
	require.ErrorAs(t, err, &coded)
	assert.Equal(t, jeq.CodeServerError, coded.Code)
	select {
	case <-reader.closed:
	default:
		t.Fatal("worker failure did not close the blocked input")
	}
}

func TestMapStreamClosesBlockedReadOnOutputFailure(t *testing.T) {
	reader := &closeableBlockedReader{blocked: make(chan struct{}), closed: make(chan struct{})}
	release := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- processMapStream(context.Background(), &cobra.Command{}, failingRenderer{}, bufio.NewReader(reader), []byte(`{"state":"first"}`), mapFlags{input: "ndjson", name: "x", statePointer: "/state"}, map[string]contract.Question{"q": {Type: contract.TypeNoul, Instructions: json.RawMessage(`"is it?"`)}}, nil, "m", releaseStreamClient{release: release}, func() { _ = reader.Close() })
	}()
	<-reader.blocked
	close(release)
	require.Error(t, <-done)
	select {
	case <-reader.closed:
	default:
		t.Fatal("output failure did not close the blocked input")
	}
}

func TestMapStreamEmitsBeforeReadError(t *testing.T) {
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	client := &streamClient{}
	err := processMapStream(context.Background(), cmd, streamRenderer{}, bufio.NewReader(&readErrorAfterFirst{}), []byte("{\"state\":\"first\"}"), mapFlags{input: "ndjson", name: "x", statePointer: "/state"}, map[string]contract.Question{"q": {Type: contract.TypeNoul, Instructions: json.RawMessage(`"is it?"`)}}, nil, "m", client, func() {})
	require.Error(t, err)
	assert.Contains(t, out.String(), "first")
}

func TestMapStreamDoesNotReadAfterEvaluationCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	client := &cancelClient{cancel: cancel}
	reads := 0
	reader := &countingReader{reads: &reads}
	err := processMapStream(ctx, &cobra.Command{}, streamRenderer{}, bufio.NewReader(reader), []byte("{\"state\":\"first\"}"), mapFlags{input: "ndjson", name: "x", statePointer: "/state"}, map[string]contract.Question{"q": {Type: contract.TypeNoul, Instructions: json.RawMessage(`\"is it?\"`)}}, nil, "m", client, func() {})
	require.Error(t, err)
	assert.LessOrEqual(t, reads, 4)
}

type countingReader struct{ reads *int }

func (r *countingReader) Read([]byte) (int, error) {
	*r.reads++
	return 0, errors.New("reader must not be called")
}

func TestReadNDJSONSkipsEmptyAndRejectsOversized(t *testing.T) {
	record, eof, err := readNDJSONRecord(bufio.NewReader(strings.NewReader("\n\n{\"x\":1}\n")))
	require.Nil(t, err)
	assert.False(t, eof)
	assert.Equal(t, "{\"x\":1}", string(record))
	exact := strings.Repeat("x", MapMaxRecordBytes) + "\n"
	record, _, err = readNDJSONRecord(bufio.NewReader(strings.NewReader(exact)))
	require.Nil(t, err)
	assert.Len(t, record, MapMaxRecordBytes)
	_, _, err = readNDJSONRecord(bufio.NewReader(strings.NewReader(strings.Repeat("x", MapMaxRecordBytes+1))))
	require.NotNil(t, err, "oversized record accepted")
}

func TestMapStreamStopsBeforeReadingWhenCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	client := &cancelClient{}
	err := processMapStream(ctx, &cobra.Command{}, streamRenderer{}, bufio.NewReader(strings.NewReader("{\\\"state\\\":\\\"never\\\"}\\n")), []byte("{\\\"state\\\":\\\"first\\\"}"), mapFlags{input: "ndjson", name: "x", statePointer: "/state"}, map[string]contract.Question{"q": {Type: contract.TypeNoul, Instructions: json.RawMessage(`\"is it?\"`)}}, nil, "m", client, func() {})
	require.Error(t, err)
	assert.Zero(t, client.calls)
}

type eofEvidenceReader struct {
	eofSeen chan struct{}
}

func (r *eofEvidenceReader) Read([]byte) (int, error) {
	close(r.eofSeen)
	return 0, io.EOF
}

type eofWaitingClient struct{ eofSeen <-chan struct{} }

func (c eofWaitingClient) Evaluate(context.Context, contract.Request) (contract.Response, *jeq.Error) {
	<-c.eofSeen
	return contract.Response{Model: "m"}, nil
}

type failingRenderer struct{}

func (failingRenderer) RenderSuccess(io.Writer, contract.Response) error {
	return errors.New("synthetic stdout failure")
}
func (failingRenderer) RenderError(io.Writer, *jeq.Error) error { return nil }
func (failingRenderer) RenderValue(io.Writer, any) error        { return nil }
func (failingRenderer) RenderRaw(io.Writer, []byte) error {
	return errors.New("synthetic stdout failure")
}

func TestMapStreamAcceptsFastEOF(t *testing.T) {
	var out bytes.Buffer
	reader := &eofEvidenceReader{eofSeen: make(chan struct{})}
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	err := processMapStream(context.Background(), cmd, streamRenderer{}, bufio.NewReader(reader), []byte("{\"state\":\"first\"}"), mapFlags{input: "ndjson", name: "x", statePointer: "/state"}, map[string]contract.Question{"q": {Type: contract.TypeNoul, Instructions: json.RawMessage(`\"is it?\"`)}}, nil, "m", eofWaitingClient{eofSeen: reader.eofSeen}, func() {})
	require.NoError(t, err)
	select {
	case <-reader.eofSeen:
	default:
		t.Fatal("reader did not reach EOF")
	}
	assert.Contains(t, out.String(), "first")
}

func TestMapStreamStopsOnOutputFailure(t *testing.T) {
	reads := 0
	reader := &countingDataReader{reads: &reads, data: []byte("{\"state\":\"second\"}\n")}
	client := &streamClient{}
	err := processMapStream(context.Background(), &cobra.Command{}, failingRenderer{}, bufio.NewReader(reader), []byte("{\"state\":\"first\"}"), mapFlags{input: "ndjson", name: "x", statePointer: "/state"}, map[string]contract.Question{"q": {Type: contract.TypeNoul, Instructions: json.RawMessage(`\"is it?\"`)}}, nil, "m", client, func() {})
	require.Error(t, err)
	assert.LessOrEqual(t, reads, 4)
	assert.LessOrEqual(t, client.calls, 4)
}

type countingDataReader struct {
	reads *int
	data  []byte
}

func (r *countingDataReader) Read(p []byte) (int, error) {
	*r.reads++
	if len(r.data) == 0 {
		return 0, io.EOF
	}
	n := copy(p, r.data)
	r.data = r.data[n:]
	return n, nil
}

func TestMapStreamStopsOnProviderFailure(t *testing.T) {
	reads := 0
	reader := &countingDataReader{reads: &reads, data: []byte("{\"state\":\"second\"}\n{\"state\":\"third\"}\n")}
	client := &failingStreamClient{}
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	err := processMapStream(context.Background(), cmd, streamRenderer{}, bufio.NewReader(reader), []byte("{\"state\":\"first\"}"), mapFlags{input: "ndjson", name: "x", statePointer: "/state"}, map[string]contract.Question{"q": {Type: contract.TypeNoul, Instructions: json.RawMessage(`\"is it?\"`)}}, nil, "m", client, func() {})
	require.Error(t, err)
	assert.LessOrEqual(t, client.calls, 4)
	assert.LessOrEqual(t, reads, 3)
	assert.Contains(t, out.String(), "first")
}

type failingStreamClient struct {
	mu    sync.Mutex
	calls int
}

func (c *failingStreamClient) Evaluate(_ context.Context, request contract.Request) (contract.Response, *jeq.Error) {
	c.mu.Lock()
	c.calls++
	c.mu.Unlock()
	if strings.Contains(string(request.State), "second") {
		return contract.Response{}, jeq.NewError(jeq.CodeServerError, "provider failure")
	}
	return contract.Response{Model: "m"}, nil
}

func TestMapStreamRejectsMalformedAndNonObject(t *testing.T) {
	for _, record := range []string{"not-json", "[]"} {
		err := processMapStream(context.Background(), &cobra.Command{}, streamRenderer{}, bufio.NewReader(strings.NewReader("")), []byte(record), mapFlags{input: "ndjson", name: "x", statePointer: "/state"}, map[string]contract.Question{"q": {Type: contract.TypeNoul, Instructions: json.RawMessage(`\"is it?\"`)}}, nil, "m", &streamClient{}, func() {})
		require.Error(t, err, "record %q accepted", record)
	}
}

func TestMapStreamProcessesMoreThanFormerRecordLimit(t *testing.T) {
	var input strings.Builder
	for i := 0; i < MapMaxRecords+1; i++ {
		input.WriteString("{\"state\":\"ok\"}\n")
	}
	client := &streamClient{}
	deps, _ := streamDeps(client, input.String(), "{\"questions\":{\"q\":{\"type\":\"noul\",\"instructions\":\"is it?\"}}}")
	var out, stderr bytes.Buffer
	code := RunWithDeps([]string{"map", "--as", "x", "--input", "ndjson", "--questions", "q.json", "--model", "m"}, &out, &stderr, streamRenderer{}, deps)
	assert.Equal(t, 0, code, "stderr=%q", stderr.String())
	assert.Equal(t, MapMaxRecords+1, client.calls)
}
