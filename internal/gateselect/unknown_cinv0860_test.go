// SPDX-License-Identifier: Apache-2.0
package gateselect

import (
	"strings"
	"testing"
)

// TestUnknownPlanNarrowsOnlyThroughLanguageFrontiers_CINV0860: Select, the CI
// PR tier, runs an UNKNOWN plan narrowed only when every unknown is a
// non-module-level LANGUAGE_FRONTIER, an UNOWNED_DIRTY_PATH, a
// NO_SELECTABLE_TEST or a deleted Go source of a package that still exists
// (attributed to it and its importer by rule (a)); an unindexed existing
// source, a deleted source of a removed package, another reason, a malformed
// reason and an UNKNOWN plan naming none fall back. The UNOWNED_DIRTY_PATH and
// NO_SELECTABLE_TEST cases were V1-3354 negatives, inverted by V1-3480.
// SelectStructural (`corvint test`) is unchanged.
func TestUnknownPlanNarrowsOnlyThroughLanguageFrontiers_CINV0860(t *testing.T) {
	root := writeFixture(t, map[string]string{
		"go.mod":            "module " + fixtureModule + "\n\ngo 1.22\n",
		"core/core.go":      "package core\n",
		"core/core_test.go": "package core\n",
		"user/user.go":      "package user\n\nimport _ \"" + fixtureModule + "/core\"\n",
		"user/user_test.go": "package user\n",
	})
	type unknown = struct {
		Reason string `json:"reason"`
		Detail string `json:"detail"`
	}
	plan := func(scope string, unknowns ...unknown) Receipt {
		var receipt Receipt
		receipt.Provider.Go.State = "RUNNABLE"
		receipt.Provider.Go.Packages = []string{fixtureModule + "/core"}
		receipt.Plan.Scope = scope
		receipt.Plan.Dirty = []string{"core/core.go", "Makefile", "core/gone.go", "removed/gone.go"}
		receipt.Plan.Unknown = unknowns
		return receipt
	}
	core, user := fixtureModule+"/core", fixtureModule+"/user"
	frontier := unknown{"LANGUAGE_FRONTIER", "typescript:dynamic-import:a.ts:1 `x`"}
	cannot := func(reason string) string {
		return "FALLBACK the plan names the unknown " + reason + ", which the selector cannot attribute"
	}
	for _, tc := range []struct {
		name    string
		plan    Receipt
		verdict string
	}{
		{"bounded", plan("BOUNDED"), "run " + core + " " + user},
		{"language frontiers alone", plan("UNKNOWN", frontier, unknown{"LANGUAGE_FRONTIER", "shell:foreign-invocation-unmodelled"}), "run " + core + " " + user},
		{"an unowned dirty path", plan("UNKNOWN", unknown{"UNOWNED_DIRTY_PATH", "Makefile"}), "run " + core + " " + user},
		{"an unowned path beside frontiers", plan("UNKNOWN", frontier, unknown{"UNOWNED_DIRTY_PATH", "Makefile"}), "run " + core + " " + user},
		{"no selectable test", plan("UNKNOWN", unknown{"NO_SELECTABLE_TEST", "go:x"}), "run " + core + " " + user},
		{"a deleted source of a package that exists", plan("UNKNOWN", unknown{"UNINDEXED_SOURCE_PATH", "core/gone.go"}), "run " + core + " " + user},
		{"a module-level frontier", plan("UNKNOWN", unknown{"LANGUAGE_FRONTIER", "go:cgo-frontier"}), "FALLBACK Go frontier is unknown at module level: go:cgo-frontier"},
		{"an unindexed source that exists", plan("UNKNOWN", unknown{"UNINDEXED_SOURCE_PATH", "core/core.go"}), cannot("UNINDEXED_SOURCE_PATH")},
		{"a deleted source of a removed package", plan("UNKNOWN", unknown{"UNINDEXED_SOURCE_PATH", "removed/gone.go"}), cannot("UNINDEXED_SOURCE_PATH")},
		{"an unindexed path that is not dirty", plan("UNKNOWN", unknown{"UNINDEXED_SOURCE_PATH", "core/other.go"}), cannot("UNINDEXED_SOURCE_PATH")},
		{"another reason", plan("UNKNOWN", unknown{"PLAN_TRUNCATED", "x"}), cannot("PLAN_TRUNCATED")},
		{"a malformed reason", plan("UNKNOWN", unknown{"run everything", "x"}), cannot("a malformed reason")},
		{"an empty reason", plan("UNKNOWN", unknown{"", "x"}), cannot("a malformed reason")},
		{"no unknown named", plan("UNKNOWN"), "FALLBACK the plan is UNKNOWN but names no unknown"},
		// The rule reads the unknowns, not the scope.
		{"a bounded scope naming one", plan("BOUNDED", unknown{"UNINDEXED_SOURCE_PATH", "core/core.go"}), cannot("UNINDEXED_SOURCE_PATH")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := verdict(tc.plan, root); got != tc.verdict {
				t.Fatalf("verdict %q, want %q", got, tc.verdict)
			}
			structural := SelectStructural(tc.plan, fixtureModule, root)
			if last := structural[len(structural)-1]; strings.Contains(tc.verdict, "cannot attribute") && !strings.HasPrefix(last, "run ") {
				t.Fatalf("SelectStructural %q, want it unchanged", last)
			}
		})
	}
}
