// Package arch guards the ADR 0002 dependency rules:
//
//	domain → standard library only
//	infra  → domain
//	cli    → domain
//	main   → cli + infra
//
// and forbids every import of cmd/jeq. Violations fail the normal gate.
package arch_test

import (
	"encoding/json"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const modulePath = "github.com/cristianoliveira/jeq"

// graph maps a package import path to its direct imports.
type graph map[string][]string

// ring classifies a package into an architecture ring.
type ring string

const (
	ringDomain   ring = "domain"
	ringInfra    ring = "infra"
	ringCLI      ring = "cli"
	ringMain     ring = "main"
	ringInternal ring = "internal-other" // known internal helpers (e.g. fixtures)
	ringExternal ring = "external"       // third-party modules
	ringStdlib   ring = "stdlib"
)

func classify(importPath string) ring {
	switch importPath {
	case modulePath + "/cmd/jeq":
		return ringMain
	case modulePath + "/internal/cli":
		return ringCLI
	}
	switch {
	case strings.HasPrefix(importPath, modulePath+"/internal/domain/"):
		return ringDomain
	case strings.HasPrefix(importPath, modulePath+"/internal/infra/"):
		return ringInfra
	case strings.HasPrefix(importPath, modulePath+"/internal/"):
		return ringInternal
	case strings.HasPrefix(importPath, modulePath+"/"):
		return ringMain // unknown top-level module package; treat as wiring
	case strings.Contains(strings.SplitN(importPath, "/", 2)[0], "."):
		return ringExternal
	default:
		return ringStdlib
	}
}

// violations returns every edge that breaks the ADR 0002 rules, one message each.
func violations(g graph) []string {
	var bad []string
	for pkg, imports := range g {
		from := classify(pkg)

		// No package of any ring imports the composition root.
		for _, imp := range imports {
			if classify(imp) == ringMain {
				bad = append(bad, pkg+" imports cmd/jeq (forbidden by ADR 0002)")
			}
		}

		for _, imp := range imports {
			to := classify(imp)
			if !edgeAllowed(from, to) {
				bad = append(bad, pkg+" ("+string(from)+") imports "+imp+" ("+string(to)+")")
			}
		}
	}
	return bad
}

func edgeAllowed(from, to ring) bool {
	switch from {
	case ringDomain:
		// domain → standard library only.
		return to == ringStdlib || to == ringDomain || to == ringInternal
	case ringInfra:
		// infra → domain (plus stdlib, third-party, and sibling infra).
		return to != ringCLI && to != ringMain
	case ringCLI:
		// cli → domain (plus stdlib and third-party such as Cobra).
		return to != ringInfra && to != ringMain
	case ringMain:
		// main → cli + infra.
		return to == ringCLI || to == ringInfra || to == ringStdlib || to == ringExternal
	default:
		return true
	}
}

func TestImportRules(t *testing.T) {
	domain := modulePath + "/internal/domain/jeq"
	infra := modulePath + "/internal/infra/render"
	cli := modulePath + "/internal/cli"
	main := modulePath + "/cmd/jeq"

	tests := []struct {
		name  string
		graph graph
		want  []string // expected violation substrings; empty means no violation
	}{
		{
			name:  "domain may import stdlib and sibling domain packages",
			graph: graph{domain: {"fmt", modulePath + "/internal/domain/contract"}},
			want:  nil,
		},
		{
			name:  "domain must not import third-party modules",
			graph: graph{domain: {"github.com/spf13/cobra"}},
			want:  []string{"imports github.com/spf13/cobra"},
		},
		{
			name:  "domain must not import infra",
			graph: graph{domain: {infra}},
			want:  []string{domain + " (domain) imports " + infra},
		},
		{
			name:  "domain must not import cli",
			graph: graph{domain: {cli}},
			want:  []string{"imports " + cli},
		},
		{
			name:  "infra may import domain and stdlib",
			graph: graph{infra: {domain, "net/http"}},
			want:  nil,
		},
		{
			name:  "infra must not import cli",
			graph: graph{infra: {cli}},
			want:  []string{"imports " + cli},
		},
		{
			name:  "cli may import domain and third-party",
			graph: graph{cli: {domain, "github.com/spf13/cobra"}},
			want:  nil,
		},
		{
			name:  "cli must not import infra",
			graph: graph{cli: {infra}},
			want:  []string{"imports " + infra},
		},
		{
			name:  "main may import cli and infra",
			graph: graph{main: {cli, infra}},
			want:  nil,
		},
		{
			name:  "main must not import domain",
			graph: graph{main: {domain}},
			want:  []string{"imports " + domain},
		},
		{
			name:  "no package may import main",
			graph: graph{cli: {main}},
			want:  []string{"imports cmd/jeq", cli + " (cli) imports " + main},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := violations(tt.graph)
			require.Len(t, got, len(tt.want), "expected violations %v, got %v", tt.want, got)
			for i, want := range tt.want {
				assert.Contains(t, got[i], want, "violation %d", i)
			}
		})
	}
}

// TestModuleGraphRespectsImportRules runs the same rules against the real
// module graph reported by `go list -json ./...`.
func TestModuleGraphRespectsImportRules(t *testing.T) {
	out, err := exec.Command("go", "list", "-json", "./...").Output()
	require.NoError(t, err)

	g := graph{}
	dec := json.NewDecoder(strings.NewReader(string(out)))
	type listPkg struct {
		ImportPath string
		Imports    []string
	}
	for dec.More() {
		var p listPkg
		require.NoError(t, dec.Decode(&p))
		g[p.ImportPath] = p.Imports
	}
	require.NotEmpty(t, g, "go list must report a non-empty module graph")
	bad := violations(g)
	assert.Empty(t, bad, "module graph violates ADR 0002:\n%s", strings.Join(bad, "\n"))
}
