//go:build darwin || linux

// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/sessiontest"
)

// TSP-V0-006 (amended 2026-11-11, V1-5162): source slicing makes at most 16 plans of the other
// dirty paths, counting the plan of every candidate it rejects as also selected, not only the
// candidates it keeps. Twenty packages that each change a plain function and import a changed
// package a are each selected by the others' plan, so before the amendment every one of them
// planned (selection-baseline's pr-41 replay planned 102 rejected candidates over 567 dirty
// paths). Every package stays whole either way; only the plan count and the reason change.
func TestTSPV0006SourceSlicingPlansAtMostSixteen(t *testing.T) {
	const count = 20
	base := map[string]string{}
	edits := map[string]string{"a/a.go": "package a\n\nfunc A() int { return 2 }\n"}
	for i := range count {
		name := fmt.Sprintf("p%d", i)
		base[name+"/"+name+".go"] = "package " + name + "\n\nimport \"example.test/m/a\"\n\nfunc P() int { return a.A() }\n"
		edits[name+"/"+name+".go"] = "package " + name + "\n\nimport \"example.test/m/a\"\n\nfunc P() int { return a.A() + 1 }\n"
	}
	root, revision := bulkSliceFixture(t, count, base, edits)
	_, _, sliced, walks, _ := bulkSlices(t, root, revision)
	if walks > testSourceSliceLimit {
		t.Fatalf("source slicing made %d plans of the other dirty paths, want at most %d", walks, testSourceSliceLimit)
	}
	codes := map[string]int{}
	for i := range count {
		name := fmt.Sprintf("p%d", i)
		got, ok := sliced["example.test/m/"+name]
		if !ok || got.Pattern != "" {
			t.Fatalf("%s sliced %+v, want whole", name, got)
		}
		code := sessiontest.ReasonCode(got.Reason)
		if !strings.Contains(got.Reason, "(also selected by ") && !strings.Contains(got.Reason, "(source slicing plans at most 16 packages)") {
			t.Fatalf("%s whole for %q, want also selected or the plan bound", name, got.Reason)
		}
		codes[code]++
	}
	// Package a, whose change the p packages share, is the first candidate: its plan (of the p
	// files) does not select it, so it counts one of the sixteen plans and the p packages get
	// the other fifteen.
	if bound := count + 1 - testSourceSliceLimit; codes[sessiontest.ReasonAlsoSelected] != count-bound || codes[sessiontest.ReasonSourceSliceBound] != bound {
		t.Fatalf("reason codes %v, want %d also selected and %d at the plan bound", codes, count-bound, bound)
	}
}
