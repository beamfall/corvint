// SPDX-License-Identifier: Apache-2.0
package main

import (
	"encoding/json"
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
		// The prose is reworded on purpose: the counts read only the code
		// (STR-V0-548).
		manifestUnit{"go", "go:a", "whole", "whole: reworded prose", "direct-source-non-function-edit"},
		manifestUnit{"go", "go:b", "whole", "whole: DIRECT_SOURCE_CHANGE via a/a.go changed too", "reached-via-dependency-path"},
		manifestUnit{"go", "go:c", "selected", "", "selected-reaching-change"},
		manifestUnit{"shell", "shell:d", "whole", "", ""})
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
		// TSP-V0-710: every change of the fixed set records the replay its floor was measured on.
		if r := c.Recorded; r == nil || (r.CIScope != "NARROW" && r.CIScope != "FULL" && r.CIScope != "NOTHING") ||
			(r.Verdict != "run" && r.Verdict != "FALLBACK" && r.Verdict != "NOTHING") {
			t.Errorf("%s: recorded = %+v, want a CI scope and a selector verdict", c.ID, c.Recorded)
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

// TSP-V0-710 (V1-3501): a replay names each change whose CI scope or verdict moved from its
// record, with the selector's reason, ahead of the floor verdict, and counts the changes that
// carry no record; an unmoved change prints nothing and the floor check is unchanged.
func TestSelectionBaselineNamesMovedVerdicts_V13501(t *testing.T) {
	changes := []change{
		{ID: "pr-13", Recorded: &recorded{CIScope: "NARROW", Verdict: "run"}},
		{ID: "pr-14", Recorded: &recorded{CIScope: "NARROW", Verdict: "run"}},
		{ID: "pr-15", Recorded: &recorded{CIScope: "FULL", Verdict: "FALLBACK"}},
		{ID: "pr-16"},
	}
	results := []Result{
		{ID: "pr-13", Kind: "merged", Verdict: "run", CIScope: "NARROW"},
		{ID: "pr-14", Kind: "merged", Verdict: "FALLBACK", VerdictReason: "Go frontier is unknown at module level: go:gated-root-locators-changed", CIScope: "FULL"},
		{ID: "pr-15", Kind: "merged", Verdict: "run", CIScope: "NARROW"},
		{ID: "pr-16", Kind: "merged", Verdict: "run", CIScope: "NARROW"},
	}
	want := []string{
		"selection-baseline: MOVED pr-14 ci NARROW -> FULL (verdict run -> FALLBACK: Go frontier is unknown at module level: go:gated-root-locators-changed)",
		"selection-baseline: MOVED pr-15 ci FULL -> NARROW (verdict FALLBACK -> run)",
		"selection-baseline: UNRECORDED 1 change(s) carry no recorded verdict",
	}
	if got := Moved(changes, results); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("moved lines:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if got := Moved(changes[:1], results[:1]); len(got) != 0 {
		t.Fatalf("an unmoved recorded change printed %v", got)
	}
	if got := Failures(Summarize(results), floor{NarrowMerged: 3}); len(got) != 0 {
		t.Fatalf("a move alone failed the floor: %v", got)
	}
}

// TestSelectionBaselinePinsTestAffectedAgreement_AFPV02403 (V1-4363): a NARROWED corvint test
// manifest that runs a Go package the plan does not select, or leaves a failing package unrun,
// breaks the gate; a FULL manifest is not compared, and a nested module unit is no plan unit.
func TestSelectionBaselinePinsTestAffectedAgreement_AFPV02403(t *testing.T) {
	var p plan
	p.Plan.Selected = append(p.Plan.Selected, struct {
		UnitID  string `json:"unitId"`
		Witness struct {
			Kind string `json:"kind"`
		} `json:"witness"`
	}{UnitID: "go:m/a"})
	agree := manifest{Scope: "NARROWED", Units: []manifestUnit{{Adapter: "go", Unit: "go:m/a"}, {Adapter: "go", Unit: "go-module:tools/x"}}}
	res := Result{Failures: []failure{{Package: "m/a"}}}
	checkAgreement(&res, p, agree)
	if len(res.TestExtra) != 0 || len(res.TestMisses) != 0 {
		t.Fatalf("agreeing manifest flagged: %+v", res)
	}
	extra := manifest{Scope: "NARROWED", Units: []manifestUnit{{Adapter: "go", Unit: "go:m/a"}, {Adapter: "go", Unit: "go:m/b"}}}
	res = Result{Failures: []failure{{Package: "m/c"}}}
	checkAgreement(&res, p, extra)
	if strings.Join(res.TestExtra, ",") != "m/b" || strings.Join(res.TestMisses, ",") != "m/c" {
		t.Fatalf("extra or miss not named: %+v", res)
	}
	if fails := Failures(Summarize([]Result{res}), floor{}); len(fails) != 2 {
		t.Fatalf("failures = %v, want the extra and the miss", fails)
	}
	full := Result{Failures: []failure{{Package: "m/c"}}}
	checkAgreement(&full, p, manifest{Scope: "FULL", Units: extra.Units})
	if len(full.TestExtra) != 0 || len(full.TestMisses) != 0 {
		t.Fatalf("a FULL manifest was compared: %+v", full)
	}
}

// TestAFPV02403NothingManifestMissesFailingPackages (review P2): a NOTHING manifest runs no
// package, so a failing package the plan and the CI selector both select is a test miss and
// breaks the gate; the CI selector's own miss check passing does not excuse it. A NOTHING
// manifest for a change with no failing package still omits the package the plan selects and
// breaks the gate (review round e P2); an unrecognized scope is also pinned.
func TestAFPV02403NothingManifestMissesFailingPackages(t *testing.T) {
	var p plan
	p.Plan.Selected = append(p.Plan.Selected, struct {
		UnitID  string `json:"unitId"`
		Witness struct {
			Kind string `json:"kind"`
		} `json:"witness"`
	}{UnitID: "go:m/a"})
	res := Result{CIScope: "NARROW", Packages: []string{"m/a"}, Failures: []failure{{Package: "m/a"}}}
	checkFailures(&res)
	checkAgreement(&res, p, manifest{Scope: "NOTHING"})
	if len(res.Misses) != 0 || len(res.TestExtra) != 0 || strings.Join(res.TestMisses, ",") != "m/a" {
		t.Fatalf("a NOTHING manifest hid the failing package: %+v", res)
	}
	if fails := Failures(Summarize([]Result{res}), floor{}); len(fails) != 2 {
		t.Fatalf("failures = %v, want the NOTHING manifest's test miss and its omitted package", fails)
	}
	// Units a NOTHING manifest should not carry are not read as run.
	res = Result{Failures: []failure{{Package: "m/a"}}}
	checkAgreement(&res, p, manifest{Scope: "NOTHING", Units: []manifestUnit{{Adapter: "go", Unit: "go:m/a"}}})
	if strings.Join(res.TestMisses, ",") != "m/a" || len(res.TestExtra) != 0 {
		t.Fatalf("a NOTHING manifest's units were read as run: %+v", res)
	}
	clean := Result{CIScope: "NARROW", Packages: []string{"m/a"}}
	checkAgreement(&clean, p, manifest{Scope: "NOTHING"})
	if len(clean.TestMisses) != 0 || len(clean.TestExtra) != 0 || testOmittedJSON(t, clean) != `["m/a"]` {
		t.Fatalf("a NOTHING manifest without failures did not omit the planned package: %+v", clean)
	}
	if fails := Failures(Summarize([]Result{clean}), floor{}); len(fails) != 1 {
		t.Fatalf("failures = %v, want the NOTHING manifest's omitted package", fails)
	}
	odd := Result{Failures: []failure{{Package: "m/a"}}}
	checkAgreement(&odd, p, manifest{Scope: "", Units: []manifestUnit{{Adapter: "go", Unit: "go:m/a"}}})
	if strings.Join(odd.TestMisses, ",") != "m/a" {
		t.Fatalf("an unrecognized manifest scope was not compared as an empty run set: %+v", odd)
	}
}

// TestAFPV02403ManifestOmitsPlannedPackage (review round e P2): the agreement is checked in both
// directions. A NARROWED manifest that does not run a root Go package the plan selects breaks the
// gate even when the change records no failing package; a nested module's own unit in the
// manifest and a plan unit of another adapter are not compared.
func TestAFPV02403ManifestOmitsPlannedPackage(t *testing.T) {
	var p plan
	for _, id := range []string{"go:m/b", "shell:x", "go:m/a", "go:m/b"} {
		p.Plan.Selected = append(p.Plan.Selected, struct {
			UnitID  string `json:"unitId"`
			Witness struct {
				Kind string `json:"kind"`
			} `json:"witness"`
		}{UnitID: id})
	}
	res := Result{CIScope: "NARROW", Packages: []string{"m/a", "m/b"}}
	checkAgreement(&res, p, manifest{Scope: "NARROWED", Units: []manifestUnit{{Adapter: "go", Unit: "go:m/a"}, {Adapter: "go", Unit: "go-module:tools/x"}}})
	if got := testOmittedJSON(t, res); got != `["m/b"]` || len(res.TestExtra) != 0 || len(res.TestMisses) != 0 {
		t.Fatalf("omitted = %s, want only m/b: %+v", got, res)
	}
	s := Summarize([]Result{res})
	fails := Failures(s, floor{})
	if len(fails) != 1 || !strings.Contains(fails[0], "AFP-V0-2403") {
		t.Fatalf("failures = %v, want the omitted planned package", fails)
	}
	if lines := verdictLines(s, floor{}); !strings.Contains(lines[0], " test-extra=0 test-omitted=1 test-misses=0") {
		t.Fatalf("verdict line = %q, want test-omitted=1", lines[0])
	}
	agree := Result{}
	checkAgreement(&agree, p, manifest{Scope: "NARROWED", Units: []manifestUnit{{Adapter: "go", Unit: "go:m/a"}, {Adapter: "go", Unit: "go:m/b"}}})
	if got := testOmittedJSON(t, agree); got != "" {
		t.Fatalf("a manifest running every planned package omitted %s", got)
	}
}

// TestAFPV02403NestedModuleUnitIsNotOmitted (review round e P2): a dependent nested module's
// synthetic `go:` unit (AFP-V0-582) is no root package. The plan may select it while the manifest
// excludes it (STR-V0-069); it is skipped only when the replayed head holds a go.mod in its
// directory. An excluded root package, a path outside the root module or one escaping the clone
// stays compared, so the gate still fails on it.
func TestAFPV02403NestedModuleUnitIsNotOmitted(t *testing.T) {
	clone := t.TempDir()
	for _, dir := range []string{"nested", "root/pkg"} {
		if err := os.MkdirAll(filepath.Join(clone, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(clone, "nested", "go.mod"), []byte("module m/nested\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	nested := nestedModuleUnits(clone, "m", []string{
		"go:m/nested: a dependent nested module (AFP-V0-582)",
		"go:m/root/pkg: not a module",
		"go:other/nested: outside the root module",
		"go:m/../nested: escapes the clone",
	})
	if len(nested) != 1 || !nested["go:m/nested"] {
		t.Fatalf("nested = %v, want only go:m/nested", nested)
	}
	var p plan
	for _, id := range []string{"go:m/a", "go:m/nested", "go:m/root/pkg"} {
		p.Plan.Selected = append(p.Plan.Selected, struct {
			UnitID  string `json:"unitId"`
			Witness struct {
				Kind string `json:"kind"`
			} `json:"witness"`
		}{UnitID: id})
	}
	res := Result{}
	checkAgreement(&res, p, manifest{Scope: "NARROWED", Units: []manifestUnit{{Adapter: "go", Unit: "go:m/a"}}, nested: nested})
	if got := testOmittedJSON(t, res); got != `["m/root/pkg"]` {
		t.Fatalf("omitted = %s, want the excluded root package only", got)
	}
}

// testOmittedJSON returns the result's encoded testOmitted list, or "" when it is absent.
func testOmittedJSON(t *testing.T, r Result) string {
	t.Helper()
	data, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	return string(doc["testOmitted"])
}
