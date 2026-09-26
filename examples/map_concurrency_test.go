package examples_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMapConcurrencyExampleUsesBuiltBinary(t *testing.T) {
	toolDir := t.TempDir()
	require.NoError(t, os.Symlink(jeqBin, filepath.Join(toolDir, "jeq")))
	cmd := exec.Command("go", "run", "./examples/map-concurrency", "--records", "100")
	cmd.Dir = repoRoot
	cmd.Env = append(os.Environ(), "PATH="+toolDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "example output: %s", output)
	line := strings.TrimSpace(string(output))
	var summary struct {
		Records  int    `json:"records"`
		Requests int    `json:"requests"`
		Peak     int    `json:"peak"`
		Order    bool   `json:"order"`
		Status   string `json:"status"`
	}
	fields := strings.Fields(line)
	require.Len(t, fields, 6, "summary=%q", line)
	for _, field := range fields {
		if field == "PASS" {
			summary.Status = "PASS"
			continue
		}
		parts := strings.SplitN(field, "=", 2)
		require.Len(t, parts, 2, "invalid summary field: %q", field)
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
	assert.Equal(t, 100, summary.Records, "summary=%q", line)
	assert.Equal(t, 100, summary.Requests, "summary=%q", line)
	assert.Greater(t, summary.Peak, 1, "summary=%q", line)
	assert.LessOrEqual(t, summary.Peak, 4, "summary=%q", line)
	assert.True(t, summary.Order, "summary=%q", line)
	assert.Equal(t, "PASS", summary.Status, "summary=%q", line)
}
