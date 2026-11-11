// SPDX-License-Identifier: Apache-2.0
package gateselect

import (
	"context"
	"reflect"
	"slices"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
)

// TestQualifiedUnownedPathIsThePlannersVerdict_RNCV1004: the qualified
// selector re-observes Go alone, so its proof graph names every non-Go dirty
// path UNOWNED_DIRTY_PATH. Whether a runner instance owns the path is the
// planner's verdict (RNC-V1-004, owner decision R32): a path a declared runner
// owns, which the planner therefore does not mark unowned and no Go rule
// attributes, keeps the Go selection narrowed, while a path the planner itself
// marks UNOWNED_DIRTY_PATH and no Go rule attributes still runs every test.
func TestQualifiedUnownedPathIsThePlannersVerdict_RNCV1004(t *testing.T) {
	root, plan := qualifiedFixture(t)
	plan.Plan.Dirty = []string{"core/core.go", "web/app.ts"}
	graph, err := affected.BuildPartial(context.Background(), root, golang.New())
	if err != nil {
		t.Fatal(err)
	}
	if proof := affected.Select(graph, plan.Plan.Dirty); !slices.ContainsFunc(proof.Unknown, func(u affected.Unknown) bool {
		return u.Reason == affected.UnknownUnownedDirtyPath && u.Detail == "web/app.ts"
	}) {
		t.Fatalf("the Go-only proof no longer marks web/app.ts unowned, so the fixture proves nothing: %+v", proof.Unknown)
	}
	check := func(plan Receipt, want string) {
		t.Helper()
		direct := SelectQualified(plan, fixtureModule, root)
		indexed := SelectQualifiedIndexed(plan, fixtureModule, root, StartIndex(root, fixtureModule, nil))
		if !reflect.DeepEqual(direct, indexed) {
			t.Fatalf("consumer divergence:\n%v\n%v", direct, indexed)
		}
		if got := direct[len(direct)-1]; got != want {
			t.Fatalf("verdict %q, want %q; audit=%v", got, want, direct)
		}
	}
	// web/app.ts is a declared runner's: the planner names no unknown for it.
	check(plan, "run "+fixtureModule+"/user")
	unowned := plan
	unowned.Plan.Scope = "UNKNOWN"
	unowned.Plan.Unknown = append(unowned.Plan.Unknown, struct {
		Reason string `json:"reason"`
		Detail string `json:"detail"`
	}{"UNOWNED_DIRTY_PATH", "web/app.ts"})
	check(unowned, "FALLBACK "+UnownedPathFallback+": web/app.ts")
}
