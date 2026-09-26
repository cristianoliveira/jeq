package examples_test

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadableWorkflowSnippetsUseRootFixtures(t *testing.T) {
	fakeDir := t.TempDir()
	fake := filepath.Join(fakeDir, "jeq")
	require.NoError(t, os.WriteFile(fake, []byte("#!/bin/sh\ncat\n"), 0o700))
	for _, tc := range []struct{ name, fixture, questions string }{
		{"release", "examples/readable-workflows/release/fixtures/release.json", "examples/readable-workflows/release/questions.json"},
		{"support", "examples/readable-workflows/support/fixtures/ticket.json", "examples/readable-workflows/support/questions.json"},
		{"incident", "examples/readable-workflows/incident/fixtures/incident.json", "examples/readable-workflows/incident/category-questions.json"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := exec.Command("bash", "-c", "< "+tc.fixture+" jeq map --as example --questions "+tc.questions+" --state-pointer /change")
			cmd.Dir, cmd.Env = repoRoot, append(os.Environ(), "PATH="+fakeDir+string(os.PathListSeparator)+os.Getenv("PATH"))
			out, err := cmd.Output()
			require.NoError(t, err, "snippet output=%q", out)
			assert.NotEmpty(t, out)
		})
	}
}

func TestIncidentCatalogJQKnownAndUnknownKeys(t *testing.T) {
	catalog := filepath.Join(repoRoot, "examples/readable-workflows/incident/runbook-catalog.json")
	filter := `
    (._jeq.category.answers.category.choice // error("missing category answer")) as $category |
    ($catalog[0][$category] // error("unknown catalog category: " + $category)) as $criteria |
    if (($criteria | type) != "object" or ($criteria | length) == 0) then
      error("empty catalog category: " + $category)
    else
      .request = {"model":"jev-latest","state":.incident,"questions":{"runbook":{"type":"choice","instructions":("Select the approved runbook for category " + $category),"criteria":$criteria}}}
    end`
	known := []byte(`{"incident":{"id":"INC-1"},"_jeq":{"category":{"answers":{"category":{"choice":"technical"}}}}}`)
	cmd := exec.Command("jq", "-c", "--slurpfile", "catalog", catalog, filter)
	cmd.Stdin = bytes.NewReader(known)
	out, err := cmd.Output()
	require.NoError(t, err, "known category jq")
	var document struct {
		Request struct {
			Questions map[string]struct {
				Criteria map[string]string `json:"criteria"`
			} `json:"questions"`
		} `json:"request"`
	}
	require.NoError(t, json.Unmarshal(out, &document))
	assert.NotEmpty(t, document.Request.Questions["runbook"].Criteria["technical-api-outage"], "known catalog lookup missing from %s", out)

	unknown := bytes.Replace(known, []byte(`"technical"`), []byte(`"unknown"`), 1)
	cmd = exec.Command("jq", "-c", "--slurpfile", "catalog", catalog, filter)
	cmd.Stdin = bytes.NewReader(unknown)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err = cmd.Run()
	assert.Error(t, err, "unknown category unexpectedly succeeded")
	assert.Contains(t, stderr.String(), "unknown catalog category", "unknown category diagnostic")
}
