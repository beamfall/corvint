// SPDX-License-Identifier: Apache-2.0
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSelectionBaselineGateFailsOnFloorOrMiss_V12892 covers the V1-2892 gate:
// the CI rule, the miss check against recorded failures, and the floor.
func TestSelectionBaselineGateFailsOnFloorOrMiss_V12892(t *testing.T) {
	costs := map[string]float64{"m/a": 60, "m/b": 30, "m/c": 10}
	for _, tc := range []struct {
		verdict string
		github  bool
		want    string
	}{
		{"run", false, "NARROW"},
		{"NOTHING", false, "NOTHING"},
		{"FALLBACK", false, "FULL"},
		{"run", true, "FULL"},
		{"", false, "FULL"},
	} {
		if got := CIScope(tc.verdict, tc.github); got != tc.want {
			t.Errorf("CIScope(%q, %v) = %s, want %s", tc.verdict, tc.github, got, tc.want)
		}
	}

	narrowed := Result{Kind: "failed-ci", Failures: []failure{{Package: "m/a"}, {Package: "m/c"}, {Unattributed: "no package"}}}
	fillVerdict(&narrowed, "selected m/a\nrun m/b m/a\n", costs)
	checkFailures(&narrowed)
	if narrowed.CIScope != "NARROW" || narrowed.CostShare != 0.9 || narrowed.PackageShare != 0.667 {
		t.Fatalf("narrowed = %+v", narrowed)
	}
	if len(narrowed.Misses) != 1 || narrowed.Misses[0] != "m/c" || narrowed.Unattributed != 1 {
		t.Fatalf("misses = %v unattributed = %d, want [m/c] and 1", narrowed.Misses, narrowed.Unattributed)
	}

	full := Result{Kind: "failed-ci", Failures: []failure{{Package: "m/c"}}}
	fillVerdict(&full, "FALLBACK provider.go.state is UNAVAILABLE\n", costs)
	checkFailures(&full)
	if full.CIScope != "FULL" || full.CostShare != 1 || len(full.Misses) != 0 || full.VerdictReason == "" {
		t.Fatalf("a FULL run misses nothing: %+v", full)
	}

	nothing := Result{Kind: "failed-ci", Failures: []failure{{Package: "m/a"}}}
	fillVerdict(&nothing, "NOTHING no package\n", costs)
	checkFailures(&nothing)
	if len(nothing.Misses) != 1 || !strings.Contains(nothing.MissCauses[0], "selected nothing") {
		t.Fatalf("NOTHING with a failure is a miss: %+v", nothing)
	}

	merged := []Result{{Kind: "merged", CIScope: "NARROW", CostShare: 0.2}, {Kind: "merged", CIScope: "FULL", CostShare: 1}, {Kind: "merged", CIScope: "NOTHING", PlanScope: "BOUNDED"}}
	s := Summarize(append(merged, narrowed, full))
	if s.Merged != 3 || s.NarrowMerged != 2 || s.BoundedMerged != 1 || s.Misses != 1 || s.FailedCI != 2 || s.FailingPackages != 2 || s.MedianCostShareMerged != 0.2 {
		t.Fatalf("summary = %+v", s)
	}
	if got := Failures(s, floor{NarrowMerged: 2}); len(got) != 1 || !strings.Contains(got[0], "missed") {
		t.Fatalf("a miss fails the gate: %v", got)
	}
	s.Misses = 0
	if got := Failures(s, floor{NarrowMerged: 3}); len(got) != 1 || !strings.Contains(got[0], "below the floor") {
		t.Fatalf("a narrow count below the floor fails the gate: %v", got)
	}
	if got := Failures(s, floor{NarrowMerged: 2}); len(got) != 0 {
		t.Fatalf("at the floor with no miss the gate passes: %v", got)
	}
}

func TestSelectionBaselineUnknownKindsAndManifest_V12892(t *testing.T) {
	if got := UnknownKind("LANGUAGE_FRONTIER", "typescript:dynamic-import:a.ts:3 `x`"); got != "LANGUAGE_FRONTIER typescript:dynamic-import" {
		t.Fatalf("frontier kind = %q", got)
	}
	if got := UnknownKind("UNOWNED_DIRTY_PATH", "Makefile"); got != "UNOWNED_DIRTY_PATH" {
		t.Fatalf("other kind = %q", got)
	}
	var m manifest
	m.Units = append(m.Units,
		manifestUnit{"go", "go:a", "whole", "whole: DIRECT_SOURCE_CHANGE via a/a.go (its test file a/a_test.go changed too)"},
		manifestUnit{"go", "go:b", "whole", "whole: DEPENDENCY_PATH via a/a.go"},
		manifestUnit{"go", "go:c", "selected", ""},
		manifestUnit{"shell", "shell:d", "whole", ""})
	var r Result
	fillManifest(&r, m)
	if r.WholePackages != 2 || r.SlicedPackages != 1 || r.WholeWithTests != 1 {
		t.Fatalf("manifest counts = %+v", r)
	}
}

func TestSelectionBaselineChangeSetIsValid_V12892(t *testing.T) {
	set, err := readChanges(filepath.Join("..", "..", "benchmarks", "selection-baseline", "changes.json"))
	if err != nil {
		t.Fatal(err)
	}
	merged := 0
	for _, c := range set.Changes {
		if c.Kind == "merged" {
			merged++
		}
		if c.Kind == "failed-ci" && len(c.Failures) == 0 {
			t.Errorf("%s: a failed-ci change records no failure", c.ID)
		}
	}
	if merged < 19 || set.Floor.Misses != 0 {
		t.Fatalf("merged = %d floor = %+v", merged, set.Floor)
	}
	bad := filepath.Join(t.TempDir(), "changes.json")
	if err := os.WriteFile(bad, []byte(`{"profile":"`+changesProfile+`","changes":[],"extra":1}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readChanges(bad); err == nil {
		t.Fatal("an unknown member is refused")
	}
}
