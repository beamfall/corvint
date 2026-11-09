// SPDX-License-Identifier: Apache-2.0
package affected_test

import (
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// AFP-V0-3360, AFP-V0-3361: fixture-only dashboard readers have declared
// scopes. Unrelated record additions leave them out, while their fixtures,
// source and the producer they build still select them.
func TestUnboundedDashboardScopes_AFPV03360(t *testing.T) {
	graph, module := repositoryNarrowingGraph(t)
	for _, dirty := range []string{
		"docs/build-log/2026-11-09-probe.md",                                 //corvint:fixture-path
		".corvint/changes/5180518051805180518051805180518051805180.cem.json", //corvint:fixture-path
	} {
		selected := selectedUnits(graph, module, dirty)
		for _, directory := range []string{"internal/dashboard/repository", "conformance/dashboard-snapshot-v0"} {
			if witness, ok := selected[directory]; ok {
				t.Errorf("%s: bounded fixture reader %s selected as %s", dirty, directory, witness)
			}
		}
		// No blanket build-log exclusion: these inspect the tracked tree
		// and must remain selected. CEM sidecars already have a separate
		// repository-owned exclusion. (The offload runner's ENVFILE source
		// is host-only since AFP-V0-3340/3341, so its callers no longer
		// carry a dynamic-source closure into the tree.)
		if dirty == "docs/build-log/2026-11-09-probe.md" { //corvint:fixture-path
			for _, id := range []string{"internal/doccorpus/selfcorpus", "tools/test-read-scopes-check"} {
				if _, ok := selected[id]; !ok {
					t.Errorf("%s: unresolved/whole-tree reader %s lost", dirty, id)
				}
			}
		}
	}
	for _, tc := range []struct{ dirty, reader string }{
		{"internal/dashboard/repository/testdata/process_helper/main.go", "internal/dashboard/repository"},
		{"internal/dashboard/repository/filesystem.go", "internal/dashboard/repository"},
		{"internal/secretscreen/testdata/parity.json", "conformance/dashboard-snapshot-v0"},
		{"cmd/corvint-dashboard-snapshot/main.go", "conformance/dashboard-snapshot-v0"},
		{"go.mod", "conformance/dashboard-snapshot-v0"},
	} {
		if _, ok := selectedUnits(graph, module, tc.dirty)[tc.reader]; !ok {
			t.Errorf("%s: true reader %s lost", tc.dirty, tc.reader)
		}
	}
}

// AFP-V0-3364: the ledger reporter names its writer only in a comment. The
// existing foreign-caller declaration must not turn that mention into the
// writer's arbitrary execution closure; real calls still keep it unbounded.
func TestUnboundedLedgerMention_AFPV03364(t *testing.T) {
	graph, module := repositoryNarrowingGraph(t)
	const reporter = "driver:make:Makefile#dogfood-change-runs"
	selected := selectedUnits(graph, module, "docs/build-log/2026-11-09-probe.md") //corvint:fixture-path
	if chain, reads := graph.UnboundedChain(reporter); len(chain) != 0 {
		t.Errorf("comment-only reference retained an unbounded chain %v reading %s", chain, reads)
	}
	// The target can still be selected by its independent computed make
	// argument. That frontier belongs to the separate driver lane.
	if _, ok := selected["driver:make:Makefile#dogfood-change"]; !ok {
		t.Error("the real writer lost its runtime closure")
	}
	if _, ok := selectedUnits(graph, module, "script/dogfood-change-runs.sh")[reporter]; !ok {
		t.Error("changing the reporter no longer selects its make target")
	}
	for _, unit := range affected.Select(graph, []string{"docs/build-log/2026-11-09-probe.md"}).Unknown { //corvint:fixture-path
		if unit.Detail == "shell:foreign-callers-invalid" {
			t.Fatal("foreign-caller partition failed its completeness gate")
		}
	}
}
