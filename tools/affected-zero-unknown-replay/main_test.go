// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func replayFixture() (safetyReceipt, plan, plan, map[string]map[int]bool) {
	old := plan{Selected: []selection{{UnitID: "go:a", Tests: []string{"a_test.go"}}}, Excluded: []selection{{UnitID: "go:b"}}, Unknown: []unknown{}}
	now := old
	r := safetyReceipt{Merge: strings.Repeat("a", 40), Parent: strings.Repeat("b", 40), MutationComplete: true,
		ParentSuite: suite{Revision: strings.Repeat("b", 40), Complete: true, Inventory: []string{"go:a", "go:b"}, Results: []observation{{"go:a", "TestA", "pass"}, {"go:b", "TestB", "pass"}}},
		MergeSuite:  suite{Revision: strings.Repeat("a", 40), Complete: true, Inventory: []string{"go:a", "go:b"}, Results: []observation{{"go:a", "TestA", "pass"}, {"go:b", "TestB", "pass"}}},
		Mutants:     []mutant{{ID: "mutant-1", Side: "merge", Path: "a.go", Line: 2, KilledBy: []observation{{Unit: "go:a", Test: "TestA"}}}},
	}
	return r, old, now, map[string]map[int]bool{"merge:a.go": {2: true}, "parent:a.go": {2: true}}
}

// AZU-V0-009, AZU-V0-8903: mutating selection must fail the same checker CI
// uses, both for changed full-suite outcomes and changed-line mutant killers.
func TestSafetyRejectsUnsafeDrop_AZU_V0_8903(t *testing.T) {
	for _, kind := range []string{"outcome", "mutant", "both"} {
		t.Run(kind, func(t *testing.T) {
			r, old, now, lines := replayFixture()
			if kind == "outcome" || kind == "both" {
				r.MergeSuite.Results[1].Outcome = "fail"
			}
			if kind == "mutant" || kind == "both" {
				r.Mutants[0].KilledBy = append(r.Mutants[0].KilledBy, observation{Unit: "go:b", Test: "TestB"})
			}
			body, _ := json.Marshal(r)
			got := safety(body, r.Merge, r.Parent, old, now, lines)
			if got.Status != "UNSAFE_DROP" || len(got.Misses) == 0 {
				t.Fatalf("unsafe selection admitted: %+v", got)
			}
			now.Selected = append(now.Selected, selection{UnitID: "go:b"})
			now.Excluded = []selection{}
			body, _ = json.Marshal(r)
			if fixed := safety(body, r.Merge, r.Parent, old, now, lines); fixed.Status != "OBSERVED_NO_MISS" {
				t.Fatalf("repair rejected: %+v", fixed)
			}
		})
	}
}

// AZU-V0-8903: missing, incomplete, wrong-revision, unbound mutation and
// unknown test evidence cannot turn an unobserved safety run into acceptance.
func TestSafetyRefusesIncompleteEvidence_AZU_V0_8903(t *testing.T) {
	for _, kind := range []string{"missing-suite", "wrong-revision", "missing-mutation", "unchanged-line", "unknown-killer", "duplicate-result", "missing-result", "wrong-inventory"} {
		t.Run(kind, func(t *testing.T) {
			r, old, now, lines := replayFixture()
			switch kind {
			case "missing-suite":
				r.MergeSuite.Complete = false
			case "wrong-revision":
				r.Parent = "other"
			case "missing-mutation":
				r.MutationComplete = false
			case "unchanged-line":
				r.Mutants[0].Line = 3
			case "unknown-killer":
				r.Mutants[0].KilledBy[0].Unit = "go:ghost"
			case "duplicate-result":
				r.MergeSuite.Results = append(r.MergeSuite.Results, r.MergeSuite.Results[0])
			case "missing-result":
				r.MergeSuite.Results = r.MergeSuite.Results[:1]
			case "wrong-inventory":
				r.MergeSuite.Inventory = []string{"go:a"}
			}
			body, _ := json.Marshal(r)
			got := safety(body, strings.Repeat("a", 40), strings.Repeat("b", 40), old, now, lines)
			if got.Status != "NOT_PRODUCED" || got.Detail == "" {
				t.Fatalf("incomplete evidence admitted: %+v", got)
			}
		})
	}
}

func TestChangedLineCoordinates_AZU_V0_8903(t *testing.T) {
	diff := "diff --git a/a.go b/a.go\n--- a/a.go\n+++ b/a.go\n@@ -1,0 +2,2 @@\n+x\n+y\n@@ -8 +10 @@\n-z\n+q\ndiff --git a/b.go b/b.go\n--- a/b.go\n+++ b/b.go\n@@ -1 +1,0 @@\n-x\n"
	lines := changedLines(diff)
	if len(lines["merge:a.go"]) != 3 || !lines["merge:a.go"][2] || !lines["merge:a.go"][3] || !lines["merge:a.go"][10] || len(lines["merge:b.go"]) != 0 {
		t.Fatalf("wrong changed lines: %+v", lines)
	}
}

func TestBeforeAfterNamesEveryRemovedUnknown_AZU_V0_8903(t *testing.T) {
	old := plan{Selected: []selection{{UnitID: "kept"}, {UnitID: "dropped"}}, Unknown: []unknown{{"LANGUAGE_FRONTIER", "shell:foreign"}}}
	now := plan{Selected: []selection{{UnitID: "kept"}, {UnitID: "added"}}, Unknown: []unknown{}}
	var r row
	compare(&r, old, now)
	if len(r.RemovedUnknown) != 1 || strings.Join(r.AddedSelections, ",") != "added" || strings.Join(r.DroppedSelections, ",") != "dropped" {
		t.Fatalf("incomplete delta: %+v", r)
	}
}

// AZU-V0-8903: the frozen acceptance set cannot be shortened or repointed.
func TestFrozenMergePrefix_AZU_V0_8903(t *testing.T) {
	if !validFrozenPrefix(frozenMergePrefix) {
		t.Fatal("contractual prefix rejected")
	}
	if validFrozenPrefix(frozenMergePrefix[:7]) {
		t.Fatal("missing frozen merge accepted")
	}
	altered := append([]string(nil), frozenMergePrefix...)
	altered[1] = strings.Repeat("0", 40)
	if validFrozenPrefix(altered) {
		t.Fatal("repointed frozen merge accepted")
	}
}
