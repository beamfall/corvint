package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type runsRefusalDoc struct {
	Code       string `json:"code"`
	Reason     string `json:"reason"`
	Option     string `json:"option"`
	Run        int    `json:"run"`
	Unobserved int    `json:"unobserved"`
}

// PWR-V0-059: a runs-invalid-arguments refusal names its reason and, when known, the option; it
// never echoes a value.
func TestPWRV0059_RunsInvalidArgumentsReason(t *testing.T) {
	for _, c := range []struct {
		args           []string
		option, reason string
	}{
		{[]string{"runs"}, "", "SUBCOMMAND_MISSING"},
		{[]string{"runs", "unknown"}, "", "SUBCOMMAND_UNKNOWN"},
		{[]string{"runs", "baseline"}, "", "SUBCOMMAND_MISSING"},
		{[]string{"runs", "stability"}, "runs", "OPTION_REQUIRED"},
		{[]string{"runs", "stability", "--runs", "x.json", "--min-runs", "1"}, "min-runs", "VALUE_INVALID"},
		{[]string{"runs", "stability", "--runs", "x.json", "--min-runs"}, "min-runs", "VALUE_MISSING"},
		{[]string{"runs", "stability", "--runs", "x.json", "--last", "0"}, "last", "VALUE_INVALID"},
		{[]string{"runs", "stability", "--runs", "x.json", "extra"}, "", "ARGUMENT_UNEXPECTED"},
		{[]string{"runs", "stability", "--runs", "x.json", "--bogus"}, "bogus", "OPTION_UNKNOWN"},
		{[]string{"runs", "stability", "--runs", "x.json", "--secret=do-not-echo"}, "secret", "OPTION_UNKNOWN"},
		{[]string{"runs", "stability", "--runs", "x.json", "--window", "completed"}, "last", "OPTION_REQUIRED"},
		{[]string{"runs", "stability", "--runs", "x.json", "--last", "2", "--window", "bogus"}, "window", "VALUE_INVALID"},
		{[]string{"runs", "compare", "--baseline", "a.json"}, "candidate", "OPTION_REQUIRED"},
		{[]string{"runs", "baseline", "record", "--rev", "HEAD", "--full", "a.json"}, "isolated", "OPTION_REQUIRED"},
		{[]string{"runs", "baseline", "record", "--rev", "HEAD", "--full", "a.json", "--isolated", "b.json", "--allow-unobserved=maybe"}, "allow-unobserved", "VALUE_INVALID"},
		{[]string{"runs", "baseline", "diff", "--baseline", "a.json,b.json", "--runs", "c.json"}, "baseline", "VALUE_INVALID"},
	} {
		code, out, errs := runsCLI(t, c.args...)
		var d runsRefusalDoc
		decodeRuns(t, errs, &d)
		if code != 2 || out != "" || d.Code != "runs-invalid-arguments" || d.Reason != c.reason || d.Option != c.option || strings.Contains(errs, "do-not-echo") || strings.Contains(errs, "maybe") {
			t.Errorf("%q: code=%d out=%q err=%q", c.args, code, out, errs)
		}
	}
}

// runsStarted sets stats.startTime on a runsReport document.
func runsStarted(body, start string) string {
	return strings.Replace(body, `"stats":{}`, `"stats":{"startTime":"`+start+`"}`, 1)
}

// PWR-V0-063: --last N pools only the last N logical runs by stats.startTime and marks the others
// outsideWindow; a report without a start time is refused rather than placed.
func TestPWRV0063_RunsStabilityLast(t *testing.T) {
	dir := t.TempDir()
	writeRunsReports(t, dir, map[string]string{
		"old.json":     runsStarted(runsReport("failed", runsDeclared), "2026-10-01T00:00:00.000Z"),
		"new1.json":    runsStarted(runsReport("passed", runsDeclared), "2026-10-03T00:00:00.000Z"),
		"new2.json":    runsStarted(runsReport("passed", runsDeclared), "2026-10-02T00:00:00.000Z"),
		"untimed.json": runsReport("passed", runsDeclared),
	})
	code, out, errs := runsCLI(t, "--root", dir, "runs", "stability", "--runs", "new1.json,old.json,new2.json", "--last", "2", "--min-runs", "2")
	var doc struct {
		Last int `json:"last"`
		Runs []struct {
			OutsideWindow bool `json:"outsideWindow"`
		} `json:"runs"`
		Tests []struct {
			Verdict string `json:"verdict"`
		} `json:"tests"`
	}
	decodeRuns(t, out, &doc)
	if code != 0 || doc.Last != 2 || len(doc.Runs) != 3 || doc.Runs[0].OutsideWindow || !doc.Runs[1].OutsideWindow || doc.Runs[2].OutsideWindow ||
		len(doc.Tests) != 1 || doc.Tests[0].Verdict != "DETERMINISTIC" {
		t.Fatalf("window: %d %s %s", code, out, errs)
	}
	code, out, errs = runsCLI(t, "--root", dir, "runs", "stability", "--runs", "new1.json,untimed.json", "--last", "1")
	var d runsRefusalDoc
	decodeRuns(t, errs, &d)
	if code != 2 || out != "" || d.Code != "runs-report-unavailable" || d.Reason != "START_TIME_MISSING" || d.Option != "last" || d.Run != 2 {
		t.Fatalf("untimed: %d %s %s", code, out, errs)
	}
}

// PWR-V0-060, PWR-V0-061: a full run whose test was interrupted is refused as a baseline unless
// --allow-unobserved records the test as unobserved; a candidate failure of that test is then new
// (UNOBSERVED_AT_BASELINE) and blocks, never excused.
func TestPWRV0060_RunsBaselineUnobserved(t *testing.T) {
	repo := t.TempDir()
	reuseGit(t, repo, "init", "-q")
	reuseGit(t, repo, "config", "user.email", "corvint@example.test")
	reuseGit(t, repo, "config", "user.name", "Corvint Test")
	reuseCommit(t, repo, map[string]string{"a.txt": "1"})
	writeRunsReports(t, repo, map[string]string{
		"full.json": runsReport("interrupted", ""), "iso.json": runsReport("failed", ""), "cand.json": runsReport("failed", ""),
	})
	record := []string{"--root", repo, "runs", "baseline", "record", "--rev", "HEAD", "--full", "full.json", "--isolated", "iso.json"}
	code, out, errs := runsCLI(t, record...)
	var d runsRefusalDoc
	decodeRuns(t, errs, &d)
	if code != 2 || out != "" || d.Code != "runs-baseline-unavailable" || d.Reason != "FULL_RUN_INCOMPLETE" || d.Option != "full" || d.Unobserved != 1 {
		t.Fatalf("incomplete: %d %s %s", code, out, errs)
	}
	code, out, errs = runsCLI(t, append(record, "--allow-unobserved")...)
	if code != 0 || !strings.Contains(out, `"known":[]`) || !strings.Contains(out, `"unobserved":[{`) || !strings.Contains(out, `"reason":"RUN_INTERRUPTED"`) {
		t.Fatalf("allowed: %d %s %s", code, out, errs)
	}
	if err := os.WriteFile(filepath.Join(repo, "baseline.json"), []byte(out), 0o600); err != nil {
		t.Fatal(err)
	}
	code, out, errs = runsCLI(t, "--root", repo, "runs", "baseline", "diff", "--baseline", "baseline.json", "--runs", "cand.json")
	var diff struct {
		Verdict string `json:"verdict"`
		New     []struct {
			Reason string `json:"reason"`
		} `json:"new"`
		UnobservedAtBaseline []struct {
			File string `json:"file"`
		} `json:"unobservedAtBaseline"`
		Counts map[string]int `json:"counts"`
	}
	decodeRuns(t, out, &diff)
	if code != 1 || diff.Verdict != "BLOCKED" || len(diff.New) != 1 || diff.New[0].Reason != "UNOBSERVED_AT_BASELINE" ||
		len(diff.UnobservedAtBaseline) != 1 || diff.Counts["unobservedAtBaseline"] != 1 || diff.Counts["blocking"] != 1 {
		t.Fatalf("diff: %d %s %s", code, out, errs)
	}
}

// PWR-V0-062: a baseline recorded on a line the branch point does not descend from is STALE
// BASELINE_NOT_ANCESTOR and names the merge base and the commits on each side of it.
func TestPWRV0062_RunsBaselineNotAncestorSides(t *testing.T) {
	repo := t.TempDir()
	reuseGit(t, repo, "init", "-q")
	reuseGit(t, repo, "config", "user.email", "corvint@example.test")
	reuseGit(t, repo, "config", "user.name", "Corvint Test")
	fork := reuseCommit(t, repo, map[string]string{"a.txt": "1"})
	reuseGit(t, repo, "checkout", "-q", "-b", "side")
	reuseCommit(t, repo, map[string]string{"b.txt": "1"})
	reuseCommit(t, repo, map[string]string{"b.txt": "2"})
	reuseGit(t, repo, "checkout", "-q", fork)
	reuseCommit(t, repo, map[string]string{"a.txt": "2"})
	writeRunsReports(t, repo, map[string]string{"full.json": runsReport("failed", ""), "iso.json": runsReport("failed", ""), "cand.json": runsReport("failed", "")})
	code, out, errs := runsCLI(t, "--root", repo, "runs", "baseline", "record", "--rev", "side", "--full", "full.json", "--isolated", "iso.json")
	if code != 0 {
		t.Fatalf("record: %d %s", code, errs)
	}
	if err := os.WriteFile(filepath.Join(repo, "baseline.json"), []byte(out), 0o600); err != nil {
		t.Fatal(err)
	}
	code, out, errs = runsCLI(t, "--root", repo, "runs", "baseline", "diff", "--baseline", "baseline.json", "--runs", "cand.json", "--base", "HEAD")
	var diff struct {
		Staleness struct {
			Status              string `json:"status"`
			Reason              string `json:"reason"`
			MergeBase           string `json:"mergeBase"`
			BaselineSideCommits *int   `json:"baselineSideCommits"`
			BranchSideCommits   *int   `json:"branchSideCommits"`
		} `json:"staleness"`
	}
	decodeRuns(t, out, &diff)
	s := diff.Staleness
	if code != 0 || s.Status != "STALE" || s.Reason != "BASELINE_NOT_ANCESTOR" || s.MergeBase != fork ||
		s.BaselineSideCommits == nil || *s.BaselineSideCommits != 2 || s.BranchSideCommits == nil || *s.BranchSideCommits != 1 {
		t.Fatalf("staleness: %d %s %s", code, out, errs)
	}
}

// PWR-V0-176: an option and its value passed as one argument names the option and a hint, never the value.
func TestPWRV0176_RunsJoinedOptionValue(t *testing.T) {
	code, out, errs := runsCLI(t, "runs", "baseline", "diff", "--baseline", "b.json", "--runs", "r.json", "--own e2e/x.spec.ts")
	var d struct {
		runsRefusalDoc
		Hint string `json:"hint"`
	}
	decodeRuns(t, errs, &d)
	if code != 2 || out != "" || d.Reason != "OPTION_UNKNOWN" || d.Option != "own" || d.Hint != "OPTION_VALUE_JOINED" || strings.Contains(errs, "x.spec.ts") {
		t.Fatalf("code=%d out=%q err=%q", code, out, errs)
	}
}
