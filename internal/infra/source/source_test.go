package source_test

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cristianoliveira/jeq/internal/domain/jeq"
	"github.com/cristianoliveira/jeq/internal/infra/source"
)

func TestReadFileExactBytesWithinLimit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.txt")
	want := []byte("line one\nline two \xe2\x9c\x93\n")
	require.NoError(t, os.WriteFile(path, want, 0o644))

	var opens int
	open := func(string) (io.ReadCloser, error) {
		opens++
		return source.OSOpen(path)
	}

	got, err := source.ReadFile(path, 1024, open, false)
	require.Nil(t, err)
	assert.Equal(t, want, got)
	assert.Equal(t, 1, opens)
}

func TestReadFileFailures(t *testing.T) {
	dir := t.TempDir()
	existing := filepath.Join(dir, "exists.txt")
	require.NoError(t, os.WriteFile(existing, []byte("xxx"), 0o644))
	empty := filepath.Join(dir, "empty.txt")
	require.NoError(t, os.WriteFile(empty, nil, 0o644))

	tests := []struct {
		name        string
		path        string
		limit       int64
		forbidEmpty bool
		open        source.Opener
		wantInMsg   string
	}{
		{
			name:      "missing file names the source",
			path:      filepath.Join(dir, "missing.txt"),
			limit:     1024,
			wantInMsg: "does not exist",
		},
		{
			name:      "directory is rejected",
			path:      dir,
			limit:     1024,
			wantInMsg: "directory",
		},
		{
			name:        "empty forbidden",
			path:        empty,
			limit:       1024,
			forbidEmpty: true,
			wantInMsg:   "empty",
		},
		{
			name:      "oversize fails deterministically",
			path:      existing,
			limit:     2,
			wantInMsg: "byte limit",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := source.ReadFile(tt.path, tt.limit, source.OSOpen, tt.forbidEmpty)
			require.NotNil(t, err)
			assert.Equal(t, jeq.CodeInputInvalid, err.Code)
			assert.Contains(t, err.Message, tt.path, "offending source")
			assert.Contains(t, err.Message, tt.wantInMsg)
			assert.NotEmpty(t, err.Recovery)
		})
	}
}

func TestReadFileUnreadable(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root: permission bits are not enforced")
	}
	path := filepath.Join(t.TempDir(), "secret.txt")
	require.NoError(t, os.WriteFile(path, []byte("x"), 0o000))

	_, err := source.ReadFile(path, 1024, source.OSOpen, false)
	require.NotNil(t, err)
	assert.Equal(t, jeq.CodeInputInvalid, err.Code)
	assert.Contains(t, err.Message, "readable")
}

func TestReadStdinPiped(t *testing.T) {
	content := []byte("{\"questions\":{}}")
	r := &countingReader{r: strings.NewReader(string(content))}

	got, err := source.ReadStdin(r, 1024, func() bool { return false }, true)
	require.Nil(t, err)
	assert.Equal(t, content, got)
	assert.Greater(t, r.reads, 0)
	assert.LessOrEqual(t, r.total, 1024+1)
}

func TestReadStdinTTYFailsBeforeAnyRead(t *testing.T) {
	r := &countingReader{r: strings.NewReader("data")}

	_, err := source.ReadStdin(r, 1024, func() bool { return true }, true)
	require.NotNil(t, err)
	assert.Equal(t, jeq.CodeInputInvalid, err.Code)
	assert.Zero(t, r.reads, "TTY must fail before reading stdin")
	assert.Contains(t, err.Message, "terminal")
}

func TestReadStdinOversize(t *testing.T) {
	r := &countingReader{r: strings.NewReader(strings.Repeat("x", 64))}

	_, err := source.ReadStdin(r, 32, func() bool { return false }, false)
	require.NotNil(t, err)
	assert.Equal(t, jeq.CodeInputInvalid, err.Code)
	assert.LessOrEqual(t, r.total, 33, "reader must stop at limit+1")
}

func TestReadStdinEmptyForbidden(t *testing.T) {
	r := &countingReader{r: strings.NewReader("")}

	_, err := source.ReadStdin(r, 32, func() bool { return false }, true)
	require.NotNil(t, err)
	assert.Equal(t, jeq.CodeInputInvalid, err.Code)
}

func TestReadStdinExactlyAtLimitSucceeds(t *testing.T) {
	content := strings.Repeat("y", 32)
	r := &countingReader{r: strings.NewReader(content)}

	got, err := source.ReadStdin(r, 32, func() bool { return false }, false)
	require.Nil(t, err)
	assert.Equal(t, content, string(got))
}

type countingReader struct {
	r     io.Reader
	reads int
	total int
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.reads++
	c.total += n
	return n, err
}
