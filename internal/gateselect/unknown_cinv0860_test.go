// SPDX-License-Identifier: Apache-2.0
package gateselect

import (
	"strings"
	"testing"
)

// TestUnknownPlanNarrowsOnlyThroughLanguageFrontiers_CINV0860: Select, the CI
// PR tier, runs an UNKNOWN plan narrowed only when every unknown is a
// LANGUAGE_FRONTIER that is not module level; any other unknown, alone or
// beside frontiers, a malformed reason and an UNKNOWN plan naming no unknown
// fall back. SelectStructural (`corvint test`) is unchanged.
func TestUnknownPlanNarrowsOnlyThroughLanguageFrontiers_CINV0860(t *testing.T) {
	root := writeFixture(t, map[string]string{
		"go.mod":            "module " + fixtureModule + "\n\ngo 1.22\n",
		"core/core.go":      "package core\n",
		"core/core_test.go": "package core\n",
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
		receipt.Plan.Dirty = []string{"core/core.go", "Makefile"}
		receipt.Plan.Unknown = unknowns
		return receipt
	}
	frontier := unknown{"LANGUAGE_FRONTIER", "typescript:dynamic-import:a.ts:1 `x`"}
	for _, tc := range []struct {
		name    string
		plan    Receipt
		verdict string
	}{
		{"bounded", plan("BOUNDED"), "run " + fixtureModule + "/core"},
		{"language frontiers alone", plan("UNKNOWN", frontier, unknown{"LANGUAGE_FRONTIER", "shell:foreign-invocation-unmodelled"}), "run " + fixtureModule + "/core"},
		{"a module-level frontier", plan("UNKNOWN", unknown{"LANGUAGE_FRONTIER", "go:cgo-frontier"}), "FALLBACK Go frontier is unknown at module level: go:cgo-frontier"},
		{"an unowned dirty path", plan("UNKNOWN", unknown{"UNOWNED_DIRTY_PATH", "Makefile"}), "FALLBACK the plan names the unknown UNOWNED_DIRTY_PATH, not a language frontier"},
		{"an unowned path beside frontiers", plan("UNKNOWN", frontier, unknown{"UNOWNED_DIRTY_PATH", "Makefile"}), "FALLBACK the plan names the unknown UNOWNED_DIRTY_PATH, not a language frontier"},
		{"an unindexed source path", plan("UNKNOWN", unknown{"UNINDEXED_SOURCE_PATH", "x.go"}), "FALLBACK the plan names the unknown UNINDEXED_SOURCE_PATH, not a language frontier"},
		{"no selectable test", plan("UNKNOWN", unknown{"NO_SELECTABLE_TEST", "go:x"}), "FALLBACK the plan names the unknown NO_SELECTABLE_TEST, not a language frontier"},
		{"a malformed reason", plan("UNKNOWN", unknown{"run everything", "x"}), "FALLBACK the plan names the unknown a malformed reason, not a language frontier"},
		{"an empty reason", plan("UNKNOWN", unknown{"", "x"}), "FALLBACK the plan names the unknown a malformed reason, not a language frontier"},
		{"no unknown named", plan("UNKNOWN"), "FALLBACK the plan is UNKNOWN but names no unknown"},
		// The rule reads the unknowns, not the scope: a BOUNDED receipt that
		// names another unknown is not trusted either.
		{"a bounded scope naming one", plan("BOUNDED", unknown{"UNOWNED_DIRTY_PATH", "Makefile"}), "FALLBACK the plan names the unknown UNOWNED_DIRTY_PATH, not a language frontier"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := verdict(tc.plan, root); got != tc.verdict {
				t.Fatalf("verdict %q, want %q", got, tc.verdict)
			}
			structural := SelectStructural(tc.plan, fixtureModule, root)
			if last := structural[len(structural)-1]; strings.Contains(tc.verdict, ", not a language frontier") && !strings.HasPrefix(last, "run ") {
				t.Fatalf("SelectStructural %q, want it unchanged", last)
			}
		})
	}
}
