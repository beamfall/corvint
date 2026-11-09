// SPDX-License-Identifier: Apache-2.0
package affected_test

import (
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

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
