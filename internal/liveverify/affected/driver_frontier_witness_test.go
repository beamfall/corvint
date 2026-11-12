// SPDX-License-Identifier: Apache-2.0

package affected

import (
	"slices"
	"testing"
	"testing/fstest"
)

// V1-5033 criterion 3: a FRONTIER_SITE witness that survives the literal
// file-set readers (AFP-V0-9401, AFP-V0-9402) still names the edge it was
// selected through, and the site it names carries its frontier reason. A
// literal enumerator must not be reported as a frontier site at all.
//
// AFP-V0-3600 (lane compsite, V1-5179) narrows AFP-V0-9402's clause that
// computed fallbacks stay intact: an evaluated target whose every
// alternative is literal and reads the whole tree, such as
// `$(if $(CI),git ls-files,find .)`, is a file-set reader, not a frontier
// site. The computed site here therefore uses a fallback whose command word
// is an overridable make value, and the AFP-V0-3600 shape is pinned beside
// it as a PATH_GLOB_READER.
func TestDriverFrontierSiteWitnessNamesEdgeOrReason_AFPV09402(t *testing.T) {
	files := fstest.MapFS{
		"go.mod": {Data: []byte("module example.test/m\n\ngo 1.27.0\n")},
		"Makefile": {Data: []byte("computed:\n\t$(if $(CI),git ls-files,$(LISTER) .)\n" +
			"evaluated:\n\t$(if $(CI),git ls-files,find .)\n" +
			"runner:\n\t$$RUNNER check\n" +
			"enumerate:\n\tgit ls-files -z\n" +
			"all: computed enumerate\n" +
			"plain:\n\techo done\n")},
		".github/workflows/ci.yml": {Data: []byte("jobs:\n" +
			"  caller:\n    steps:\n      - run: make all\n" +
			"  computed:\n    steps:\n      - run: $RUNNER check\n" +
			"  plain:\n    steps:\n      - run: echo done\n")},
		"owned.g":      {Data: []byte("source")},
		"owned_test.g": {Data: []byte("test")},
	}
	owner := fake{name: "g", units: []Unit{{ID: "g:owned", Sources: []string{"owned.g"}, Tests: []string{"owned_test.g"}}}}
	graph, err := BuildFS(files, driverFixtureLanguage{owner}, Driver{})
	if err != nil {
		t.Fatal(err)
	}
	wantSites := []string{"driver:make:Makefile#computed", "driver:make:Makefile#runner", "driver:ci:.github/workflows/ci.yml#computed"}
	for _, dirty := range []string{"new.fixture", "owned.g", "owned_test.g", "unindexed/new.go", ".corvint/changes/new.cem.json"} {
		t.Run(dirty, func(t *testing.T) {
			plan := Select(graph, []string{dirty})
			selected := witnesses(plan)
			sites := 0
			for _, selection := range plan.Selected {
				witness := selection.Witness
				if witness.Kind != WitnessFrontierSite {
					continue
				}
				sites++
				// A site is selected through itself: one edge, the site unit.
				if len(witness.Via) != 1 || witness.Via[0] != selection.UnitID {
					t.Errorf("%s: FRONTIER_SITE witness does not name the site as its one edge: %+v", selection.UnitID, witness)
					continue
				}
				if witness.DirtyPath != dirty {
					t.Errorf("%s: FRONTIER_SITE witness names dirty path %q, want %q", selection.UnitID, witness.DirtyPath, dirty)
				}
				site, found := graph.Unit(witness.Via[0])
				if !found {
					t.Errorf("%s: witness edge starts at an unknown unit %q", selection.UnitID, witness.Via[0])
					continue
				}
				if !slices.Contains(site.Frontier, FrontierDriverComputedInvocation) || slices.Contains(site.Frontier, "") {
					t.Errorf("%s: frontier site %s does not carry the computed-invocation reason: %+v", selection.UnitID, site.ID, site.Frontier)
				}
			}
			if sites != len(wantSites) {
				t.Errorf("FRONTIER_SITE count = %d, want %d: %+v", sites, len(wantSites), selected)
			}
			for _, id := range wantSites {
				if selected[id].Kind != WitnessFrontierSite {
					t.Errorf("computed site %s lost its FRONTIER_SITE witness: %+v", id, selected[id])
				}
			}
			// The literal enumerator and the caller reached only through
			// the sites keep a real edge instead of a frontier claim.
			if got := selected["driver:make:Makefile#enumerate"]; got.Kind != WitnessPathGlobReader || got.Glob == "" || got.DirtyPath != dirty {
				t.Errorf("literal enumerator: witness %+v, want PATH_GLOB_READER naming its glob and %s", got, dirty)
			}
			// AFP-V0-3600: the evaluated whole-tree reader keeps its
			// selection on every dirty path through its /* edge, and only
			// its witness becomes that file-set read.
			const evaluated = "driver:make:Makefile#evaluated"
			if got := selected[evaluated]; got.Kind != WitnessPathGlobReader || got.Glob != "/*" || got.DirtyPath != dirty ||
				len(got.Via) != 1 || got.Via[0] != evaluated {
				t.Errorf("AFP-V0-3600 evaluated reader: witness %+v, want PATH_GLOB_READER through itself naming /* and %s", got, dirty)
			}
			if unit, _ := graph.Unit(evaluated); slices.Contains(unit.Frontier, FrontierDriverComputedInvocation) {
				t.Errorf("AFP-V0-3600 evaluated reader kept the computed-invocation hedge: %+v", unit.Frontier)
			}
			for _, id := range []string{"driver:make:Makefile#all", "driver:ci:.github/workflows/ci.yml#caller"} {
				if got := selected[id]; got.Kind != WitnessDependency || len(got.Via) < 2 {
					t.Errorf("%s: witness %+v, want DEPENDENCY_PATH with its edge", id, got)
				}
			}
			for _, id := range []string{"driver:make:Makefile#plain", "driver:ci:.github/workflows/ci.yml#plain"} {
				if _, found := selected[id]; found {
					t.Errorf("unrelated driver %s selected: %+v", id, selected[id])
				}
			}
		})
	}
}
