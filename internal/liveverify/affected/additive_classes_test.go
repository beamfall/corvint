// SPDX-License-Identifier: Apache-2.0
package affected

import (
	"slices"
	"testing"
)

// TestReachedThroughCoversEveryTraversalClass_AFPV03142 holds additive
// narrowing sound for every edge class the dependency walk follows
// (AFP-V0-3142). For each class of traversalEdges, the list traverseWhere
// walks, it adds one edge of that class from a changed unit (`x:dep`) to the
// narrowed package and checks that the walk reaches the package through it,
// that the record falls back naming `x:dep`, and that the importer, its
// importer and the test user the record would exclude are selected. A class
// added to the walk is added to this loop with it. Without the edge the same
// dirty set keeps the record NARROWED, so the edge alone demotes it.
func TestReachedThroughCoversEveryTraversalClass_AFPV03142(t *testing.T) {
	t.Parallel()
	language := additiveFake()
	language.units = append(language.units, Unit{ID: "x:dep", Sources: []string{"dep/dep.x"}, Tests: []string{"dep/dep_test.x"}})
	dirty := []string{"core/fresh.x", "dep/dep.x"}

	control := Select(mustBuild(t, language).WithGoAdditive([]GoAdditive{narrowedCore()}), dirty)
	if len(control.GoAdditive) != 1 || control.GoAdditive[0].State != GoAdditiveNarrowed {
		t.Fatalf("control record %+v, want NARROWED: the fixture must not reach x:core without the injected edge", control.GoAdditive)
	}

	classes := len(mustBuild(t, language).traversalEdges())
	if classes != 4 {
		t.Errorf("the walk follows %d edge classes; this test and the AFP-V0-3142 row name four (import, advisory, route dispatch, code run)", classes)
	}
	for class := range classes {
		graph := mustBuild(t, language)
		edges := graph.traversalEdges()[class]
		edges["x:dep"] = append(edges["x:dep"], "x:core")
		if _, reached := graph.traverse(map[string]Witness{"x:dep": {Kind: WitnessDirectSource, DirtyPath: "dep/dep.x"}}, nil)["x:core"]; !reached {
			t.Errorf("class %d: the walk does not follow the injected edge", class)
			continue
		}
		plan := Select(graph.WithGoAdditive([]GoAdditive{narrowedCore()}), dirty)
		if len(plan.GoAdditive) != 1 || plan.GoAdditive[0].State != GoAdditiveFallback || plan.GoAdditive[0].Reason != GoAdditiveReachedThrough+": x:dep" {
			t.Errorf("class %d: record %+v, want FALLBACK %s: x:dep", class, plan.GoAdditive, GoAdditiveReachedThrough)
		}
		selected := sortedSelection(plan)
		for _, id := range []string{"x:core", "x:dep", "x:leaf", "x:ref", "x:unref", "x:user"} {
			if !slices.Contains(selected, id) {
				t.Errorf("class %d: %s not selected: %v", class, id, selected)
			}
		}
		if len(plan.GoAdditiveExcluded) != 0 {
			t.Errorf("class %d: the fallen-back record still excludes %+v", class, plan.GoAdditiveExcluded)
		}
	}
}
