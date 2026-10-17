package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

type baselineDiffDoc struct {
	Profile          string   `json:"profile"`
	Verdict          string   `json:"verdict"`
	Reasons          []string `json:"reasons"`
	BaselineRevision string   `json:"baselineRevision"`
	BaselineSha256   string   `json:"baselineSha256"`
	Staleness        struct {
		Status      string `json:"status"`
		BranchPoint string `json:"branchPoint"`
		Commits     *int   `json:"commits"`
		MaxAge      int    `json:"maxAge"`
		Reason      string `json:"reason"`
	} `json:"staleness"`
	Counts map[string]int `json:"counts"`
	Known  []struct {
		Excused bool   `json:"excused"`
		Reason  string `json:"reason"`
	} `json:"known"`
}

// PWR-V0-041..045: record a baseline at a revision, then gate candidate runs on it through the
// CLI: a verified known failure passes, an own-file failure blocks with exit 1, staleness is
// FRESH within --max-age, STALE beyond it and NOT_OBSERVED without --base.
func TestPWRV0041_RunsBaselineCLI(t *testing.T) {
	repo := t.TempDir()
	reuseGit(t, repo, "init", "-q")
	reuseGit(t, repo, "config", "user.email", "corvint@example.test")
	reuseGit(t, repo, "config", "user.name", "Corvint Test")
	rev := reuseCommit(t, repo, map[string]string{"a.txt": "1"})
	writeRunsReports(t, repo, map[string]string{
		"full.json": runsReport("failed", ""), "iso.json": runsReport("failed", ""), "cand.json": runsReport("failed", ""),
		"declared.json": runsReport("failed", `"corvintRunIdentity":{"applicationRevision":"other"}`),
	})

	code, out, errs := runsCLI(t, "--root", repo, "runs", "baseline", "record", "--rev", "HEAD", "--full", "full.json", "--isolated", "iso.json")
	if code != 0 || !strings.Contains(out, `"profile":"corvint-run-baseline/0"`) || !strings.Contains(out, `"revision":"`+rev+`"`) {
		t.Fatalf("record: %d %s %s", code, out, errs)
	}
	if err := os.WriteFile(filepath.Join(repo, "baseline.json"), []byte(out), 0o600); err != nil {
		t.Fatal(err)
	}

	diff := func(want int, extra ...string) baselineDiffDoc {
		t.Helper()
		args := append([]string{"--root", repo, "runs", "baseline", "diff", "--baseline", "baseline.json", "--runs", "cand.json"}, extra...)
		code, out, errs := runsCLI(t, args...)
		if code != want {
			t.Fatalf("diff %v: exit %d, want %d: %s %s", extra, code, want, out, errs)
		}
		var d baselineDiffDoc
		decodeRuns(t, out, &d)
		return d
	}
	d := diff(0)
	if d.Verdict != "PASS" || d.BaselineRevision != rev || len(d.BaselineSha256) != 64 || d.Counts["excused"] != 1 || d.Staleness.Status != "NOT_OBSERVED" || d.Staleness.Reason != "BASE_NOT_GIVEN" {
		t.Fatalf("pass: %+v", d)
	}
	if d = diff(1, "--own", "cart.spec.ts"); d.Verdict != "BLOCKED" || len(d.Known) != 1 || d.Known[0].Reason != "OWN_FILE" || d.Counts["blocking"] != 1 {
		t.Fatalf("own file: %+v", d)
	}

	for i := 0; i < 3; i++ {
		reuseCommit(t, repo, map[string]string{"a.txt": strconv.Itoa(i + 2)})
	}
	if d = diff(0, "--base", "HEAD", "--max-age", "3"); d.Staleness.Status != "FRESH" || d.Staleness.Commits == nil || *d.Staleness.Commits != 3 {
		t.Fatalf("fresh: %+v", d.Staleness)
	}
	if d = diff(0, "--base", "HEAD", "--max-age", "2"); d.Staleness.Status != "STALE" || d.Staleness.Reason != "OLDER_THAN_MAX_AGE" || d.Verdict != "PASS" {
		t.Fatalf("stale: %+v", d)
	}

	// Refusals: a report declaring another revision, an unresolvable revision, a malformed baseline.
	if code, _, errs := runsCLI(t, "--root", repo, "runs", "baseline", "record", "--rev", "HEAD", "--full", "declared.json", "--isolated", "iso.json"); code != 2 || !strings.Contains(errs, "APPLICATION_REVISION_DIFFERS") {
		t.Fatalf("declared revision: %d %s", code, errs)
	}
	if code, _, errs := runsCLI(t, "--root", repo, "runs", "baseline", "record", "--rev", "nope", "--full", "full.json", "--isolated", "iso.json"); code != 2 || !strings.Contains(errs, "runs-revision-unresolved") {
		t.Fatalf("revision: %d %s", code, errs)
	}
	if code, _, errs := runsCLI(t, "--root", repo, "runs", "baseline", "diff", "--baseline", "cand.json", "--runs", "cand.json"); code != 2 || !strings.Contains(errs, "runs-baseline-unavailable") {
		t.Fatalf("malformed baseline: %d %s", code, errs)
	}
	// Removing the isolated evidence un-excuses the known failure.
	if err := os.Remove(filepath.Join(repo, "iso.json")); err != nil {
		t.Fatal(err)
	}
	if d = diff(1); d.Known[0].Reason != "EVIDENCE_MISSING" {
		t.Fatalf("missing evidence: %+v", d)
	}
}
