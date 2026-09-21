package cli

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

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

type orderedReader struct {
	phase   int
	written <-chan struct{}
}

func (r *orderedReader) Read(p []byte) (int, error) {
	if r.phase == 0 {
		r.phase = 1
		return copy(p, []byte("{\"state\":\"first\"}\n")), nil
	}
	if r.phase == 2 {
		return 0, io.EOF
	}
	select {
	case <-r.written:
		r.phase = 2
		return copy(p, []byte("{\"state\":\"second\"}\n")), nil
	default:
		return 0, errors.New("read occurred before first output")
	}
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

func TestRunMapCancellationClosesBlockedPipe(t *testing.T) {
	pipeReader, pipeWriter := io.Pipe()
	reader := &blockedPipeReader{PipeReader: pipeReader, blocked: make(chan struct{})}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	deps := AskDeps{Stdin: reader, ReadStdin: func(io.Reader, int64, bool) ([]byte, *jeq.Error) { return nil, nil }, ReadFile: func(string, int64) ([]byte, *jeq.Error) {
		return []byte(`{"questions":{"q":{"type":"noul","instructions":"is it?"}}}`), nil
	}, NewClient: func(string, time.Duration, string, int, func(string)) APIClient { return &streamClient{} }, Getenv: func(key string) string {
		if key == "TYPESAFE_API_KEY" {
			return "test"
		}
		return ""
	}}
	cmd := NewMapCmd(deps)
	cmd.SetArgs([]string{"--as", "x", "--input", "ndjson", "--questions", "q.json", "--model", "m"})
	done := make(chan error, 1)
	go func() { done <- cmd.ExecuteContext(ctx) }()
	<-reader.blocked
	cancel()
	err := <-done
	_ = pipeWriter
	if err == nil || !strings.Contains(err.Error(), string(jeq.CodeInterrupted)) {
		t.Fatalf("err=%v", err)
	}
}

func TestMapNDJSONEmitsBeforeProducerEOF(t *testing.T) {
	written := make(chan struct{})
	var output bytes.Buffer
	reader := &orderedReader{written: written}
	client := &streamClient{}
	deps := AskDeps{Stdin: reader, ReadStdin: func(io.Reader, int64, bool) ([]byte, *jeq.Error) { return nil, nil }, ReadFile: func(string, int64) ([]byte, *jeq.Error) {
		return []byte(`{"questions":{"q":{"type":"noul","instructions":"is it?"}}}`), nil
	}, NewClient: func(string, time.Duration, string, int, func(string)) APIClient { return client }, Getenv: func(key string) string {
		if key == "TYPESAFE_API_KEY" {
			return "test"
		}
		return ""
	}}
	code := RunWithDeps([]string{"map", "--as", "x", "--input", "ndjson", "--questions", "q.json", "--model", "m"}, &orderedWriter{output: &output, written: written}, io.Discard, streamRenderer{}, deps)
	if code != 0 || client.calls != 2 || !bytes.Contains(output.Bytes(), []byte("first")) {
		t.Fatalf("code=%d calls=%d output=%q", code, client.calls, output.String())
	}
}

func TestMapStreamEmitsBeforeReadError(t *testing.T) {
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	client := &streamClient{}
	err := processMapStream(context.Background(), cmd, streamRenderer{}, bufio.NewReader(&readErrorAfterFirst{}), []byte("{\"state\":\"first\"}"), mapFlags{input: "ndjson", name: "x", statePointer: "/state"}, map[string]contract.Question{"q": {Type: contract.TypeNoul, Instructions: json.RawMessage(`"is it?"`)}}, nil, "m", client)
	if err == nil || !strings.Contains(out.String(), "first") {
		t.Fatalf("err=%v out=%q", err, out.String())
	}
}

func TestMapStreamDoesNotReadAfterEvaluationCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	client := &cancelClient{cancel: cancel}
	reads := 0
	reader := &countingReader{reads: &reads}
	err := processMapStream(ctx, &cobra.Command{}, streamRenderer{}, bufio.NewReader(reader), []byte("{\"state\":\"first\"}"), mapFlags{input: "ndjson", name: "x", statePointer: "/state"}, map[string]contract.Question{"q": {Type: contract.TypeNoul, Instructions: json.RawMessage(`\"is it?\"`)}}, nil, "m", client)
	if err == nil || reads != 0 {
		t.Fatalf("err=%v reads=%d", err, reads)
	}
}

type countingReader struct{ reads *int }

func (r *countingReader) Read([]byte) (int, error) {
	*r.reads++
	return 0, errors.New("reader must not be called")
}

func TestReadNDJSONSkipsEmptyAndRejectsOversized(t *testing.T) {
	record, eof, err := readNDJSONRecord(bufio.NewReader(strings.NewReader("\n\n{\"x\":1}\n")))
	if err != nil || eof || string(record) != "{\"x\":1}" {
		t.Fatalf("record=%q eof=%v err=%v", record, eof, err)
	}
	exact := strings.Repeat("x", MapMaxRecordBytes) + "\n"
	if record, _, err := readNDJSONRecord(bufio.NewReader(strings.NewReader(exact))); err != nil || len(record) != MapMaxRecordBytes {
		t.Fatalf("boundary record len=%d err=%v", len(record), err)
	}
	_, _, err = readNDJSONRecord(bufio.NewReader(strings.NewReader(strings.Repeat("x", MapMaxRecordBytes+1))))
	if err == nil {
		t.Fatal("oversized record accepted")
	}
}

func TestMapStreamStopsBeforeReadingWhenCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	client := &cancelClient{}
	err := processMapStream(ctx, &cobra.Command{}, streamRenderer{}, bufio.NewReader(strings.NewReader("{\\\"state\\\":\\\"never\\\"}\\n")), []byte("{\\\"state\\\":\\\"first\\\"}"), mapFlags{input: "ndjson", name: "x", statePointer: "/state"}, map[string]contract.Question{"q": {Type: contract.TypeNoul, Instructions: json.RawMessage(`\"is it?\"`)}}, nil, "m", client)
	if err == nil || client.calls != 0 {
		t.Fatalf("err=%v calls=%d", err, client.calls)
	}
}

type eofEvidenceReader struct {
	output  *bytes.Buffer
	eofSeen bool
}

func (r *eofEvidenceReader) Read([]byte) (int, error) {
	if r.output.Len() == 0 {
		panic("EOF requested before first output")
	}
	r.eofSeen = true
	return 0, io.EOF
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

func TestMapStreamRequestsEOFOnlyAfterFirstOutput(t *testing.T) {
	var out bytes.Buffer
	reader := &eofEvidenceReader{output: &out}
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	if err := processMapStream(context.Background(), cmd, streamRenderer{}, bufio.NewReader(reader), []byte("{\"state\":\"first\"}"), mapFlags{input: "ndjson", name: "x", statePointer: "/state"}, map[string]contract.Question{"q": {Type: contract.TypeNoul, Instructions: json.RawMessage(`\"is it?\"`)}}, nil, "m", &streamClient{}); err != nil || !reader.eofSeen {
		t.Fatalf("err=%v eof=%v out=%q", err, reader.eofSeen, out.String())
	}
}

func TestMapStreamStopsOnOutputFailure(t *testing.T) {
	reads := 0
	reader := &countingDataReader{reads: &reads, data: []byte("{\"state\":\"second\"}\n")}
	client := &streamClient{}
	err := processMapStream(context.Background(), &cobra.Command{}, failingRenderer{}, bufio.NewReader(reader), []byte("{\"state\":\"first\"}"), mapFlags{input: "ndjson", name: "x", statePointer: "/state"}, map[string]contract.Question{"q": {Type: contract.TypeNoul, Instructions: json.RawMessage(`\"is it?\"`)}}, nil, "m", client)
	if err == nil || reads != 0 || client.calls != 1 {
		t.Fatalf("err=%v reads=%d calls=%d", err, reads, client.calls)
	}
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
	err := processMapStream(context.Background(), cmd, streamRenderer{}, bufio.NewReader(reader), []byte("{\"state\":\"first\"}"), mapFlags{input: "ndjson", name: "x", statePointer: "/state"}, map[string]contract.Question{"q": {Type: contract.TypeNoul, Instructions: json.RawMessage(`\"is it?\"`)}}, nil, "m", client)
	if err == nil || client.calls != 2 || reads != 1 || !strings.Contains(out.String(), "first") {
		t.Fatalf("err=%v calls=%d reads=%d out=%q", err, client.calls, reads, out.String())
	}
}

type failingStreamClient struct{ calls int }

func (c *failingStreamClient) Evaluate(context.Context, contract.Request) (contract.Response, *jeq.Error) {
	c.calls++
	if c.calls == 2 {
		return contract.Response{}, jeq.NewError(jeq.CodeServerError, "provider failure")
	}
	return contract.Response{Model: "m"}, nil
}

func TestMapStreamRejectsMalformedAndNonObject(t *testing.T) {
	for _, record := range []string{"not-json", "[]"} {
		err := processMapStream(context.Background(), &cobra.Command{}, streamRenderer{}, bufio.NewReader(strings.NewReader("")), []byte(record), mapFlags{input: "ndjson", name: "x", statePointer: "/state"}, map[string]contract.Question{"q": {Type: contract.TypeNoul, Instructions: json.RawMessage(`\"is it?\"`)}}, nil, "m", &streamClient{})
		if err == nil {
			t.Fatalf("record %q accepted", record)
		}
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
	if code != 0 || client.calls != MapMaxRecords+1 {
		t.Fatalf("code=%d calls=%d stderr=%q", code, client.calls, stderr.String())
	}
}
