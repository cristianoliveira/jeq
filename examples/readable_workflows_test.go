package examples_test

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadableWorkflowSnippetsUseRootFixtures(t *testing.T) {
	fakeDir := t.TempDir()
	fake := filepath.Join(fakeDir, "jeq")
	if err := os.WriteFile(fake, []byte("#!/bin/sh\ncat\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ name, fixture, questions string }{
		{"release", "examples/readable-workflows/release/fixtures/release.json", "examples/readable-workflows/release/questions.json"},
		{"support", "examples/readable-workflows/support/fixtures/ticket.json", "examples/readable-workflows/support/questions.json"},
		{"incident", "examples/readable-workflows/incident/fixtures/incident.json", "examples/readable-workflows/incident/category-questions.json"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := exec.Command("bash", "-c", "< "+tc.fixture+" jeq map --as example --questions "+tc.questions+" --state-pointer /change")
			cmd.Dir, cmd.Env = repoRoot, append(os.Environ(), "PATH="+fakeDir+string(os.PathListSeparator)+os.Getenv("PATH"))
			if out, err := cmd.Output(); err != nil || len(out) == 0 {
				t.Fatalf("snippet failed: err=%v output=%q", err, out)
			}
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
	if err != nil {
		t.Fatalf("known category jq failed: %v", err)
	}
	var document struct {
		Request struct {
			Questions map[string]struct {
				Criteria map[string]string `json:"criteria"`
			} `json:"questions"`
		} `json:"request"`
	}
	if err := json.Unmarshal(out, &document); err != nil {
		t.Fatal(err)
	}
	if document.Request.Questions["runbook"].Criteria["technical-api-outage"] == "" {
		t.Fatalf("known catalog lookup missing from %s", out)
	}

	unknown := bytes.Replace(known, []byte(`"technical"`), []byte(`"unknown"`), 1)
	cmd = exec.Command("jq", "-c", "--slurpfile", "catalog", catalog, filter)
	cmd.Stdin = bytes.NewReader(unknown)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err == nil || !strings.Contains(stderr.String(), "unknown catalog category") {
		t.Fatalf("unknown category unexpectedly succeeded: err=%v stderr=%q", err, stderr.String())
	}
}
