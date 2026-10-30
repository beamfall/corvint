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

// baselineRepo records a baseline over one known failure in a fresh repository and returns it.
func baselineRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	reuseGit(t, repo, "init", "-q")
	reuseGit(t, repo, "config", "user.email", "corvint@example.test")
	reuseGit(t, repo, "config", "user.name", "Corvint Test")
	reuseCommit(t, repo, map[string]string{"a.txt": "1"})
	writeRunsReports(t, repo, map[string]string{
		"full.json": runsReport("failed", ""), "iso.json": runsReport("failed", ""),
		"cand-noroot.json": runsReport("failed", ""),
		"cand-e2e.json":    runsReport("failed", `"corvintRunIdentity":{"testRootPath":"e2e"}`),
	})
	code, out, errs := runsCLI(t, "--root", repo, "runs", "baseline", "record", "--rev", "HEAD", "--full", "full.json", "--isolated", "iso.json")
	if code != 0 {
		t.Fatalf("record: %d %s", code, errs)
	}
	if err := os.WriteFile(filepath.Join(repo, "baseline.json"), []byte(out), 0o600); err != nil {
		t.Fatal(err)
	}
	return repo
}

// PWR-V0-043: with --own given, a candidate whose test root does not resolve fails closed: its
// known failure that matches no own path blocks as TEST_ROOT_UNRESOLVED instead of being excused;
// a declared testRootPath resolves the own file.
func TestPWRV0043_RunsBaselineOwnFailsClosed(t *testing.T) {
	repo := baselineRepo(t)
	for _, c := range []struct {
		runs, own, verdict, reason string
	}{
		{"cand-noroot.json", "e2e/cart.spec.ts", "BLOCKED", reasonTestRootUnresolved},
		{"cand-e2e.json", "e2e/cart.spec.ts", "BLOCKED", "OWN_FILE"},
		{"cand-e2e.json", "e2e", "BLOCKED", "OWN_FILE"},
		{"cand-e2e.json", "e2e/other.spec.ts", "PASS", "KNOWN_VERIFIED"},
		{"cand-noroot.json", "", "PASS", "KNOWN_VERIFIED"},
	} {
		args := []string{"--root", repo, "runs", "baseline", "diff", "--baseline", "baseline.json", "--runs", c.runs}
		if c.own != "" {
			args = append(args, "--own", c.own)
		}
		_, out, errs := runsCLI(t, args...)
		var d baselineDiffDoc
		decodeRuns(t, out, &d)
		if d.Verdict != c.verdict || len(d.Known) != 1 || d.Known[0].Reason != c.reason {
			t.Errorf("%s --own %q: %+v %s", c.runs, c.own, d, errs)
		}
	}
}

// PWR-V0-041, PWR-V0-042: evidence is recorded relative to --root, so a relative --root used for
// both record and diff re-reads the same reports instead of prefixing the root twice.
func TestPWRV0042_RunsBaselineRelativeRoot(t *testing.T) {
	repo := baselineRepo(t)
	t.Chdir(filepath.Dir(repo))
	rel := filepath.Base(repo)
	code, out, errs := runsCLI(t, "--root", rel, "runs", "baseline", "record", "--rev", "HEAD", "--full", "full.json", "--isolated", "iso.json")
	if code != 0 || !strings.Contains(out, `"path":"full.json"`) {
		t.Fatalf("record: %d %s %s", code, out, errs)
	}
	if err := os.WriteFile(filepath.Join(repo, "rel-baseline.json"), []byte(out), 0o600); err != nil {
		t.Fatal(err)
	}
	code, out, errs = runsCLI(t, "--root", rel, "runs", "baseline", "diff", "--baseline", "rel-baseline.json", "--runs", "cand-noroot.json")
	var d baselineDiffDoc
	decodeRuns(t, out, &d)
	if code != 0 || d.Verdict != "PASS" || len(d.Known) != 1 || d.Known[0].Reason != "KNOWN_VERIFIED" {
		t.Fatalf("diff: %d %+v %s", code, d, errs)
	}
}
