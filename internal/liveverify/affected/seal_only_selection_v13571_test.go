package affected_test

import (
	"context"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/languages"
	"github.com/Beamfall/corvint/internal/testsupport"
)

// TestSealOnlyCEMDeltaStaysBounded_V13571 pins the delta a seal commit
// leaves on a batch branch: archived CEMs under .corvint/changes/ and
// nothing else. Every non-test shell script that reads that directory is
// declared in .corvint/shell-foreign-callers.json (TSP-V0-005), so the plan
// stays BOUNDED with no unknown, selects only the readers and their
// dependents, and excludes the rest of the graph (V1-3571). Before the seal,
// bind-coverage and install-lifecycle scripts were declared it carried
// shell:foreign-invocation-unmodelled. This file must not spell those scripts'
// file names: any candidate file naming one would have to be declared too.
//
// A real seal also deletes the checked CEM at .corvint/change.cem.json, which
// the TypeScript core tests read by literal: the plan then reaches the
// TypeScript plugin through a terminal selection, so AFP-V0-818 names every
// end-to-end site no declared runtime edge holds. The Playwright fixtures under
// Go testdata directories keep that site on their own unit (AFP-V0-725) and
// .corvint/runtime-edges.json models the rest (AFP-V0-819), so that delta stays
// BOUNDED too.
func TestSealOnlyCEMDeltaStaysBounded_V13571(t *testing.T) {
	testsupport.RequireLiveTier(t)
	graph, err := affected.Build(context.Background(), repositoryRoot(t), languages.All()...)
	if err != nil {
		t.Fatal(err)
	}
	var sealed []string
	for _, name := range batch1028cUnowned {
		if strings.HasPrefix(name, ".corvint/changes/") {
			sealed = append(sealed, name)
		}
	}
	withChecked := append([]string{".corvint/change.cem.json"}, sealed[:1]...)
	for _, dirty := range [][]string{sealed[:1], sealed, withChecked} {
		plan := affected.Select(graph, dirty)
		if plan.Scope != affected.ScopeBounded || len(plan.Unknown) != 0 {
			t.Errorf("%d sealed CEMs: scope %s, unknown %v; want BOUNDED with none", len(dirty), plan.Scope, plan.Unknown)
		}
		if len(plan.Selected) == 0 || len(plan.Excluded) == 0 {
			t.Errorf("%d sealed CEMs: %d selected, %d excluded; want the CEM readers only", len(dirty), len(plan.Selected), len(plan.Excluded))
		}
		for _, selection := range plan.Selected {
			switch selection.Witness.Kind {
			case affected.WitnessPathLiteralReader, affected.WitnessDeclaredReadScope, affected.WitnessDependency:
			case affected.WitnessFrontierSite:
				// A make target or job that invokes a computed name is
				// selected on every plan (AZU-V0-001).
				if selection.Kind != affected.SelectionKindCIJob && selection.Kind != affected.SelectionKindDriverTarget {
					t.Errorf("%s selected by %s with kind %q; want a driver selection", selection.UnitID, selection.Witness.Kind, selection.Kind)
				}
			default:
				t.Errorf("%s selected by %s; want a CEM reader or its dependent", selection.UnitID, selection.Witness.Kind)
			}
		}
	}
}
