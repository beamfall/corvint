package affected_test

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
	"github.com/Beamfall/corvint/internal/liveverify/affected/languages"
)

var narrowingGraph struct {
	once   sync.Once
	graph  *affected.Graph
	module string
	err    error
}

// repositoryNarrowingGraph builds this repository's graph with every planned
// language once for the TSP-V0-900 and TSP-V0-901 tests, which read the real
// declarations under .corvint/.
func repositoryNarrowingGraph(t *testing.T) (*affected.Graph, string) {
	t.Helper()
	narrowingGraph.once.Do(func() {
		root := repositoryRoot(t)
		narrowingGraph.graph, narrowingGraph.err = affected.Build(context.Background(), root, languages.All()...)
		if narrowingGraph.err == nil {
			narrowingGraph.module, narrowingGraph.err = golang.ReadModulePath(root)
		}
	})
	if narrowingGraph.err != nil {
		t.Fatal(narrowingGraph.err)
	}
	return narrowingGraph.graph, narrowingGraph.module
}

// selectedUnits maps each unit the plan for the dirty paths selects to its
// witness kind, with Go package ids shortened to their repository directory.
func selectedUnits(graph *affected.Graph, module string, dirty ...string) map[string]string {
	units := map[string]string{}
	for _, selection := range affected.Select(graph, dirty).Selected {
		units[strings.TrimPrefix(selection.UnitID, "go:"+module+"/")] = selection.Witness.Kind
	}
	return units
}

// TestDeclaredReadScopesNarrowSpecAndRequirementsPlans_TSP_V0_900: the read
// scopes this repository declares for test units whose path literals name
// temporary fixtures, or that run a toolchain over a bounded part of the tree,
// keep those units out of a plan for one spec edit and for a
// REQUIREMENTS.tsv edit, while a unit that reads every spec, or the
// requirements index, stays selected (V1-3763, V1-3587, V1-3788).
func TestDeclaredReadScopesNarrowSpecAndRequirementsPlans_TSP_V0_900(t *testing.T) {
	graph, module := repositoryNarrowingGraph(t)
	narrowed := []string{
		"internal/perfbudget", "internal/cemchain", "conformance/cli-parity-v0",
		"conformance/mcp-2026-07-28", "internal/jstestprovider", "internal/tasks/ticket",
		"protocol/integrations", "tools/ci-forward-row", "tools/requirement-id-collisions",
		"shell:script/gate-affected_test.sh", "shell:script/dogfood-change_test.sh",
		"shell:script/vscode-check-install_test.sh", "shell:script/check-install-lifecycle_test.sh",
	}
	for _, test := range []struct {
		dirty    string
		narrowed []string
		readers  []string
	}{
		{"docs/specs/test-selection-precision-v0.md", narrowed, []string{"internal/specindex", "internal/lrfrepo"}},
		{"docs/specs/REQUIREMENTS.tsv", append([]string{"internal/compactionkernel", "internal/dashboard/roadmap",
			"internal/sharedpath", "script/spec-coverage-audit"}, narrowed...), []string{"internal/lrfrepo"}},
	} {
		units := selectedUnits(graph, module, test.dirty)
		for _, unit := range test.narrowed {
			if kind, selected := units[unit]; selected {
				t.Errorf("%s: %s selected as %s; its declared read scope excludes the path", test.dirty, unit, kind)
			}
		}
		for _, unit := range test.readers {
			if _, selected := units[unit]; !selected {
				t.Errorf("%s: true reader %s not selected", test.dirty, unit)
			}
		}
	}
}

// TestDeclaredReadScopesKeepTrueReaders_TSP_V0_901: each narrowed unit stays
// selected for a path its declared read scope names, so a declaration that
// drops a real read fails here instead of leaving the reader unselected.
func TestDeclaredReadScopesKeepTrueReaders_TSP_V0_901(t *testing.T) {
	graph, module := repositoryNarrowingGraph(t)
	for _, test := range []struct{ dirty, unit string }{
		{"docs/specs/mcp-server-2026-07-28-v0.md", "conformance/mcp-2026-07-28"},
		{"internal/taskman/testdata/issue502-optional-keys-record.json", "internal/tasks/ticket"},
		{".github/forward-rows/failed.jq", "tools/ci-forward-row"},
		{".github/cishards/package-costs.json", "internal/perfbudget"},
		{"integrations/opencode/README.md", "internal/opencodequalification"},
		{"internal/capcatalog/trees/corvint-cem-candidate.json", "cmd/corvint-postmerge-workflow"},
		{"examples/evidence-provider/v0/main.go", "examples/evidence-provider/v0/conformance"},
		{"Makefile", "shell:script/dogfood-cache_test.sh"},
		{"Makefile", "shell:script/core-n1-replay-default_test.sh"},
		{"Makefile", "shell:script/spec-requirements-conflict_test.sh"},
		{"extensions/vscode/biome.json", "shell:script/vscode-check-install_test.sh"},
		{"tools/gate-affected-select/main.go", "shell:script/gate-affected_test.sh"},
		{"docs/DOGFOOD.md", "shell:script/dogfood-change_test.sh"},
	} {
		if _, selected := selectedUnits(graph, module, test.dirty)[test.unit]; !selected {
			t.Errorf("%s: declared reader %s not selected", test.dirty, test.unit)
		}
	}
}
