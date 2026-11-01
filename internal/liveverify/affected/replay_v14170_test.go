package affected_test

import (
	"context"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/languages"
)

// TestHeavyShEditKeepsShellReaders_V14170: an edit to script/heavy.sh keeps
// the shell units analyzer-schema_test.sh and codex-review_test.sh selected,
// although Go tests in this repository also carry the literal
// "script/heavy.sh". V1-4170 suspected that literal dropped them; at this
// revision both stay selected, and the plan names one witness per unit, so a
// PATH_LITERAL_READER witness can stand where shell:dynamic-source was
// expected without the unit leaving the plan.
func TestHeavyShEditKeepsShellReaders_V14170(t *testing.T) {
	root := repositoryRoot(t)
	graph, err := affected.Build(context.Background(), root, languages.All()...)
	if err != nil {
		t.Fatal(err)
	}
	plan := affected.Select(graph, []string{"script/heavy.sh"})                                                  //corvint:fixture-path
	for _, unit := range []string{"shell:script/analyzer-schema_test.sh", "shell:script/codex-review_test.sh"} { //corvint:fixture-path
		selected := false
		for _, selection := range plan.Selected {
			selected = selected || selection.UnitID == unit
		}
		if !selected {
			t.Errorf("%s is not selected for a script/heavy.sh edit; scope %s", unit, plan.Scope)
		}
	}
}
