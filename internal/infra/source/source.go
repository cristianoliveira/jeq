// Package source reads explicitly selected input files and stdin. It is the
// only package that touches the filesystem or stdin; the domain receives
// resolved bytes and stays pure. Failures are coded GEV_INPUT_INVALID, name
// the offending source, and carry one actionable recovery instruction.
package source

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"

	"github.com/cristianoliveira/gev/internal/domain/gev"
)

// Opener abstracts file opening so tests can count and inject.
type Opener func(path string) (io.ReadCloser, error)

// OSOpen is the production opener.
func OSOpen(path string) (io.ReadCloser, error) { return os.Open(path) }

// ReadFile reads an explicitly selected file source, up to limit bytes.
// Bytes are preserved exactly within the limit.
func ReadFile(path string, limit int64, open Opener, forbidEmpty bool) ([]byte, *gev.Error) {
	if limit <= 0 {
		return nil, misuse("byte limit must be positive")
	}

	f, err := open(path)
	if err != nil {
		switch {
		case errors.Is(err, fs.ErrNotExist):
			return nil, sourceError(path, "does not exist", "check the path, or pass '-' to read piped stdin")
		case errors.Is(err, fs.ErrPermission):
			return nil, sourceError(path, "is not readable", "check the file permissions")
		default:
			return nil, sourceError(path, fmt.Sprintf("cannot be opened: %v", err), "check the path and try again")
		}
	}
	defer func() { _ = f.Close() }()

	// Directories open successfully on some systems; detect before reading.
	if s, ok := f.(interface{ Stat() (fs.FileInfo, error) }); ok {
		if info, statErr := s.Stat(); statErr == nil && info.IsDir() {
			return nil, sourceError(path, "is a directory", "provide a file path, not a directory")
		}
	}

	data, over, rerr := readBounded(f, limit)
	if rerr != nil {
		return nil, sourceError(path, fmt.Sprintf("cannot be read: %v", rerr), "check the file and try again")
	}
	if over {
		return nil, sourceError(path, fmt.Sprintf("exceeds the %d byte limit", limit),
			"split or trim the content; gev reads at most the configured byte limit")
	}
	if forbidEmpty && len(data) == 0 {
		return nil, sourceError(path, "is empty", "provide non-empty content for this source")
	}
	return data, nil
}

// ReadOptionalFile reads a user config without treating absence as an error.
func ReadOptionalFile(path string, limit int64) ([]byte, *gev.Error, bool) {
	return ReadOptionalFileWithOpener(path, limit, OSOpen)
}

// ReadOptionalFileWithOpener is the injectable one-open implementation used by tests.
func ReadOptionalFileWithOpener(path string, limit int64, open Opener) ([]byte, *gev.Error, bool) {
	if limit <= 0 {
		return nil, misuse("byte limit must be positive"), true
	}
	f, err := open(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil, false
		}
		return nil, sourceError(path, fmt.Sprintf("cannot be opened: %v", err), "check the path and try again"), true
	}
	defer func() { _ = f.Close() }()
	if s, ok := f.(interface{ Stat() (fs.FileInfo, error) }); ok {
		if info, statErr := s.Stat(); statErr == nil && info.IsDir() {
			return nil, sourceError(path, "is a directory", "provide a file path, not a directory"), true
		}
	}
	data, over, readErr := readBounded(f, limit)
	if readErr != nil {
		return nil, sourceError(path, fmt.Sprintf("cannot be read: %v", readErr), "check the path and try again"), true
	}
	if over {
		return nil, sourceError(path, fmt.Sprintf("exceeds the %d byte limit", limit), "split or trim the content; gev reads at most the configured byte limit"), true
	}
	return data, nil, true
}

// ReadStdin reads explicit '-' stdin content. A terminal fails fast
// pre-read: gev never blocks waiting for a human to type a document.
func ReadStdin(stdin io.Reader, limit int64, isTTY func() bool, forbidEmpty bool) ([]byte, *gev.Error) {
	if limit <= 0 {
		return nil, misuse("byte limit must be positive")
	}
	if isTTY != nil && isTTY() {
		return nil, gev.NewError(gev.CodeInputInvalid,
			"stdin is a terminal; '-' would block waiting for typed input"+
				"; pipe the content or pass a file path instead").WithRecovery(
			"pipe the document: gev ... < file.json, or pass a file path instead of '-'")
	}

	data, over, rerr := readBounded(stdin, limit)
	if rerr != nil {
		return nil, gev.WrapError(gev.CodeInputInvalid, rerr,
			"stdin: read failed; check the pipe and try again")
	}
	if over {
		return nil, gev.NewError(gev.CodeInputInvalid,
			fmt.Sprintf("stdin: exceeds the %d byte limit; split or trim the piped content", limit))
	}
	if forbidEmpty && len(data) == 0 {
		return nil, gev.NewError(gev.CodeInputInvalid,
			"stdin: provided no content; pipe the document or pass a file path instead of '-'")
	}
	return data, nil
}

// readBounded reads at most limit bytes plus one sentinel byte to detect
// oversize deterministically.
func readBounded(r io.Reader, limit int64) (data []byte, over bool, err error) {
	data, err = io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, false, err
	}
	if int64(len(data)) > limit {
		return nil, true, nil
	}
	return data, false, nil
}

func sourceError(path, problem, recovery string) *gev.Error {
	return gev.NewError(gev.CodeInputInvalid,
		fmt.Sprintf("source %s: %s; %s", path, problem, recovery)).WithRecovery(recovery)
}

func misuse(msg string) *gev.Error {
	return gev.NewError(gev.CodeInputInvalid, msg+"; this is a gev bug")
}
