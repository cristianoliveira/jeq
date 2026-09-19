package examples_test

import (
	"bytes"
	"encoding/json"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

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
