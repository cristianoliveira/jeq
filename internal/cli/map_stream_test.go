package cli

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

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
