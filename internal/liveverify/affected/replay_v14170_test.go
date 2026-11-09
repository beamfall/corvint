package affected_test

import (
	"context"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/languages"
	"github.com/Beamfall/corvint/internal/testsupport"
)

// TestHeavyShEditKeepsShellReaders_V14170 retains the invocation edges from
// analyzer-schema_test.sh and codex-review_test.sh to script/heavy.sh, even
// through their computed wrapper paths (AFP-V0-553). AFP-V0-3340/3341 now
// proves heavy.sh's host-only environment source bounded, so these readers
// remain selected for a heavy.sh edit but not for an unrelated docs edit.
func TestHeavyShEditKeepsShellReaders_V14170(t *testing.T) {
	testsupport.RequireLiveTier(t)
	root := repositoryRoot(t)
	graph, err := affected.Build(context.Background(), root, languages.All()...)
	if err != nil {
		t.Fatal(err)
	}
	for _, dirty := range []string{"script/heavy.sh", "docs/AUTOMATION.md"} { //corvint:fixture-path
		plan := affected.Select(graph, []string{dirty})
		for _, unit := range []string{"shell:script/analyzer-schema_test.sh", "shell:script/codex-review_test.sh"} { //corvint:fixture-path
			selected := false
			for _, selection := range plan.Selected {
				selected = selected || selection.UnitID == unit
			}
			if want := dirty == "script/heavy.sh"; selected != want {
				t.Errorf("%s selected=%v for a %s edit, want %v; scope %s", unit, selected, dirty, want, plan.Scope)
			}
		}
	}
}
