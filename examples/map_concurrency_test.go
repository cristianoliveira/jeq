package examples_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestMapConcurrencyExampleUsesBuiltBinary(t *testing.T) {
	toolDir := t.TempDir()
	if err := os.Symlink(jeqBin, filepath.Join(toolDir, "jeq")); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "run", "./examples/map-concurrency", "--records", "100")
	cmd.Dir = repoRoot
	cmd.Env = append(os.Environ(), "PATH="+toolDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("example failed: %v\n%s", err, output)
	}
	line := strings.TrimSpace(string(output))
	if !strings.Contains(line, "records=100") || !strings.Contains(line, "requests=100") || !strings.Contains(line, "PASS") {
		t.Fatalf("unexpected output: %q", line)
	}
}
