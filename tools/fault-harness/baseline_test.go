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
	"slices"
	"testing"
)

// suiteRuns returns three runs of n passing tests named go:p#T000...
func suiteRuns(n int) []RunResult {
	var runs []RunResult
	for i := 0; i < confirmRuns; i++ {
		r := RunResult{}
		for j := 0; j < n; j++ {
			r[fmt.Sprintf("go:p#T%03d", j)] = Pass
		}
		runs = append(runs, r)
	}
	return runs
}

func TestBaselineQuarantine_FIH_V1_004(t *testing.T) {
	// Flaky baseline: 3 percent of tests fail in at least one of three runs.
	runs := suiteRuns(100)
	runs[0]["go:p#T001"], runs[1]["go:p#T002"], runs[2]["go:p#T003"] = Fail, Fail, Fail
	rec, err := evaluateBaseline("h", runs, nil)
	if err != nil || !rec.Disqualified || rec.Invalid || !reflect.DeepEqual(rec.Quarantined, []string{"go:p#T001", "go:p#T002", "go:p#T003"}) {
		t.Fatalf("flaky baseline: %+v %v", rec, err)
	}
	// Exactly 2 percent does not exceed the cap.
	runs = suiteRuns(100)
	runs[0]["go:p#T001"] = Fail
	delete(runs[1], "go:p#T002") // missing from one run: not a clean pass
	if rec, _ = evaluateBaseline("h", runs, nil); rec.Disqualified || len(rec.Quarantined) != 2 {
		t.Fatalf("2 percent: %+v", rec)
	}
	// Excluded tests, quarantined plus unrunnable, above 5 percent: INVALID
	// without disqualifying.
	runs = suiteRuns(100)
	runs[2]["go:p#T010"] = Skip // passed twice, skipped once: quarantined
	for _, id := range []string{"go:p#T020", "go:p#T021", "go:p#T022"} {
		for _, r := range runs {
			r[id] = Skip
		}
	}
	rec, _ = evaluateBaseline("h", runs, []string{"go:p#T030", "go:image#TGone"})
	if rec.Disqualified || !rec.Invalid || rec.Suite != 101 || len(rec.Quarantined) != 1 || len(rec.Unrunnable) != 5 {
		t.Fatalf("excluded cap: %+v", rec)
	}
	if _, err := evaluateBaseline("h", runs[:2], nil); err == nil {
		t.Fatal("two baseline runs accepted")
	}
	// Quarantined test omitted: Q's dependency is changed by a sealed PR whose
	// plan lacks Q; the count is 1 and names Q.
	q := "go:dep#TestQ"
	baselines := []baselineRecord{{SHA: "h", Quarantined: []string{q}}}
	planned := func(id string, entries []planEntry, deps ...string) caseResult {
		return caseResult{ID: id, HeadSHA: "h", Faults: []faultResult{{Planned: true, Plan: entries, DependentUnits: deps}}}
	}
	got := omittedQuarantined(baselines, []caseResult{
		planned("pr1", []planEntry{{Unit: "go:other"}}, "go:dep", "go:other"),
		planned("pr2", []planEntry{{Unit: "go:dep"}}, "go:dep"),                                // plan holds Q
		planned("pr3", []planEntry{{Unit: "go:other"}}, "go:other"),                            // dependency untouched
		{ID: "pr4", HeadSHA: "h", Faults: []faultResult{{DependentUnits: []string{"go:dep"}}}}, // never planned
	})
	if !reflect.DeepEqual(got, []omittedTest{{Test: q, Cases: []string{"pr1"}}}) {
		t.Fatalf("omitted quarantined: %+v", got)
	}
}

// TestRunSealedSet_FIH_V1_004_005_006 runs a sealed one-PR Go set end to end:
// generated mutants on the PR's changed line, a baseline with an always
// failing test, three-run kills, a narrowed plan and its misses.
func TestRunSealedSet_FIH_V1_004_005_006(t *testing.T) {
	ctx := context.Background()
	repo := newFixtureRepo(t)
	repo.writeGoModule()
	repo.write("flaky/flaky.go", "package flaky\n\nimport \"oracle.invalid/leaf\"\n\nfunc Seen() int { return leaf.Value() }\n")
	repo.write("flaky/flaky_test.go", "package flaky\n\nimport \"testing\"\n\nfunc TestBroken(t *testing.T) { t.Fatal(\"broken in the pinned image\") }\n")
	base := repo.commit("base")
	repo.write("leaf.go", "package leaf\n\nfunc Value() int { return 3 - 2 }\n")
	head := repo.commit("pr: compute the value")
	pre := basePrereg(t)
	pre.Repository.SHA = head
	prePath, preHash := writePrereg(t, pre)
	var w bytes.Buffer
	if code := run(ctx, []string{"generate-go", "--preregistration", prePath, "--hash", preHash, "--root", repo.root, "--pr", "7", "--base", base, "--head", head}, &w); code != 0 {
		t.Fatalf("generate-go: %d %s", code, w.String())
	}
	var c harnessCase
	if err := json.Unmarshal(w.Bytes(), &c); err != nil || len(c.Faults) != 3 {
		t.Fatalf("generated case %s %v", w.String(), err)
	}
	p, err := loadPreregistration(prePath, preHash)
	if err != nil {
		t.Fatal(err)
	}
	set := faultSet{Profile: faultSetProfile, PreregistrationSHA256: p.SHA256, Runner: "go", Cases: []harnessCase{c}}
	setPath := writeJSON(t, set)
	setBody, _ := os.ReadFile(setPath)
	sealPath := writeJSON(t, sealRecord{Profile: sealProfile, Runner: "go", PreregistrationSHA256: p.SHA256, RuleSHA256: p.RevertRule.SHA256, FaultSetSHA256: sha(setBody)})
	planner, mode := fakePlanner(t)
	setMode(t, mode, "go:oracle.invalid/leaf")
	clones := t.TempDir()
	out := filepath.Join(t.TempDir(), "results.json")
	args := []string{"run", "--preregistration", prePath, "--hash", preHash, "--set", setPath, "--seal", sealPath, "--root", repo.root, "--clones", clones, "--corvint", planner, "--out", out}
	w.Reset()
	if code := run(ctx, args, &w); code != 0 {
		t.Fatalf("run: %d %s", code, w.String())
	}
	var res results
	raw, _ := os.ReadFile(out)
	if err := json.Unmarshal(raw, &res); err != nil || len(res.Cases) != 1 || len(res.Baselines) != 1 {
		t.Fatalf("results %s %v", raw, err)
	}
	broken, value, double := "go:oracle.invalid/leaf/flaky#TestBroken", "go:oracle.invalid/leaf#TestValue", "go:oracle.invalid/leaf/other#TestDouble"
	b := res.Baselines[0]
	if b.SHA != head || b.Suite != 3 || !reflect.DeepEqual(b.Quarantined, []string{broken}) || !b.Disqualified {
		t.Fatalf("baseline %+v", b)
	}
	r := res.Cases[0]
	if r.Status != caseCounted {
		t.Fatalf("case %+v", r)
	}
	for _, f := range r.Faults {
		if f.Status != faultKilled || !f.Planned || f.PlanKind != "NARROWED" || !slices.Contains(f.Kills, value) || !slices.Contains(f.Kills, double) || slices.Contains(f.Kills, broken) || !reflect.DeepEqual(f.Misses, []string{double}) {
			t.Fatalf("fault %+v", f)
		}
	}
	if got := omittedQuarantined(res.Baselines, res.Cases); len(got) != 1 || got[0].Test != broken {
		t.Fatalf("omitted quarantined %+v", got)
	}
	if entries, _ := os.ReadDir(clones); len(entries) != 0 {
		t.Fatalf("clones leaked: %v", entries)
	}
	// Changed seeds: the run refuses and runs nothing.
	body, _ := os.ReadFile(prePath)
	if err := os.WriteFile(prePath, bytes.Replace(body, []byte("20261109"), []byte("1"), 1), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(out); err != nil {
		t.Fatal(err)
	}
	w.Reset()
	if code := run(ctx, args, &w); code != 1 || !bytes.Contains(w.Bytes(), []byte("refused: preregistration-changed")) {
		t.Fatalf("edited seed: %d %s", code, w.String())
	}
	if entries, _ := os.ReadDir(clones); len(entries) != 0 || fileExists(out) {
		t.Fatal("a refused run cloned or wrote results")
	}
}

func fileExists(path string) bool { _, err := os.Stat(path); return err == nil }
