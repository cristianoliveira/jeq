package examples_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
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
	var summary struct {
		Records  int    `json:"records"`
		Requests int    `json:"requests"`
		Peak     int    `json:"peak"`
		Order    bool   `json:"order"`
		Status   string `json:"status"`
	}
	fields := strings.Fields(line)
	if len(fields) != 6 {
		t.Fatalf("unexpected summary fields: %q", line)
	}
	for _, field := range fields {
		if field == "PASS" {
			summary.Status = "PASS"
			continue
		}
		parts := strings.SplitN(field, "=", 2)
		if len(parts) != 2 {
			t.Fatalf("invalid summary field: %q", field)
		}
		switch parts[0] {
		case "records":
			summary.Records, _ = strconv.Atoi(parts[1])
		case "requests":
			summary.Requests, _ = strconv.Atoi(parts[1])
		case "peak":
			summary.Peak, _ = strconv.Atoi(parts[1])
		case "order":
			summary.Order = parts[1] == "true"
		}
	}
	if summary.Records != 100 || summary.Requests != 100 || summary.Peak <= 1 || summary.Peak > 4 || !summary.Order || summary.Status != "PASS" {
		t.Fatalf("unexpected summary: %q", line)
	}
}
