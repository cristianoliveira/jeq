package source_test

import (
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/cristianoliveira/jeq/internal/domain/jeq"
	"github.com/cristianoliveira/jeq/internal/infra/source"
)

func TestReadOptionalFileMissingAndReadsOneBoundedStream(t *testing.T) {
	data, err, found := source.ReadOptionalFile("/definitely/missing/jeq-config.json", 32)
	if err != nil || found || data != nil {
		t.Fatalf("data=%q err=%v found=%v", data, err, found)
	}

	var opens int
	open := func(string) (io.ReadCloser, error) { opens++; return io.NopCloser(strings.NewReader("abc")), nil }
	data, coded, found := source.ReadOptionalFileWithOpener("config.json", 32, open)
	if coded != nil || !found || string(data) != "abc" || opens != 1 {
		t.Fatalf("data=%q err=%v found=%v opens=%d", data, coded, found, opens)
	}
}

func TestReadOptionalFileReportsOversizeAndReadFailure(t *testing.T) {
	t.Run("oversized file returns input error and is found", func(t *testing.T) {
		open := func(string) (io.ReadCloser, error) { return io.NopCloser(strings.NewReader("abcd")), nil }
		_, err, found := source.ReadOptionalFileWithOpener("config.json", 3, open)
		if err == nil || err.Code != jeq.CodeInputInvalid || !found {
			t.Fatalf("err=%v found=%v", err, found)
		}
	})

	t.Run("read failure returns input error and is found", func(t *testing.T) {
		readErr := errors.New("boom")
		bad := func(string) (io.ReadCloser, error) { return &failingCloser{err: readErr}, nil }
		_, err, found := source.ReadOptionalFileWithOpener("config.json", 32, bad)
		if err == nil || err.Code != jeq.CodeInputInvalid || !found {
			t.Fatalf("read err=%v found=%v", err, found)
		}
	})
}

type failingCloser struct{ err error }

func (f *failingCloser) Read([]byte) (int, error) { return 0, f.err }
func (f *failingCloser) Close() error             { return nil }
