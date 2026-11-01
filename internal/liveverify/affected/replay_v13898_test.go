package affected_test

import (
	"context"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/languages"
)

// sealPair is the dirty path set of a real dogfood seal (V1-3898): bind
// commit f31dd540e adds the shared bind CEM and seal commit 8dca05f13 moves it
// to its sealed path, so the bind-to-seal range deletes the one and adds the
// other, and a lane range that squashes both adds only the sealed CEM.
var sealPair = map[string][]string{
	"bind..seal": {".corvint/change.cem.json", ".corvint/changes/f31dd540ea3447aa439103e2470a71198ce5171f.cem.json"},
	"lane..seal": {".corvint/changes/f31dd540ea3447aa439103e2470a71198ce5171f.cem.json"},
}

// TestSealCommitPairIsBounded_AFPV0921 replays that seal pair against this
// repository's graph with every plugin (AFP-V0-921, V1-3898). Both CEM paths
// are declared free of unbounded readers (AFP-V0-772), so the plan is BOUNDED
// with no unknown: the test-less shell readers of the CEM paths carry no
// users, the TypeScript cross-language entries do not bear, and the shell
// readers' own foreign-invocation entries are covered by their declared
// callers (TSP-V0-005). The literal readers are still selected. A further
// dirty path that the exclusions do not cover still makes the plan follow
// the rules the exclusion lifts.
func TestSealCommitPairIsBounded_AFPV0921(t *testing.T) {
	graph, err := affected.Build(context.Background(), repositoryRoot(t), languages.All()...)
	if err != nil {
		t.Fatal(err)
	}
	for name, dirty := range sealPair {
		plan := affected.Select(graph, dirty)
		if plan.Scope != affected.ScopeBounded || len(plan.Unknown) != 0 {
			t.Errorf("%s: scope %s, unknown %+v", name, plan.Scope, plan.Unknown)
		}
		selected := map[string]bool{}
		for _, selection := range plan.Selected {
			selected[selection.UnitID] = true
		}
		for _, reader := range []string{"go:github.com/Beamfall/corvint/cmd/corvint", "shell:script/dogfood-change_test.sh"} {
			if !selected[reader] {
				t.Errorf("%s: literal reader %s not selected", name, reader)
			}
		}
	}
	// The read-scope declaration is a runtime-readers path the exclusions do
	// not name, so every unbounded reader is selected again.
	widened := affected.Select(graph, append([]string{".corvint/test-read-scopes.json"}, sealPair["lane..seal"]...))
	if len(widened.Selected) <= len(affected.Select(graph, sealPair["lane..seal"]).Selected) {
		t.Errorf("a dirty path the exclusions do not cover selected no more units: %d", len(widened.Selected))
	}
}
