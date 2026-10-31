package affected_test

import (
	"context"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
	"github.com/Beamfall/corvint/internal/liveverify/affected/languages"
)

// TestForwardRowsChangeSelectsCIForwardRow_V13382: an edit to a
// .github/forward-rows script selects tools/ci-forward-row, whose tests run
// those scripts, instead of falling back to the whole repository. The shared
// source walk skips hidden directories, so verdict.sh was an owned shell file
// no unit declared (UNINDEXED_SOURCE_PATH, outside the AFP-V0-028
// allowlist); the shell plugin now indexes .github (NSA-V0-800). bar.jq,
// which no plugin owns, is pinned beside it. Fails on BASE for verdict.sh.
func TestForwardRowsChangeSelectsCIForwardRow_V13382(t *testing.T) {
	root := repositoryRoot(t)
	graph, err := affected.Build(context.Background(), root, languages.All()...)
	if err != nil {
		t.Fatal(err)
	}
	module, err := golang.ReadModulePath(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, dirty := range []string{".github/forward-rows/verdict.sh", ".github/forward-rows/bar.sh", ".github/forward-rows/bar.jq"} {
		plan := affected.Select(graph, []string{dirty})
		for _, unknown := range plan.Unknown {
			if unknown.Reason == affected.UnknownUnindexedSourcePath {
				t.Errorf("%s: plan has %s %s", dirty, unknown.Reason, unknown.Detail)
			}
		}
		selected := false
		for _, selection := range plan.Selected {
			selected = selected || selection.UnitID == "go:"+module+"/tools/ci-forward-row"
		}
		if !selected {
			t.Errorf("%s: tools/ci-forward-row is not selected; scope %s, unknown %+v", dirty, plan.Scope, plan.Unknown)
		}
	}
}
