// SPDX-License-Identifier: Apache-2.0
package affected_test

import (
	"testing"
)

// AFP-V0-3360, AFP-V0-3361: fixture-only dashboard readers have declared
// scopes. Unrelated record additions leave them out, while their fixtures,
// source and the producer they build still select them.
func TestUnboundedDashboardScopes_AFPV03360(t *testing.T) {
	graph, module := repositoryNarrowingGraph(t)
	for _, dirty := range []string{
		"docs/build-log/2026-11-09-probe.md",                                 //corvint:fixture-path
		".corvint/changes/5180518051805180518051805180518051805180.cem.json", //corvint:fixture-path
	} {
		selected := selectedUnits(graph, module, dirty)
		for _, directory := range []string{"internal/dashboard/repository"} {
			if witness, ok := selected[directory]; ok {
				t.Errorf("%s: bounded fixture reader %s selected as %s", dirty, directory, witness)
			}
		}
		// No blanket build-log exclusion: these inspect the tracked tree
		// and must remain selected. CEM sidecars already have a separate
		// repository-owned exclusion. (The offload runner's ENVFILE source
		// is host-only since AFP-V0-3340/3341, so its callers no longer
		// carry a dynamic-source closure into the tree.)
		if dirty == "docs/build-log/2026-11-09-probe.md" { //corvint:fixture-path
			for _, id := range []string{"internal/doccorpus/selfcorpus", "tools/test-read-scopes-check"} {
				if _, ok := selected[id]; !ok {
					t.Errorf("%s: unresolved/whole-tree reader %s lost", dirty, id)
				}
			}
		}
	}
	for _, tc := range []struct{ dirty, reader string }{
		{"internal/dashboard/repository/testdata/process_helper/main.go", "internal/dashboard/repository"},
		{"internal/dashboard/repository/filesystem.go", "internal/dashboard/repository"},
	} {
		if _, ok := selectedUnits(graph, module, tc.dirty)[tc.reader]; !ok {
			t.Errorf("%s: true reader %s lost", tc.dirty, tc.reader)
		}
	}
}

