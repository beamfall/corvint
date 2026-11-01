package affected_test

import (
	"context"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
	"github.com/Beamfall/corvint/internal/liveverify/affected/languages"
)

// TestCloseoutRecipeEditSelectsItsTests_V14058 (ASD-V0-946): an edit to the
// dogfood-closeout recipe in the Makefile, or to the shell test that pins it,
// selects script/dogfood-cache_test.sh and tools/test-commands-check, whose Go
// test runs that pin in CI's Go shards (796520b19 left main red when no CI job
// ran the shell test).
func TestCloseoutRecipeEditSelectsItsTests_V14058(t *testing.T) {
	root := repositoryRoot(t)
	graph, err := affected.Build(context.Background(), root, languages.All()...)
	if err != nil {
		t.Fatal(err)
	}
	module, err := golang.ReadModulePath(root)
	if err != nil {
		t.Fatal(err)
	}
	wants := map[string][]string{
		"Makefile":                     {"shell:script/dogfood-cache_test.sh", "go:" + module + "/tools/test-commands-check"},
		"script/dogfood-cache_test.sh": {"shell:script/dogfood-cache_test.sh", "go:" + module + "/tools/test-commands-check"},
	}
	for dirty, units := range wants {
		plan := affected.Select(graph, []string{dirty})
		selected := map[string]bool{}
		for _, selection := range plan.Selected {
			selected[selection.UnitID] = true
		}
		for _, unit := range units {
			if !selected[unit] {
				t.Errorf("%s: %s is not selected; scope %s, unknown %+v", dirty, unit, plan.Scope, plan.Unknown)
			}
		}
	}
}
