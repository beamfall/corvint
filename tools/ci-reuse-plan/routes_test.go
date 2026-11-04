// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// prRunJobs is a pull-request run's jobs listing: docs-plan classified the merge, and the
// affected-first step ran (FULL) or was skipped (a DOCS hit); narrowing-verdict ran.
func prRunJobs(docs bool) []stepJob {
	planned := "success"
	if docs {
		planned = "skipped"
	}
	return []stepJob{
		{Name: "docs-plan", Conclusion: "success", Steps: []step{
			{Name: "Classify and verify immutable documentation inputs", Conclusion: "success"},
			{Name: "Plan affected-first package order from trusted base code (advisory)", Conclusion: planned}}},
		{Name: "narrowing-verdict", Conclusion: "success"},
		{Name: "ci-narrowed-tree-record", Conclusion: "skipped"},
	}
}

// queueRunJobs is a merge-queue run's jobs listing, reused (every shard skipped) or full.
func queueRunJobs(reused bool) []stepJob {
	shard, gate := "success", "success"
	if reused {
		shard, gate = "skipped", "skipped"
	}
	jobs := []stepJob{
		{Name: "docs-plan", Conclusion: "success", Steps: []step{{Name: "Find a passed result for this exact tree", Conclusion: "success"}}},
		{Name: "artifact-integrity", Conclusion: "success", Steps: []step{
			{Name: "Find a passed pull-request verdict for this tree", Conclusion: "success"},
			{Name: "Verify native Go archives", Conclusion: gate}}},
		{Name: "doc-gates-reuse", Conclusion: "success", Steps: []step{{Name: "Find a passed doc-gates verdict for this tree", Conclusion: "success"}}},
		{Name: "doc-gates", Conclusion: "success", Steps: []step{{Name: "Shared documentation gates (including host versions and use-case pins)", Conclusion: gate}}},
	}
	if reused {
		return append(jobs, stepJob{Name: "go-product-shard", Conclusion: "skipped"})
	}
	for i := 0; i < 8; i++ {
		jobs = append(jobs, stepJob{Name: "go-product-shard (" + strconv.Itoa(i) + ")", Conclusion: shard})
	}
	return jobs
}

func routeRun(id int64, event string) run {
	r := passedRun(id)
	r.Event = event
	return r
}

func routeOf(t *testing.T, rep routesReport, name string) routeReport {
	t.Helper()
	for _, r := range rep.Routes {
		if r.Name == name {
			return r
		}
	}
	t.Fatalf("no route %s in %+v", name, rep)
	return routeReport{}
}

// TestAFPV0105StaleQualificationIsFlaggedDead is the V1-4234 fixture: the DOCS classifier's
// qualification went stale, so every eligible pull-request run classified FULL and the route never
// fired while the merge queue's reuse kept firing. DOCS is DEAD, REUSE is HEALTHY, and the report
// is not OK.
func TestAFPV0105StaleQualificationIsFlaggedDead(t *testing.T) {
	var runs []run
	listed := map[int64][]stepJob{}
	for i := int64(1); i <= 12; i++ {
		runs = append(runs, routeRun(100+i, "pull_request"), routeRun(200+i, "merge_group"))
		listed[100+i] = prRunJobs(false)
		listed[200+i] = queueRunJobs(i%2 == 0)
	}
	jobs := func(id int64) ([]stepJob, bool) { j, ok := listed[id]; return j, ok }
	rep := RouteHealth(testRepo, runs, jobs, 30, 10)
	if docs := routeOf(t, rep, "DOCS"); docs.Status != "DEAD" || docs.Eligible != 12 || docs.Hits != 0 || docs.LastHitRunID != nil {
		t.Fatalf("DOCS = %+v, want DEAD over 12 eligible runs", docs)
	}
	if reuse := routeOf(t, rep, "REUSE"); reuse.Status != "HEALTHY" || reuse.Hits != 6 || reuse.LastHitRunID == nil || *reuse.LastHitRunID != 212 {
		t.Fatalf("REUSE = %+v, want HEALTHY with 6 hits, newest 212", reuse)
	}
	if integrity := routeOf(t, rep, "INTEGRITY_REUSE"); integrity.Status != "HEALTHY" || integrity.Hits != 6 {
		t.Fatalf("INTEGRITY_REUSE = %+v", integrity)
	}
	if narrowed := routeOf(t, rep, "NARROWED"); narrowed.Status != "DEAD" {
		t.Fatalf("NARROWED = %+v, want DEAD (no narrowed record in 12 MET runs)", narrowed)
	}
	if rep.OK {
		t.Fatal("a DEAD route left the report OK")
	}
	// A skipped hit step in a failed job is no hit: an earlier step's failure skipped it.
	failed := prRunJobs(true)
	failed[0].Conclusion = "failure"
	listed[105] = failed
	if docs := routeOf(t, RouteHealth(testRepo, runs, jobs, 30, 10), "DOCS"); docs.Hits != 0 || docs.Eligible != 11 {
		t.Fatalf("failed docs-plan: DOCS = %+v, want neither eligible nor a hit", docs)
	}
	// One docs-only pull request in the window brings the route back.
	listed[105] = prRunJobs(true)
	if docs := routeOf(t, RouteHealth(testRepo, runs, jobs, 30, 10), "DOCS"); docs.Status != "HEALTHY" || docs.Hits != 1 || *docs.LastHitRunID != 105 {
		t.Fatalf("DOCS = %+v, want one hit", docs)
	}
}

// TestAFPV0105WindowsThresholdsAndSignals: only the newest window eligible runs count, fewer than
// min-eligible is NOT_OBSERVED (never healthy), a renamed signal step is SIGNAL_MISSING, an
// unreadable jobs listing is counted as unread, and runs of another repository, workflow or
// event, or not completed, are never eligible.
func TestAFPV0105WindowsThresholdsAndSignals(t *testing.T) {
	listed := map[int64][]stepJob{}
	var runs []run
	// The oldest pull-request run was a DOCS hit; the newest ten were not.
	for i := int64(1); i <= 11; i++ {
		runs = append(runs, routeRun(i, "pull_request"))
		listed[i] = prRunJobs(i == 1)
	}
	jobs := func(id int64) ([]stepJob, bool) { j, ok := listed[id]; return j, ok }
	if docs := routeOf(t, RouteHealth(testRepo, runs, jobs, 10, 10), "DOCS"); docs.Status != "DEAD" || docs.Eligible != 10 {
		t.Fatalf("window 10: DOCS = %+v, want DEAD (the hit is outside the window)", docs)
	}
	if docs := routeOf(t, RouteHealth(testRepo, runs, jobs, 11, 10), "DOCS"); docs.Status != "HEALTHY" {
		t.Fatalf("window 11: DOCS = %+v, want HEALTHY", docs)
	}
	if docs := routeOf(t, RouteHealth(testRepo, runs[1:5], jobs, 30, 10), "DOCS"); docs.Status != "NOT_OBSERVED" || docs.Eligible != 4 {
		t.Fatalf("4 eligible: DOCS = %+v, want NOT_OBSERVED", docs)
	}
	rep := RouteHealth(testRepo, runs[1:5], jobs, 30, 10)
	if q := routeOf(t, rep, "REUSE"); q.Status != "NOT_OBSERVED" || q.Eligible != 0 || !rep.OK {
		t.Fatalf("no queue runs: REUSE = %+v, ok %v; want NOT_OBSERVED and OK", q, rep.OK)
	}
	// A renamed hit step: the route cannot be observed at all, which fails loud.
	renamed := map[int64][]stepJob{}
	for id := range listed {
		j := prRunJobs(false)
		j[0].Steps[1].Name = "Plan the affected-first order"
		renamed[id] = j
	}
	if docs := routeOf(t, RouteHealth(testRepo, runs, func(id int64) ([]stepJob, bool) { j, ok := renamed[id]; return j, ok }, 30, 10), "DOCS"); docs.Status != "SIGNAL_MISSING" {
		t.Fatalf("renamed hit step: DOCS = %+v", docs)
	}
	// A renamed eligibility step: no run is ever eligible.
	for id := range renamed {
		renamed[id][0].Steps[0].Name = "Classify documentation"
	}
	if rep := RouteHealth(testRepo, runs, func(id int64) ([]stepJob, bool) { j, ok := renamed[id]; return j, ok }, 30, 10); routeOf(t, rep, "DOCS").Status != "SIGNAL_MISSING" || rep.OK {
		t.Fatalf("renamed eligibility step: %+v", rep)
	}
	// Unadmitted runs and an unread listing.
	others := []run{routeRun(50, "pull_request"), routeRun(51, "pull_request"), routeRun(52, "pull_request"), routeRun(53, "pull_request"), routeRun(54, "pull_request")}
	others[0].Repository.FullName = "fork/corvint"
	others[1].Path = ".github/workflows/other.yml"
	others[2].Status = "in_progress"
	others[3].Event = "workflow_dispatch"
	for _, r := range others[:4] {
		listed[r.ID] = prRunJobs(true)
	}
	if docs := routeOf(t, RouteHealth(testRepo, append(runs[1:], others...), jobs, 30, 10), "DOCS"); docs.Status != "DEAD" || docs.Unread != 1 || docs.Eligible != 10 {
		t.Fatalf("unadmitted runs: DOCS = %+v, want DEAD over 10 with one unread", docs)
	}
}

// TestAFPV0105RepeatedRunIDsCountOnce: overlapping repeatable --runs listings may name a run more
// than once; each run id counts once (its first listing), so one run copied ten times never meets
// --min-eligible=10.
func TestAFPV0105RepeatedRunIDsCountOnce(t *testing.T) {
	listed := map[int64][]stepJob{7: prRunJobs(false)}
	jobs := func(id int64) ([]stepJob, bool) { j, ok := listed[id]; return j, ok }
	var copies []run
	for i := 0; i < 10; i++ {
		copies = append(copies, routeRun(7, "pull_request"))
	}
	if docs := routeOf(t, RouteHealth(testRepo, copies, jobs, 30, 10), "DOCS"); docs.Status != "NOT_OBSERVED" || docs.Eligible != 1 {
		t.Fatalf("one run listed 10x: DOCS = %+v, want NOT_OBSERVED over 1 eligible run", docs)
	}
	// Two overlapping listings (runs 1-6 and 4-9) are nine distinct runs, not twelve.
	var overlap []run
	for _, span := range [][2]int64{{1, 6}, {4, 9}} {
		for id := span[0]; id <= span[1]; id++ {
			overlap = append(overlap, routeRun(id, "pull_request"))
			listed[id] = prRunJobs(false)
		}
	}
	if docs := routeOf(t, RouteHealth(testRepo, overlap, jobs, 30, 10), "DOCS"); docs.Status != "NOT_OBSERVED" || docs.Eligible != 9 {
		t.Fatalf("overlapping listings: DOCS = %+v, want NOT_OBSERVED over 9 eligible runs", docs)
	}
	// The first listing of a repeated id is the one counted.
	stale := routeRun(10, "pull_request")
	stale.Status = "in_progress"
	listed[10] = prRunJobs(false)
	if docs := routeOf(t, RouteHealth(testRepo, append(append([]run{stale}, overlap...), routeRun(10, "pull_request")), jobs, 30, 10), "DOCS"); docs.Eligible != 9 {
		t.Fatalf("first listing in_progress: DOCS = %+v, want 9 eligible runs", docs)
	}
}

// TestAFPV0105RoutesCommand: the subcommand reads retained listings, prints the report, exits 1
// on a DEAD route, 0 when every route is healthy or not observed, and 2 on a usage error; its
// help names the inputs and an example.
func TestAFPV0105RoutesCommand(t *testing.T) {
	dir := t.TempDir()
	jobsDir := filepath.Join(dir, "jobs")
	if err := os.Mkdir(jobsDir, 0o700); err != nil {
		t.Fatal(err)
	}
	var listed []any
	for i := 1; i <= 10; i++ {
		listed = append(listed, map[string]any{"id": i, "head_sha": testHead, "event": "pull_request", "status": "completed",
			"conclusion": "success", "path": workflowPath, "repository": map[string]any{"full_name": testRepo}})
		writeJSON(t, filepath.Join(jobsDir, strconv.Itoa(i)+".json"), map[string]any{"jobs": prRunJobs(false)})
	}
	writeJSON(t, filepath.Join(dir, "runs.json"), map[string]any{"workflow_runs": listed})
	run := func(args ...string) (int, string, string) {
		var out, errOut bytes.Buffer
		code := routesMain(args, &out, &errOut)
		return code, out.String(), errOut.String()
	}
	code, out, _ := run("--repository", testRepo, "--runs", filepath.Join(dir, "runs.json"), "--jobs", jobsDir, "--window", "30", "--min-eligible", "10")
	var rep routesReport
	if code != 1 || json.Unmarshal([]byte(out), &rep) != nil || rep.Profile != routesProfile || rep.OK || routeOf(t, rep, "DOCS").Status != "DEAD" {
		t.Fatalf("exit %d, report %s", code, out)
	}
	if code, out, _ := run("--repository", testRepo, "--runs", filepath.Join(dir, "runs.json"), "--jobs", jobsDir, "--min-eligible", "11"); code != 0 ||
		!strings.Contains(out, `"status": "NOT_OBSERVED"`) {
		t.Fatalf("min-eligible 11: exit %d, report %s", code, out)
	}
	for _, args := range [][]string{{}, {"--repository", testRepo, "--jobs", jobsDir}, {"--repository", testRepo, "--runs", filepath.Join(dir, "runs.json"), "--jobs", jobsDir, "--window", "5", "--min-eligible", "6"},
		{"--repository", testRepo, "--runs", filepath.Join(dir, "missing.json"), "--jobs", jobsDir}} {
		if code, _, _ := run(args...); code != 2 {
			t.Errorf("%q: exit %d, want 2", args, code)
		}
	}
	code, _, help := run("--help")
	if code != 0 || !strings.Contains(help, "usage: ci-reuse-plan routes") || !strings.Contains(help, "example: ") || !strings.Contains(help, "-min-eligible") {
		t.Fatalf("help: exit %d\n%s", code, help)
	}
}
