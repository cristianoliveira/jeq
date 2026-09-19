package source_test

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianoliveira/gev/internal/domain/gev"
	"github.com/cristianoliveira/gev/internal/infra/source"
)

func TestReadFileExactBytesWithinLimit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.txt")
	want := []byte("line one\nline two \xe2\x9c\x93\n")
	if err := os.WriteFile(path, want, 0o644); err != nil {
		t.Fatal(err)
	}

	var opens int
	open := func(string) (io.ReadCloser, error) {
		opens++
		return source.OSOpen(path)
	}

	got, err := source.ReadFile(path, 1024, open, false)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("bytes not preserved exactly: %q", got)
	}
	if opens != 1 {
		t.Errorf("opener called %d times, want exactly 1", opens)
	}
}

func TestReadFileFailures(t *testing.T) {
	dir := t.TempDir()
	existing := filepath.Join(dir, "exists.txt")
	if err := os.WriteFile(existing, []byte("xxx"), 0o644); err != nil {
		t.Fatal(err)
	}
	empty := filepath.Join(dir, "empty.txt")
	if err := os.WriteFile(empty, nil, 0o644); err != nil {
		t.Fatal(err)
	}

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
			if err == nil || err.Code != gev.CodeInputInvalid {
				t.Fatalf("expected %q, got %v", gev.CodeInputInvalid, err)
			}
			if !strings.Contains(err.Message, tt.path) {
				t.Errorf("message %q does not name the offending source", err.Message)
			}
			if !strings.Contains(err.Message, tt.wantInMsg) {
				t.Errorf("message %q missing %q", err.Message, tt.wantInMsg)
			}
			if err.Recovery == "" {
				t.Error("failure carries no recovery instruction")
			}
		})
	}
}

func TestReadFileUnreadable(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root: permission bits are not enforced")
	}
	path := filepath.Join(t.TempDir(), "secret.txt")
	if err := os.WriteFile(path, []byte("x"), 0o000); err != nil {
		t.Fatal(err)
	}

	_, err := source.ReadFile(path, 1024, source.OSOpen, false)
	if err == nil || err.Code != gev.CodeInputInvalid {
		t.Fatalf("expected %q, got %v", gev.CodeInputInvalid, err)
	}
	if !strings.Contains(err.Message, "readable") {
		t.Errorf("message %q does not describe the permission problem", err.Message)
	}
}

func TestReadStdinPiped(t *testing.T) {
	content := []byte("{\"questions\":{}}")
	r := &countingReader{r: strings.NewReader(string(content))}

	got, err := source.ReadStdin(r, 1024, func() bool { return false }, true)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if string(got) != string(content) {
		t.Errorf("bytes not preserved: %q", got)
	}
	if r.reads == 0 || r.total > 1024+1 {
		t.Errorf("read count/bytes wrong: reads=%d total=%d", r.reads, r.total)
	}
}

func TestReadStdinTTYFailsBeforeAnyRead(t *testing.T) {
	r := &countingReader{r: strings.NewReader("data")}

	_, err := source.ReadStdin(r, 1024, func() bool { return true }, true)
	if err == nil || err.Code != gev.CodeInputInvalid {
		t.Fatalf("expected %q, got %v", gev.CodeInputInvalid, err)
	}
	if r.reads != 0 {
		t.Errorf("TTY stdin was read %d times; must fail pre-read", r.reads)
	}
	if !strings.Contains(err.Message, "terminal") {
		t.Errorf("message %q does not explain the TTY problem", err.Message)
	}
}

func TestReadStdinOversize(t *testing.T) {
	r := &countingReader{r: strings.NewReader(strings.Repeat("x", 64))}

	_, err := source.ReadStdin(r, 32, func() bool { return false }, false)
	if err == nil || err.Code != gev.CodeInputInvalid {
		t.Fatalf("expected %q, got %v", gev.CodeInputInvalid, err)
	}
	if r.total > 33 {
		t.Errorf("read %d bytes; must stop at limit+1", r.total)
	}
}

func TestReadStdinEmptyForbidden(t *testing.T) {
	r := &countingReader{r: strings.NewReader("")}

	_, err := source.ReadStdin(r, 32, func() bool { return false }, true)
	if err == nil || err.Code != gev.CodeInputInvalid {
		t.Fatalf("expected %q, got %v", gev.CodeInputInvalid, err)
	}
}

func TestReadStdinExactlyAtLimitSucceeds(t *testing.T) {
	content := strings.Repeat("y", 32)
	r := &countingReader{r: strings.NewReader(content)}

	got, err := source.ReadStdin(r, 32, func() bool { return false }, false)
	if err != nil {
		t.Fatalf("exactly-at-limit read failed: %v", err)
	}
	if string(got) != content {
		t.Error("bytes not preserved at the limit boundary")
	}
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
