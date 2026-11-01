package affected_test

import (
	"context"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/languages"
)

// TestSealCommitPlanIsBounded_V13898 replays a seal commit's dirty set
// (f31dd540e..8dca05f13: the bound CEM moved from .corvint/change.cem.json
// into .corvint/changes/) against
// this repository's graph with every plugin. Before V1-3898 it was UNKNOWN
// with 18 entries: shell:foreign-invocation-unmodelled from the undeclared
// dogfood and CEM scripts that name the CEM paths (now declared in
// .corvint/shell-foreign-callers.json, TSP-V0-005), and 17
// typescript:e2e-runtime-dependency entries from a terminal TypeScript
// reader of the CEM path (AFP-V0-818), which a dirty set of declared data
// alone no longer names (AFP-V0-987).
func TestSealCommitPlanIsBounded_V13898(t *testing.T) {
	graph, err := affected.Build(context.Background(), repositoryRoot(t), languages.All()...)
	if err != nil {
		t.Fatal(err)
	}
	plan := affected.Select(graph, []string{".corvint/change.cem.json", ".corvint/changes/f31dd540ea3447aa439103e2470a71198ce5171f.cem.json"})
	for _, entry := range plan.Unknown {
		t.Errorf("%s %s (hint %q)", entry.Reason, entry.Detail, entry.Hint)
	}
	if plan.Scope == "UNKNOWN" {
		t.Errorf("scope %s", plan.Scope)
	}
}
