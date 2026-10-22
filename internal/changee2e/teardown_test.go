package changee2e

import (
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/jstestprovider"
)

// CCE-V0-048 (V1-2410): a run-level reporter error is attributed by its PWP-V3-009 location like a
// test failure: inside the staged test tree it is the test's own teardown failure; unlocated, or
// located outside the tree or in the linked node_modules, it stays infrastructure.
func TestCCEV0048TeardownAttribution(t *testing.T) {
	tree := t.TempDir()
	roots := treeRoots(tree)
	global := func(loc *jstestprovider.Anchor) jstestprovider.Receipt {
		return jstestprovider.Receipt{Tests: []jstestprovider.TestOutcome{{State: jstestprovider.StatePassed}},
			Infrastructure: &jstestprovider.InfrastructureFailure{Reason: jstestprovider.ReasonReporterGlobalError, Detail: "Error: boom", Location: loc}}
	}
	for _, tc := range []struct {
		name string
		r    jstestprovider.Receipt
		want string
	}{
		{"scenario file", global(&jstestprovider.Anchor{File: filepath.Join(tree, "e2e", "a.spec.ts"), Line: 4}), "teardown-failed at e2e/a.spec.ts:4"},
		{"fixture file", global(&jstestprovider.Anchor{File: filepath.Join(tree, "e2e", "fixtures.ts"), Line: 9}), "teardown-failed at e2e/fixtures.ts:9"},
		{"unlocated", global(nil), "infrastructure:reporter-global-error"},
		{"outside the tree", global(&jstestprovider.Anchor{File: "/elsewhere/fixtures.ts", Line: 9}), "infrastructure:reporter-global-error"},
		{"node_modules", global(&jstestprovider.Anchor{File: filepath.Join(tree, "node_modules", "x.js"), Line: 9}), "infrastructure:reporter-global-error"},
		{"other infrastructure keeps its reason", jstestprovider.Receipt{Infrastructure: &jstestprovider.InfrastructureFailure{Reason: "server-not-ready",
			Location: &jstestprovider.Anchor{File: filepath.Join(tree, "a.spec.ts"), Line: 1}}}, "infrastructure:server-not-ready"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, got := qualify(tc.r, nil, 1, roots); got != tc.want {
				t.Fatalf("qualify reason %q, want %q", got, tc.want)
			}
		})
	}
}
