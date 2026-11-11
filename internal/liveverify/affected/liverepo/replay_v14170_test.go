package liverepo_test

import (
	"context"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/languages"
	"github.com/Beamfall/corvint/internal/liveverify/affected/shell"
	"github.com/Beamfall/corvint/internal/testsupport"
)

// TestHeavyShEditKeepsShellReaders_V14170 retains the invocation edges from
// analyzer-schema_test.sh and codex-review_test.sh to script/heavy.sh, even
// through their computed wrapper paths (AFP-V0-553). AFP-V0-3340/3341 now
// proves heavy.sh's host-only environment source bounded, so these readers
// remain selected for a heavy.sh edit but not, through heavy.sh, for an
// unrelated docs edit. analyzer-schema_test.sh is itself a shell:tree-walk
// reader since AFP-V0-9973 (its git diff runs in a fixture repository the
// plugin cannot prove private), so a docs edit may select it only by that
// own unbounded read, never by a cause in heavy.sh.
func TestHeavyShEditKeepsShellReaders_V14170(t *testing.T) {
	testsupport.RequireLiveTier(t)
	root := repositoryRoot(t)
	graph, err := affected.Build(context.Background(), root, languages.All()...)
	if err != nil {
		t.Fatal(err)
	}
	// The invocation edges themselves, which the tree-walk witness of
	// analyzer-schema_test.sh would otherwise hide on a heavy.sh edit.
	for _, id := range []string{"shell:script/analyzer-schema_test.sh", "shell:script/codex-review_test.sh"} { //corvint:fixture-path
		if !importsReach(graph, id, "shell:script/heavy.sh") { //corvint:fixture-path
			t.Errorf("%s has no import chain to script/heavy.sh", id)
		}
	}
	for _, dirty := range []string{"script/heavy.sh", "docs/AUTOMATION.md"} { //corvint:fixture-path
		plan := affected.Select(graph, []string{dirty})
		for _, unit := range []string{"shell:script/analyzer-schema_test.sh", "shell:script/codex-review_test.sh"} { //corvint:fixture-path
			selected := false
			for _, selection := range plan.Selected {
				if selection.UnitID != unit {
					continue
				}
				own := selection.Witness.Kind == affected.WitnessUnboundedReader && selection.Witness.Cause != nil &&
					selection.Witness.Cause.Unit == unit && selection.Witness.Cause.Reads == shell.UnboundedTreeWalk
				selected = selected || dirty == "script/heavy.sh" || !(own && unit == "shell:script/analyzer-schema_test.sh") //corvint:fixture-path
			}
			if want := dirty == "script/heavy.sh"; selected != want {
				t.Errorf("%s selected=%v for a %s edit, want %v; scope %s", unit, selected, dirty, want, plan.Scope)
			}
		}
	}
}

// importsReach reports an import chain from one unit to another.
func importsReach(graph *affected.Graph, from, to string) bool {
	seen := map[string]bool{from: true}
	queue := []string{from}
	for len(queue) > 0 {
		unit, ok := graph.Unit(queue[0])
		queue = queue[1:]
		if !ok {
			continue
		}
		for _, next := range unit.Imports {
			if next == to {
				return true
			}
			if !seen[next] {
				seen[next] = true
				queue = append(queue, next)
			}
		}
	}
	return false
}
