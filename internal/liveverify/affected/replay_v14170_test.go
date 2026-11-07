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
// `${heavy:+"$heavy" ...}` with a `$(dirname "$0")` default). heavy.sh sources
// a computed path, so both are unbounded readers, as
// .corvint/unbounded-readers.json records: they stay selected for a
// heavy.sh edit and for an unrelated one (AFP-V0-553). Before AFP-V0-553
// neither invocation was an edge, and an unrelated edit dropped both.
func TestHeavyShEditKeepsShellReaders_V14170(t *testing.T) {
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
			if !selected {
				t.Errorf("%s is not selected for a %s edit; scope %s", unit, dirty, plan.Scope)
			}
		}
	}
}
