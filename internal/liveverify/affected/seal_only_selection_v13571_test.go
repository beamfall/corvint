package affected_test

import (
	"context"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/languages"
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
func TestSealOnlyCEMDeltaStaysBounded_V13571(t *testing.T) {
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
	for _, dirty := range [][]string{sealed[:1], sealed} {
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
			default:
				t.Errorf("%s selected by %s; want a CEM reader or its dependent", selection.UnitID, selection.Witness.Kind)
			}
		}
	}
}
