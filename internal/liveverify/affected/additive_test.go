package affected

import (
	"slices"
	"testing"
)

// additiveFake is a hub with one importer that references its new
// declaration, one that does not (with its own importer), a test-only user,
// and a dependency the hub itself imports.
func additiveFake() fake {
	return fake{name: "x", units: []Unit{
		{ID: "x:base", Sources: []string{"base/base.x"}, Tests: []string{"base/base_test.x"}},
		{ID: "x:core", Sources: []string{"core/core.x", "core/fresh.x"}, Tests: []string{"core/core_test.x"}, Imports: []string{"x:base"}},
		{ID: "x:ref", Sources: []string{"ref/ref.x"}, Tests: []string{"ref/ref_test.x"}, Imports: []string{"x:core"}},
		{ID: "x:unref", Sources: []string{"unref/unref.x"}, Tests: []string{"unref/unref_test.x"}, Imports: []string{"x:core"}},
		{ID: "x:leaf", Sources: []string{"leaf/leaf.x"}, Tests: []string{"leaf/leaf_test.x"}, Imports: []string{"x:unref"}},
		{ID: "x:user", Tests: []string{"user/user_test.x"}, TestImports: []string{"x:core"}},
	}}
}

func sortedSelection(plan Plan) []string {
	ids := selectedIDs(plan)
	slices.Sort(ids)
	return ids
}

func narrowedCore() GoAdditive {
	return GoAdditive{Unit: "x:core", State: GoAdditiveNarrowed, Paths: []string{"core/fresh.x"}, Declarations: []string{"Fresh"}, Referencing: []string{"x:ref"}, Unreferenced: []string{"x:unref", "x:user"}}
}

// TestADNV0003AdditiveRecordPrunesUnreferencedImporters replays ADN-V0-002,
// ADN-V0-003 and ADN-V0-005: a NARROWED record keeps the package's own tests
// and its referencing importer, drops the unreferenced importer, what only it
// reaches, and the unreferenced test user, each listed in
// plan.goAdditiveExcluded through the package and absent from plan.excluded,
// whose closed reason enumeration it does not extend (CCF-V1-006), and the
// plan reports the record.
func TestADNV0003AdditiveRecordPrunesUnreferencedImporters(t *testing.T) {
	t.Parallel()
	graph := mustBuild(t, additiveFake())
	dirty := []string{"core/fresh.x"}
	if got := sortedSelection(Select(graph, dirty)); !slices.Equal(got, []string{"x:core", "x:leaf", "x:ref", "x:unref", "x:user"}) {
		t.Fatalf("plain selection %v", got)
	}
	plan := Select(graph.WithGoAdditive([]GoAdditive{narrowedCore()}), dirty)
	if got := sortedSelection(plan); !slices.Equal(got, []string{"x:core", "x:ref"}) {
		t.Fatalf("narrowed selection %v", got)
	}
	excluded := map[string]GoAdditiveExclusion{}
	for _, exclusion := range plan.GoAdditiveExcluded {
		excluded[exclusion.UnitID] = exclusion
	}
	if len(excluded) != 3 {
		t.Fatalf("additive exclusions %+v, want leaf, unref and user", plan.GoAdditiveExcluded)
	}
	for _, id := range []string{"x:leaf", "x:unref", "x:user"} {
		exclusion := excluded[id]
		if exclusion.Invalidation != GoAdditiveInvalidation || exclusion.Through != "x:core" || exclusion.Universe != graph.Digest() {
			t.Fatalf("%s exclusion %+v", id, exclusion)
		}
	}
	for _, exclusion := range plan.Excluded {
		if exclusion.Reason != ExcludedNoDependencyPath || exclusion.Invalidation != "NEW_DEPENDENCY_EDGE_OR_DIRTY_PATH" {
			t.Fatalf("plan.excluded carries a value outside its closed enumeration: %+v", exclusion)
		}
		if _, additive := excluded[exclusion.UnitID]; additive {
			t.Fatalf("%s is in both plan.excluded and plan.goAdditiveExcluded", exclusion.UnitID)
		}
	}
	if len(plan.GoAdditive) != 1 || plan.GoAdditive[0].State != GoAdditiveNarrowed || !slices.Equal(plan.GoAdditive[0].Unreferenced, []string{"x:unref", "x:user"}) {
		t.Fatalf("plan records %+v", plan.GoAdditive)
	}
	if graph.Digest() != graph.WithGoAdditive([]GoAdditive{narrowedCore()}).Digest() {
		t.Fatal("additive records changed the graph digest")
	}
}

// TestADNV0004GraphRefusesARecordItCannotCheck replays ADN-V0-004: a record
// whose dirty paths differ from the plan's, whose package is not a direct
// source change, or whose package the walk reaches from another changed unit
// falls back with its reason and the full fan-out returns.
func TestADNV0004GraphRefusesARecordItCannotCheck(t *testing.T) {
	t.Parallel()
	graph := mustBuild(t, additiveFake())
	cases := []struct {
		name   string
		dirty  []string
		reason string
	}{
		{"an extra dirty path in the package", []string{"core/core.x", "core/fresh.x"}, GoAdditiveDirtyMismatch},
		{"a dirty path the record was not computed for", []string{"core/core_test.x"}, GoAdditiveNotDirectChange},
		{"a changed dependency reaches the package", []string{"base/base.x", "core/fresh.x"}, GoAdditiveReachedThrough + ": x:base"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			plan := Select(graph.WithGoAdditive([]GoAdditive{narrowedCore()}), tc.dirty)
			want := sortedSelection(Select(graph, tc.dirty))
			if got := sortedSelection(plan); !slices.Equal(got, want) || !slices.Contains(got, "x:unref") {
				t.Fatalf("selection %v, want the unnarrowed %v", got, want)
			}
			if len(plan.GoAdditive) != 1 || plan.GoAdditive[0].State != GoAdditiveFallback || plan.GoAdditive[0].Reason != tc.reason || plan.GoAdditive[0].Unreferenced != nil {
				t.Fatalf("record %+v, want FALLBACK %s", plan.GoAdditive, tc.reason)
			}
			if len(plan.GoAdditiveExcluded) != 0 {
				t.Fatalf("fallback plan excludes %+v as additive", plan.GoAdditiveExcluded)
			}
		})
	}
}
