package affected_test

import (
	"context"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/languages"
)

// TestHeavyShEditKeepsShellReaders_V14170: the shell units
// analyzer-schema_test.sh and codex-review_test.sh run script/heavy.sh through
// a variable (`heavy=${...-script/heavy.sh}` run in a `sh -c` string, and
// `${heavy:+"$heavy" ...}` with a `$(dirname "$0")` default), so both are
// selected for a heavy.sh edit through that invocation edge (AFP-V0-553).
// Before AFP-V0-553 neither invocation was an edge. Until V1-4283 heavy.sh
// also read as a dynamic source, which made both unbounded readers selected
// for an unrelated edit too; AFP-V0-2953 and AFP-V0-2955 bound every source
// heavy.sh holds, so an unrelated edit now selects neither.
func TestHeavyShEditKeepsShellReaders_V14170(t *testing.T) {
	root := repositoryRoot(t)
	graph, err := affected.Build(context.Background(), root, languages.All()...)
	if err != nil {
		t.Fatal(err)
	}
	for _, dirty := range []string{"script/heavy.sh", "docs/AUTOMATION.md"} { //corvint:fixture-path
		plan := affected.Select(graph, []string{dirty})
		want := dirty == "script/heavy.sh"
		for _, unit := range []string{"shell:script/analyzer-schema_test.sh", "shell:script/codex-review_test.sh"} { //corvint:fixture-path
			selected := false
			for _, selection := range plan.Selected {
				selected = selected || selection.UnitID == unit
			}
			if selected != want {
				t.Errorf("%s selected=%v for a %s edit, want %v; scope %s", unit, selected, dirty, want, plan.Scope)
			}
		}
	}
}
