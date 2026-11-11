// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// fixtureCases returns n cases of one kind, status and plan kind; a killed
// case carries one fault with the given tier.
func fixtureCases(prefix, kind, status, plan, tier string, n int) []caseResult {
	var out []caseResult
	for i := 0; i < n; i++ {
		c := caseResult{ID: fmt.Sprintf("%s%d", prefix, i), Kind: kind, PR: i + 1, HeadSHA: "h", Status: status, Faults: []faultResult{}}
		if status == caseCounted {
			c.Faults = append(c.Faults, faultResult{ID: c.ID + "-f", Status: faultKilled, Planned: true, PlanKind: plan, Tier: tier, Kills: []string{"go:p#T"}})
		}
		out = append(out, c)
	}
	return out
}

func fixtureResults(cases ...[]caseResult) results {
	res := results{Profile: resultsProfile, Runner: "go", PreregistrationSHA256: strings.Repeat("a", 64), FaultSetSHA256: strings.Repeat("b", 64), ExclusionBudget: 2,
		Baselines: []baselineRecord{{SHA: "h", Suite: 500, Quarantined: []string{}, Unrunnable: []string{}}}, Cases: []caseResult{}}
	for _, c := range cases {
		res.Cases = append(res.Cases, c...)
	}
	return res
}

func TestVerdictCountsAndSentence_FIH_V1_007_014(t *testing.T) {
	// 104 counted PRs: the G3 sentence, exactly, with the Wilson bound beside.
	v := buildVerdict(fixtureResults(
		fixtureCases("c", "mutants", caseCounted, "NARROWED", "line", 104),
		fixtureCases("n", "non-code", caseCounted, "NARROWED", "line", 32)))
	want := "0 misses in 104 pre-registered fault-injected cases per runner on pinned repos; 95 percent upper bound on the per-change miss rate 3/104; not a guarantee."
	if v.Status != statusZeroMiss || v.Code.Sentence != want || v.Code.Bound != "3/104" || v.Code.WilsonUpper95 != 0.035621 {
		t.Fatalf("verdict %+v", v.Code)
	}
	// Two bounds: one sentence with N = 104 and a second with N = 32.
	if v.NonCode.Sentence != strings.ReplaceAll(want, "104", "32") || v.NonCode.WilsonUpper95 != 0.107179 || v.NonCode.NarrowedWithKill != 32 || v.Code.NarrowedWithKill != 104 {
		t.Fatalf("non-code %+v", v.NonCode)
	}
	var w bytes.Buffer
	printVerdict(&w, v)
	for _, line := range []string{
		"runner go: ZERO_MISS\n",
		"code: " + want + " (95 percent Wilson upper bound 3.56 percent)\n",
		"non-code: " + strings.ReplaceAll(want, "104", "32") + " (95 percent Wilson upper bound 10.72 percent)\n",
		"granularity: line-level proven\n",
	} {
		if !strings.Contains(w.String(), line) {
			t.Fatalf("printed verdict lacks %q:\n%s", line, w.String())
		}
	}
	// Too few narrowed kills: 140 sealed, 28 with no kill and 21 with FULL
	// plans leave N = 91; the four counts are printed and the verdict is
	// INVALID with no release sentence.
	v = buildVerdict(fixtureResults(
		fixtureCases("k", "revert", caseCounted, "NARROWED", "line", 30),
		fixtureCases("m", "mutants", caseCounted, "NARROWED", "line", 61),
		fixtureCases("d", "mutants", caseDropped, "", "", 28),
		fixtureCases("f", "mutants", caseCounted, "FULL", "line", 11),
		fixtureCases("u", "mutants", caseCounted, "UNKNOWN", "line", 10),
		fixtureCases("n", "non-code", caseCounted, "NARROWED", "line", 30)))
	c := v.Code
	if c.Sealed != 140 || c.DroppedNoKill != 28 || c.FullOrUnknown != 21 || c.NarrowedWithKill != 91 || v.Status != statusInvalid || c.Status != statusInvalid || c.Sentence != "" || v.NonCode.Sentence != "" {
		t.Fatalf("too few narrowed: %s %+v", v.Status, c)
	}
	w.Reset()
	printVerdict(&w, v)
	if !strings.Contains(w.String(), "code: sealed 140, dropped (no kill) 28, plan FULL or UNKNOWN 21, narrowed with a confirmed kill (N) 91\n") || strings.Contains(w.String(), "0 misses") {
		t.Fatalf("printed counts:\n%s", w.String())
	}
	// Killing test not selected: MISS, listed, no sentence, never waived.
	missed := fixtureCases("x", "mutants", caseCounted, "NARROWED", "line", 104)
	missed[7].Faults[0].Misses = []string{"go:p#T"}
	v = buildVerdict(fixtureResults(missed, fixtureCases("n", "non-code", caseCounted, "NARROWED", "line", 32)))
	if v.Status != statusMiss || !reflect.DeepEqual(v.Code.Misses, []miss{{Case: "x7", Fault: "x7-f", Test: "go:p#T"}}) || v.Code.Sentence != "" || v.Granularity != "not proven" {
		t.Fatalf("miss verdict %+v", v)
	}
	if lo, hi := wilson(1, 100); lo != 0.001767 || hi != 0.054486 {
		t.Fatalf("wilson(1, 100) = %v, %v", lo, hi)
	}
}

func TestVerdictFailClosed_FIH_V1_004_014(t *testing.T) {
	ok := func() results {
		return fixtureResults(fixtureCases("c", "mutants", caseCounted, "NARROWED", "line", 100), fixtureCases("n", "non-code", caseCounted, "NARROWED", "line", 30))
	}
	if v := buildVerdict(ok()); v.Status != statusZeroMiss {
		t.Fatalf("baseline fixture: %+v", v.Reasons)
	}
	// Flaky baseline: more than 2 percent quarantined disqualifies.
	res := ok()
	res.Baselines[0].Quarantined, res.Baselines[0].Disqualified = make([]string, 11), true
	if v := buildVerdict(res); v.Status != statusDisqualified || v.Code.Sentence != "" {
		t.Fatalf("flaky baseline: %s", v.Status)
	}
	for name, mutate := range map[string]func(*results){
		"excluded tests above 5 percent": func(r *results) { r.Baselines[0].Invalid = true },
		"no baseline":                    func(r *results) { r.Baselines = nil },
		"failed measurement":             func(r *results) { r.Cases = append(r.Cases, fixtureCases("e", "mutants", caseError, "", "", 1)...) },
		"exclusions over budget":         func(r *results) { r.Cases = append(r.Cases, fixtureCases("p", "revert", caseExcluded, "", "", 3)...) },
		"no non-code N":                  func(r *results) { r.Cases = r.Cases[:100] },
	} {
		res := ok()
		mutate(&res)
		if v := buildVerdict(res); v.Status != statusInvalid || v.Code.Sentence != "" {
			t.Fatalf("%s: %s %v", name, v.Status, v.Reasons)
		}
	}
	// Two exclusions are within the budget of two and stay outside N.
	res = ok()
	res.Cases = append(res.Cases, fixtureCases("p", "revert", caseExcluded, "", "", 2)...)
	if v := buildVerdict(res); v.Status != statusZeroMiss || v.Code.Excluded != 2 || v.Code.Sealed != 102 || v.Code.NarrowedWithKill != 100 {
		t.Fatalf("excluded within budget: %s %+v", v.Status, v.Code)
	}
}

func TestVerdictGranularity_FIH_V1_015(t *testing.T) {
	// File-tier pytest: a zero-miss verdict whose plans carry tier file is
	// file-level, not line-level proven.
	res := fixtureResults(fixtureCases("c", "mutants", caseCounted, "NARROWED", "file", 100), fixtureCases("n", "non-code", caseCounted, "NARROWED", "file", 30))
	res.Runner = "pytest"
	if v := buildVerdict(res); v.Status != statusZeroMiss || v.Granularity != "file-level" {
		t.Fatalf("file tier: %s %s", v.Status, v.Granularity)
	}
	mixed := fixtureResults(fixtureCases("c", "mutants", caseCounted, "NARROWED", "line", 100), fixtureCases("n", "non-code", caseCounted, "NARROWED", "file", 30))
	if v := buildVerdict(mixed); v.Granularity != "file-level" {
		t.Fatalf("mixed tiers: %s", v.Granularity)
	}
	unrecorded := fixtureResults(fixtureCases("c", "mutants", caseCounted, "NARROWED", "", 100), fixtureCases("n", "non-code", caseCounted, "NARROWED", "line", 30))
	if v := buildVerdict(unrecorded); v.Status != statusZeroMiss || v.Granularity != "not proven" {
		t.Fatalf("unrecorded tier: %s", v.Granularity)
	}
}

func TestVerdictCommand_FIH_V1_007(t *testing.T) {
	ctx := context.Background()
	p := basePrereg(t)
	p.ExclusionBudget = 2
	prePath, preHash := writePrereg(t, p)
	lp, err := loadPreregistration(prePath, preHash)
	if err != nil {
		t.Fatal(err)
	}
	res := fixtureResults(fixtureCases("c", "mutants", caseCounted, "NARROWED", "line", 104), fixtureCases("n", "non-code", caseCounted, "NARROWED", "line", 32))
	res.PreregistrationSHA256 = lp.SHA256
	resPath := writeJSON(t, res)
	out := filepath.Join(t.TempDir(), "verdict.json")
	var w bytes.Buffer
	if code := run(ctx, []string{"verdict", "--preregistration", prePath, "--hash", preHash, "--results", resPath, "--out", out}, &w); code != 0 {
		t.Fatalf("verdict: %d %s", code, w.String())
	}
	var v verdictDoc
	raw, _ := os.ReadFile(out)
	if err := json.Unmarshal(raw, &v); err != nil || v.Profile != verdictProfile || v.Code.NarrowedWithKill != 104 || v.NonCode.NarrowedWithKill != 32 {
		t.Fatalf("verdict file %s %v", raw, err)
	}
	// Results measured under another pre-registration are refused.
	res.PreregistrationSHA256 = strings.Repeat("c", 64)
	w.Reset()
	if code := run(ctx, []string{"verdict", "--preregistration", prePath, "--hash", preHash, "--results", writeJSON(t, res)}, &w); code != 1 || !strings.Contains(w.String(), "refused: preregistration-changed") {
		t.Fatalf("foreign results: %d %s", code, w.String())
	}
	// A verdict other than zero-miss exits 1.
	res.PreregistrationSHA256 = lp.SHA256
	res.Cases = res.Cases[:50]
	w.Reset()
	if code := run(ctx, []string{"verdict", "--preregistration", prePath, "--hash", preHash, "--results", writeJSON(t, res)}, &w); code != 1 || !strings.Contains(w.String(), "runner go: INVALID") {
		t.Fatalf("invalid verdict: %d %s", code, w.String())
	}
}
