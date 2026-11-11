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
func TestDriverFrontierSiteWitnessNamesEdgeOrReason_AFPV09402(t *testing.T) {
	files := fstest.MapFS{
		"go.mod": {Data: []byte("module example.test/m\n\ngo 1.27.0\n")},
		"Makefile": {Data: []byte("computed:\n\t$(if $(CI),git ls-files,find .)\n" +
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
				if len(witness.Via) == 0 || witness.Via[len(witness.Via)-1] != selection.UnitID {
					t.Errorf("%s: FRONTIER_SITE witness names no edge ending at the unit: %+v", selection.UnitID, witness)
					continue
				}
				if witness.DirtyPath == "" {
					t.Errorf("%s: FRONTIER_SITE witness names no dirty path: %+v", selection.UnitID, witness)
				}
				site, found := graph.Unit(witness.Via[0])
				if !found {
					t.Errorf("%s: witness edge starts at an unknown unit %q", selection.UnitID, witness.Via[0])
					continue
				}
				if len(site.Frontier) == 0 || slices.Contains(site.Frontier, "") {
					t.Errorf("%s: frontier site %s carries no reason: %+v", selection.UnitID, site.ID, site)
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
			if got := selected["driver:make:Makefile#enumerate"]; got.Kind == WitnessFrontierSite || got.Kind == "" {
				t.Errorf("literal enumerator: witness %+v, want a file-set or dependency edge", got)
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
