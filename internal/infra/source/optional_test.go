package source_test

import (
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
	cases := []struct {
		name     string
		maxBytes int64
		open     func(string) (io.ReadCloser, error)
	}{
		{
			name:     "oversized file returns input error and is found",
			maxBytes: 3,
			open: func(string) (io.ReadCloser, error) {
				return io.NopCloser(strings.NewReader("abcd")), nil
			},
		},
		{
			name:     "read failure returns input error and is found",
			maxBytes: 32,
			open: func(string) (io.ReadCloser, error) {
				return &failingCloser{err: errors.New("boom")}, nil
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err, found := source.ReadOptionalFileWithOpener("config.json", tc.maxBytes, tc.open)
			require.Error(t, err)
			assert.Equal(t, jeq.CodeInputInvalid, err.Code)
			assert.True(t, found)
		})
	}
}

type failingCloser struct{ err error }

func (f *failingCloser) Read([]byte) (int, error) { return 0, f.err }
func (f *failingCloser) Close() error             { return nil }
