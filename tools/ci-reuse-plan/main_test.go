// SPDX-License-Identifier: Apache-2.0
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const (
	testRepo   = "beamfall/corvint"
	testTree   = "1111111111111111111111111111111111111111"
	testHead   = "2222222222222222222222222222222222222222"
	testTarget = "4444444444444444444444444444444444444444"
)

func passedRun(id int64) run {
	r := run{ID: id, HeadSHA: testHead, Event: "pull_request", Status: "completed", Conclusion: "success", Path: workflowPath}
	r.Repository.FullName = testRepo
	return r
}

// passedQueueRun is a successful merge-queue run of the pushed commit itself.
func passedQueueRun(id int64) run {
	r := passedRun(id)
	r.Event, r.HeadSHA = "merge_group", testTarget
	return r
}

// prMerge is the merge commit a pull-request run tested; only a queue run's target is checked.
const prMerge = "5555555555555555555555555555555555555555"

// evidence is the retained jobs listings (by run id) and job logs (by job id) Decide reads.
type evidence struct {
	jobs map[int64][]job
	logs map[int64]string
}

func (e evidence) listJobs(id int64) ([]job, bool) { j, ok := e.jobs[id]; return j, ok }

func (e evidence) readLog(id int64) ([]byte, bool) { l, ok := e.logs[id]; return []byte(l), ok }

func decide(event string, shards int, runs []run, e evidence) decision {
	return Decide(testRepo, event, testTree, testHead, testTarget, "", shards, runs, e.listJobs, e.readLog)
}

// recordLog is a hosted record-job log: checkout chatter, the echoed script and one record line.
func recordLog(tree, target, head string, shards int) string {
	return "2026-10-21T16:40:00.1000000Z ##[group]Run actions/checkout@v7\r\n" +
		"2026-10-21T16:40:01.2000000Z HEAD is now at 5555555 Merge 2222222 into 4444444\r\n" +
		"2026-10-21T16:40:02.3000000Z \x1b[36;1mprintf 'corvint-ci-tested-tree/1 tree=%s target=%s head=%s shards=%s\\n' \"$tree\" \"$GITHUB_SHA\" \"$head\" \"$SHARDS\"\x1b[0m\r\n" +
		"2026-10-21T16:40:03.4000000Z corvint-ci-tested-tree/1 tree=" + tree + " target=" + target + " head=" + head + " shards=" + strconv.Itoa(shards) + "\r\n" +
		"2026-10-21T16:40:04.5000000Z Cleaning up orphan processes\r\n"
}

// shardJobs are the go-product shards of run runID; they run repository code.
func shardJobs(runID int64, head string, shards int) []job {
	var out []job
	for shard := 0; shard < shards; shard++ {
		out = append(out, job{ID: runID*100 + int64(shard) + 1, RunID: runID, Name: fmt.Sprintf("go-product-shard (%d)", shard), HeadSHA: head, Status: "completed", Conclusion: "success"})
	}
	return out
}

// withRecord is the evidence of run runID (head head) whose record job jobID logged log.
func withRecord(e evidence, runID, jobID int64, head, log string) evidence {
	if e.jobs == nil {
		e = evidence{jobs: map[int64][]job{}, logs: map[int64]string{}}
	}
	e.jobs[runID] = append(shardJobs(runID, head, 4), job{ID: jobID, RunID: runID, Name: RecordJob, HeadSHA: head, Status: "completed", Conclusion: "success"})
	e.logs[jobID] = log
	return e
}

// prEvidence is a pull-request run's record of tree over shards.
func prEvidence(e evidence, runID int64, tree string, shards int) evidence {
	return withRecord(e, runID, runID*10, testHead, recordLog(tree, prMerge, testHead, shards))
}

// queueEvidence is a merge-queue run's record: its head and target are the queue commit.
func queueEvidence(e evidence, runID int64, tree string, shards int) evidence {
	return withRecord(e, runID, runID*10, testTarget, recordLog(tree, testTarget, testHead, shards))
}

func TestAFPV0024ReusesOnlyAnExactTreeRecordedByEveryShard(t *testing.T) {
	for _, event := range []string{"push", "merge_group"} {
		d := decide(event, 4, []run{passedRun(7)}, prEvidence(evidence{}, 7, testTree, 4))
		if d.Mode != "REUSE" || d.RunID == nil || *d.RunID != 7 || d.RecordJobID == nil || *d.RecordJobID != 70 || len(d.Records) != 4 || d.Records[3] != RecordName(testTree, 3, 4) ||
			d.Tree != testTree || d.Head != testHead || d.Profile != profile {
			t.Fatalf("%s: decision = %+v", event, d)
		}
		// An earlier run of the same head against an older base recorded another tree; the
		// later run against the merged base is the one reused.
		other := "3333333333333333333333333333333333333333"
		d = decide(event, 4, []run{passedRun(5), passedRun(9)}, prEvidence(prEvidence(evidence{}, 5, other, 4), 9, testTree, 4))
		if d.Mode != "REUSE" || *d.RunID != 9 || *d.RecordJobID != 90 {
			t.Fatalf("%s: decision = %+v", event, d)
		}
	}
}

// AFP-V0-170 (V1-2271): the shard jobs run repository code, so anything they can write (an
// artifact name, a line in their own log) is never evidence. Only the record job's own log is.
func TestAFPV0170OnlyTheRecordJobLogVouchesForTheTree(t *testing.T) {
	forged := recordLog(testTree, prMerge, testHead, 4)
	// A shard job printed a perfect record line, but the run has no record job.
	noRecordJob := evidence{jobs: map[int64][]job{7: shardJobs(7, testHead, 4)}, logs: map[int64]string{701: forged, 70: forged}}
	alterJob := func(f func(*job)) evidence {
		e := prEvidence(evidence{}, 7, testTree, 4)
		jobs := e.jobs[7]
		f(&jobs[len(jobs)-1])
		return e
	}
	twoRecordJobs := prEvidence(evidence{}, 7, testTree, 4)
	twoRecordJobs.jobs[7] = append(twoRecordJobs.jobs[7], job{ID: 71, RunID: 7, Name: RecordJob, HeadSHA: testHead, Status: "completed", Conclusion: "success"})
	twoRecordJobs.logs[71] = forged
	withLog := func(log string) evidence { return withRecord(evidence{}, 7, 70, testHead, log) }
	line := "2026-10-21T16:40:03.4000000Z corvint-ci-tested-tree/1 tree=" + testTree + " target=" + prMerge + " head=" + testHead + " shards=4"
	for name, e := range map[string]evidence{
		"no record job, forged shard log": noRecordJob,
		"jobs listing unavailable":        {logs: map[int64]string{70: forged}},
		"record log unavailable":          {jobs: prEvidence(evidence{}, 7, testTree, 4).jobs, logs: map[int64]string{}},
		"two record jobs":                 twoRecordJobs,
		"record job failed":               alterJob(func(j *job) { j.Conclusion = "failure" }),
		"record job skipped":              alterJob(func(j *job) { j.Conclusion = "skipped" }),
		"record job in progress":          alterJob(func(j *job) { j.Status, j.Conclusion = "in_progress", "" }),
		"record job of another run":       alterJob(func(j *job) { j.RunID = 8 }),
		"record job of another head":      alterJob(func(j *job) { j.HeadSHA = testTarget }),
		"record job without id":           alterJob(func(j *job) { j.ID = 0 }),
		"record job renamed":              alterJob(func(j *job) { j.Name = RecordJob + " (0)" }),
		"no record line":                  withLog("2026-10-21T16:40:00Z HEAD is now at 5555555\n"),
		"two record lines":                withLog(line + "\n" + line + "\n"),
		"record line after other text":    withLog("HEAD is now at 5555555 " + strings.TrimPrefix(line, "2026-10-21T16:40:03.4000000Z ") + "\n"),
		"record line with a suffix":       withLog(line + " shard=0\n"),
		"other tree":                      withLog(recordLog(testHead, prMerge, testTree, 4)),
		"other head":                      withLog(recordLog(testTree, prMerge, testTarget, 4)),
		"other shard count":               withLog(recordLog(testTree, prMerge, testHead, 8)),
		"short tree":                      withLog(strings.Replace(forged, "tree="+testTree, "tree="+testTree[:39], 1)),
		"older record profile":            withLog(strings.ReplaceAll(forged, "corvint-ci-tested-tree/1 ", "corvint-ci-tested-tree/0 ")),
	} {
		for _, event := range []string{"push", "merge_group"} {
			if d := decide(event, 4, []run{passedRun(7)}, e); d.Mode != "FULL" || d.RunID != nil || d.RecordJobID != nil || len(d.Records) != 0 || d.Reason == "" {
				t.Errorf("%s, %s: decision = %+v", event, name, d)
			}
		}
	}
}

func TestAFPV0024AnythingElseRunsInFull(t *testing.T) {
	alter := func(f func(*run)) []run { r := passedRun(7); f(&r); return []run{r} }
	good := prEvidence(evidence{}, 7, testTree, 4)
	cases := map[string]struct {
		runs     []run
		evidence evidence
		shards   int
	}{
		"no runs":                    {nil, good, 4},
		"failed run":                 {alter(func(r *run) { r.Conclusion = "failure" }), good, 4},
		"cancelled run":              {alter(func(r *run) { r.Conclusion = "cancelled" }), good, 4},
		"in progress":                {alter(func(r *run) { r.Status = "in_progress" }), good, 4},
		"push event":                 {alter(func(r *run) { r.Event = "push" }), good, 4},
		"other workflow":             {alter(func(r *run) { r.Path = ".github/workflows/other.yml" }), good, 4},
		"run of another repository":  {alter(func(r *run) { r.Repository.FullName = "other/corvint" }), good, 4},
		"other head":                 {alter(func(r *run) { r.HeadSHA = testTree }), good, 4},
		"pull request of the target": {alter(func(r *run) { r.HeadSHA = testTarget }), good, 4},
		"no run id":                  {alter(func(r *run) { r.ID = 0 }), good, 4},
		"evidence unavailable":       {[]run{passedRun(7)}, evidence{}, 4},
		"evidence of another run":    {[]run{passedRun(7)}, prEvidence(evidence{}, 8, testTree, 4), 4},
		"other shard count":          {[]run{passedRun(7)}, good, 8},
	}
	for _, event := range []string{"push", "merge_group"} {
		for name, c := range cases {
			d := decide(event, c.shards, c.runs, c.evidence)
			if d.Mode != "FULL" || d.RunID != nil || len(d.Records) != 0 || d.Reason == "" {
				t.Errorf("%s, %s: decision = %+v", event, name, d)
			}
		}
	}
}

// V1-0760: a main push reuses the merge-queue run of the pushed commit itself.
func TestAFPV0024PushReusesTheQueueRunOfThePushedCommit(t *testing.T) {
	d := decide("push", 4, []run{passedQueueRun(11)}, queueEvidence(evidence{}, 11, testTree, 4))
	if d.Mode != "REUSE" || d.RunID == nil || *d.RunID != 11 || *d.RecordJobID != 110 || len(d.Records) != 4 || d.Head != testHead || d.Tree != testTree {
		t.Fatalf("decision = %+v", d)
	}
	// A pull-request run without a matching record listed first does not hide the queue run.
	d = decide("push", 4, []run{passedRun(7), passedQueueRun(11)},
		queueEvidence(prEvidence(evidence{}, 7, "3333333333333333333333333333333333333333", 4), 11, testTree, 4))
	if d.Mode != "REUSE" || *d.RunID != 11 {
		t.Fatalf("decision = %+v", d)
	}
}

func TestAFPV0024QueueRunRefusalsRunInFull(t *testing.T) {
	alter := func(f func(*run)) run { r := passedQueueRun(11); f(&r); return r }
	good := queueEvidence(evidence{}, 11, testTree, 4)
	cases := map[string]struct {
		run      run
		evidence evidence
	}{
		"failed":                   {alter(func(r *run) { r.Conclusion = "failure" }), good},
		"cancelled":                {alter(func(r *run) { r.Conclusion = "cancelled" }), good},
		"in progress":              {alter(func(r *run) { r.Status = "in_progress"; r.Conclusion = "" }), good},
		"other workflow path":      {alter(func(r *run) { r.Path = ".github/workflows/ci-control-plane.yml" }), good},
		"other repository":         {alter(func(r *run) { r.Repository.FullName = "fork/corvint" }), good},
		"wrong head":               {alter(func(r *run) { r.HeadSHA = testTree }), good},
		"merged pull-request head": {alter(func(r *run) { r.HeadSHA = testHead }), good},
		"different shard count":    {passedQueueRun(11), queueEvidence(evidence{}, 11, testTree, 8)},
		// The record names another queue commit as the target.
		"record of another target":  {passedQueueRun(11), withRecord(evidence{}, 11, 110, testTarget, recordLog(testTree, prMerge, testHead, 4))},
		"record job of the PR head": {passedQueueRun(11), withRecord(evidence{}, 11, 110, testHead, recordLog(testTree, testTarget, testHead, 4))},
		"other tree":                {passedQueueRun(11), queueEvidence(evidence{}, 11, testHead, 4)},
		"evidence unavailable":      {passedQueueRun(11), evidence{}},
	}
	for name, c := range cases {
		d := decide("push", 4, []run{c.run}, c.evidence)
		if d.Mode != "FULL" || d.RunID != nil || len(d.Records) != 0 || d.Reason == "" {
			t.Errorf("%s: decision = %+v", name, d)
		}
	}
}

// AFP-V0-046: a merge-queue decision admits only a pull-request run of the queue commit's
// second parent; a merge-queue run, even of the queue commit itself, is never reused there.
func TestAFPV0046QueueDecisionAdmitsOnlyThePullRequestRun(t *testing.T) {
	queue := queueEvidence(evidence{}, 11, testTree, 4)
	d := decide("merge_group", 4, []run{passedQueueRun(11)}, queue)
	if d.Mode != "FULL" || d.RunID != nil {
		t.Fatalf("queue run admitted on merge_group: %+v", d)
	}
	wrong := passedRun(7)
	wrong.HeadSHA = testTarget
	d = decide("merge_group", 4, []run{wrong}, queueEvidence(evidence{}, 7, testTree, 4))
	if d.Mode != "FULL" || d.RunID != nil {
		t.Fatalf("pull-request run of the wrong head admitted: %+v", d)
	}
	for _, event := range []string{"", "pull_request", "release"} {
		if d := decide(event, 4, []run{passedQueueRun(11)}, queue); d.Mode != "FULL" {
			t.Fatalf("event %q admitted a queue run: %+v", event, d)
		}
	}
}

func TestAFPV0024UnreadableListingContributesNoRun(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "queue-runs.json")
	if err := os.WriteFile(good, []byte(`{"workflow_runs":[{"id":11,"event":"merge_group"}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	empty := filepath.Join(dir, "runs.json")
	if err := os.WriteFile(empty, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	runs, ok := readRuns([]string{empty, filepath.Join(dir, "absent.json"), good})
	if !ok || len(runs) != 1 || runs[0].ID != 11 {
		t.Fatalf("runs = %+v, ok = %v", runs, ok)
	}
	if runs, ok := readRuns([]string{empty}); ok || len(runs) != 0 {
		t.Fatalf("unreadable listing: runs = %+v, ok = %v", runs, ok)
	}
}

// testBase is the queue entry's first parent, the queue base on main (AFP-V0-734).
const testBase = "6666666666666666666666666666666666666666"

// narrowedLog is a hosted ci-narrowed-tree-record log with one AFP-V0-734 record line.
func narrowedLog(tree, target, base, head string, shards int) string {
	return "2026-10-28T10:00:00.1000000Z ##[group]Run actions/checkout@v7\r\n" +
		"2026-10-28T10:00:01.2000000Z corvint-ci-tested-tree/2 scope=NARROWED tree=" + tree + " target=" + target + " base=" + base +
		" head=" + head + " shards=" + strconv.Itoa(shards) + "\r\n"
}

// narrowedEvidence is a narrowed pull-request run whose only record job is ci-narrowed-tree-record.
func narrowedEvidence(runID int64, name, log string) evidence {
	e := evidence{jobs: map[int64][]job{}, logs: map[int64]string{}}
	e.jobs[runID] = append(shardJobs(runID, testHead, 4), job{ID: runID * 10, RunID: runID, Name: name, HeadSHA: testHead, Status: "completed", Conclusion: "success"})
	e.logs[runID*10] = log
	return e
}

// TestAFPV0734ReusesANarrowedPullRequestRunOfTheSameRange: a merge-queue entry reuses a
// narrowed pull-request run only when that run's ci-narrowed-tree-record job named this tree, this
// head and this entry's base, so the selection covered the same range; a main push never does.
func TestAFPV0734ReusesANarrowedPullRequestRunOfTheSameRange(t *testing.T) {
	good := narrowedLog(testTree, prMerge, testBase, testHead, 4)
	decideBase := func(event, base string, e evidence) decision {
		return Decide(testRepo, event, testTree, testHead, testTarget, base, 4, []run{passedRun(7)}, e.listJobs, e.readLog)
	}
	d := decideBase("merge_group", testBase, narrowedEvidence(7, NarrowedRecordJob, good))
	if d.Mode != "REUSE" || d.Scope != "NARROWED" || d.Base != testBase || d.RunID == nil || *d.RunID != 7 || d.RecordJobID == nil ||
		*d.RecordJobID != 70 || len(d.Records) != 4 {
		t.Fatalf("decision = %+v", d)
	}
	other := "3333333333333333333333333333333333333333"
	for name, tc := range map[string]struct {
		event, base string
		e           evidence
	}{
		"a main push":                     {"push", testBase, narrowedEvidence(7, NarrowedRecordJob, good)},
		"no entry base":                   {"merge_group", "", narrowedEvidence(7, NarrowedRecordJob, good)},
		"another base":                    {"merge_group", other, narrowedEvidence(7, NarrowedRecordJob, good)},
		"another tree":                    {"merge_group", testBase, narrowedEvidence(7, NarrowedRecordJob, narrowedLog(other, prMerge, testBase, testHead, 4))},
		"another head":                    {"merge_group", testBase, narrowedEvidence(7, NarrowedRecordJob, narrowedLog(testTree, prMerge, testBase, other, 4))},
		"another shard count":             {"merge_group", testBase, narrowedEvidence(7, NarrowedRecordJob, narrowedLog(testTree, prMerge, testBase, testHead, 8))},
		"two record lines":                {"merge_group", testBase, narrowedEvidence(7, NarrowedRecordJob, good+good)},
		"the line in the full job's log":  {"merge_group", testBase, narrowedEvidence(7, RecordJob, good)},
		"a full line in the narrowed job": {"merge_group", testBase, narrowedEvidence(7, NarrowedRecordJob, recordLog(testTree, prMerge, testHead, 4))},
	} {
		if d := decideBase(tc.event, tc.base, tc.e); d.Mode != "FULL" {
			t.Errorf("%s: decision = %+v, want FULL", name, d)
		}
	}
	// A run whose full record job also passed is reused as FULL.
	both := prEvidence(evidence{}, 7, testTree, 4)
	both.jobs[7] = append(both.jobs[7], job{ID: 71, RunID: 7, Name: NarrowedRecordJob, HeadSHA: testHead, Status: "completed", Conclusion: "success"})
	both.logs[71] = good
	if d := decideBase("merge_group", testBase, both); d.Mode != "REUSE" || d.Scope != "FULL" || *d.RecordJobID != 70 {
		t.Fatalf("decision = %+v, want the full record", d)
	}
}

// TestAFPV0812NamesTheNarrowedRecordOfAMovedBase: a merge-queue entry whose head's narrowed
// pull-request record names another base and tree stays FULL and names that record as its delta
// candidate; a push, a missing base, another head or shard count, a forged line and an exact
// reuse name none.
func TestAFPV0812NamesTheNarrowedRecordOfAMovedBase(t *testing.T) {
	other := "3333333333333333333333333333333333333333"
	moved := narrowedLog(other, prMerge, other, testHead, 4)
	decideBase := func(event, base string, e evidence) decision {
		return Decide(testRepo, event, testTree, testHead, testTarget, base, 4, []run{passedRun(7)}, e.listJobs, e.readLog)
	}
	d := decideBase("merge_group", testBase, narrowedEvidence(7, NarrowedRecordJob, moved))
	want := delta{RunID: 7, RecordJobID: 70, Tree: other, Target: prMerge, Base: other, Head: testHead}
	if d.Mode != "FULL" || d.Delta == nil || *d.Delta != want {
		t.Fatalf("decision = %+v, delta = %+v", d, d.Delta)
	}
	if b, err := json.Marshal(d); err != nil || !strings.Contains(string(b), `"delta":{"runId":7,"recordJobId":70,"tree":"`+other) {
		t.Fatalf("encoded = %s, %v", b, err)
	}
	for name, tc := range map[string]struct {
		event, base string
		e           evidence
	}{
		"a main push":         {"push", testBase, narrowedEvidence(7, NarrowedRecordJob, moved)},
		"no entry base":       {"merge_group", "", narrowedEvidence(7, NarrowedRecordJob, moved)},
		"another head":        {"merge_group", testBase, narrowedEvidence(7, NarrowedRecordJob, narrowedLog(other, prMerge, other, other, 4))},
		"another shard count": {"merge_group", testBase, narrowedEvidence(7, NarrowedRecordJob, narrowedLog(other, prMerge, other, testHead, 8))},
		"two record lines":    {"merge_group", testBase, narrowedEvidence(7, NarrowedRecordJob, moved+moved)},
		"the full job's log":  {"merge_group", testBase, narrowedEvidence(7, RecordJob, moved)},
		"an exact reuse":      {"merge_group", testBase, narrowedEvidence(7, NarrowedRecordJob, narrowedLog(testTree, prMerge, testBase, testHead, 4))},
		"a full record line":  {"merge_group", testBase, narrowedEvidence(7, NarrowedRecordJob, recordLog(other, prMerge, testHead, 4))},
	} {
		if d := decideBase(tc.event, tc.base, tc.e); d.Delta != nil || (d.Mode == "REUSE") != (name == "an exact reuse") {
			t.Errorf("%s: decision = %+v, delta = %+v", name, d, d.Delta)
		}
	}
	// An unadmitted run (another head, a failed run) names no delta either.
	failed := passedRun(7)
	failed.Conclusion = "failure"
	if d := Decide(testRepo, "merge_group", testTree, testHead, testTarget, testBase, 4, []run{failed}, narrowedEvidence(7, NarrowedRecordJob, moved).listJobs,
		narrowedEvidence(7, NarrowedRecordJob, moved).readLog); d.Delta != nil {
		t.Fatalf("failed run: delta = %+v", d.Delta)
	}
}

// queueNarrowedEvidence is a narrowed merge-queue run of the pushed commit whose only record job
// is ci-narrowed-tree-record (CIN-V0-005).
func queueNarrowedEvidence(runID int64, name, log string) evidence {
	e := evidence{jobs: map[int64][]job{}, logs: map[int64]string{}}
	e.jobs[runID] = append(shardJobs(runID, testTarget, 4), job{ID: runID * 10, RunID: runID, Name: name, HeadSHA: testTarget, Status: "completed", Conclusion: "success"})
	e.logs[runID*10] = log
	return e
}

// TestCINV0005PushReusesTheNarrowedQueueRunOfThePushedCommit (V1-3158): the main push of a
// commit whose merge-queue run passed narrowed reuses that run only when its narrowed record
// names this exact tree, target, base, head and shard count; each other case stays FULL.
func TestCINV0005PushReusesTheNarrowedQueueRunOfThePushedCommit(t *testing.T) {
	good := narrowedLog(testTree, testTarget, testBase, testHead, 4)
	push := func(base string, runs []run, e evidence) decision {
		return Decide(testRepo, "push", testTree, testHead, testTarget, base, 4, runs, e.listJobs, e.readLog)
	}
	d := push(testBase, []run{passedQueueRun(11)}, queueNarrowedEvidence(11, NarrowedRecordJob, good))
	if d.Mode != "REUSE" || d.Scope != "NARROWED" || d.Base != testBase || d.RunID == nil || *d.RunID != 11 || d.RecordJobID == nil ||
		*d.RecordJobID != 110 || len(d.Records) != 4 || !strings.Contains(d.Reason, "merge-queue run of this commit") {
		t.Fatalf("decision = %+v", d)
	}
	other := "3333333333333333333333333333333333333333"
	failed := passedQueueRun(11)
	failed.Conclusion = "failure"
	otherCommit := passedQueueRun(11)
	otherCommit.HeadSHA = other
	for name, tc := range map[string]struct {
		base string
		runs []run
		e    evidence
	}{
		// CORVINT_MQ_NARROWING off, or a tool without --base: only a full record is reused.
		"no base":                       {"", []run{passedQueueRun(11)}, queueNarrowedEvidence(11, NarrowedRecordJob, good)},
		"another base":                  {other, []run{passedQueueRun(11)}, queueNarrowedEvidence(11, NarrowedRecordJob, good)},
		"the tree differs":              {testBase, []run{passedQueueRun(11)}, queueNarrowedEvidence(11, NarrowedRecordJob, narrowedLog(other, testTarget, testBase, testHead, 4))},
		"another target":                {testBase, []run{passedQueueRun(11)}, queueNarrowedEvidence(11, NarrowedRecordJob, narrowedLog(testTree, other, testBase, testHead, 4))},
		"another head":                  {testBase, []run{passedQueueRun(11)}, queueNarrowedEvidence(11, NarrowedRecordJob, narrowedLog(testTree, testTarget, testBase, other, 4))},
		"another shard count":           {testBase, []run{passedQueueRun(11)}, queueNarrowedEvidence(11, NarrowedRecordJob, narrowedLog(testTree, testTarget, testBase, testHead, 8))},
		"two record lines":              {testBase, []run{passedQueueRun(11)}, queueNarrowedEvidence(11, NarrowedRecordJob, good+good)},
		"the queue ran FULL, no record": {testBase, []run{passedQueueRun(11)}, evidence{jobs: map[int64][]job{11: shardJobs(11, testTarget, 4)}, logs: map[int64]string{}}},
		"the record log is unreadable":  {testBase, []run{passedQueueRun(11)}, evidence{jobs: queueNarrowedEvidence(11, NarrowedRecordJob, good).jobs, logs: map[int64]string{}}},
		"the jobs are unreadable":       {testBase, []run{passedQueueRun(11)}, evidence{}},
		"the line in the full job":      {testBase, []run{passedQueueRun(11)}, queueNarrowedEvidence(11, RecordJob, good)},
		"a failed queue run":            {testBase, []run{failed}, queueNarrowedEvidence(11, NarrowedRecordJob, good)},
		"a queue run of another commit": {testBase, []run{otherCommit}, queueNarrowedEvidence(11, NarrowedRecordJob, good)},
		// A pull-request run's narrowed record never vouches for a main push (AFP-V0-734).
		"a pull-request narrowed record": {testBase, []run{passedRun(7)}, narrowedEvidence(7, NarrowedRecordJob, narrowedLog(testTree, prMerge, testBase, testHead, 4))},
	} {
		if d := push(tc.base, tc.runs, tc.e); d.Mode != "FULL" || d.Delta != nil {
			t.Errorf("%s: decision = %+v, want FULL", name, d)
		}
	}
	// A queue run whose full record also passed is reused as FULL.
	both := queueNarrowedEvidence(11, NarrowedRecordJob, good)
	both.jobs[11] = append(both.jobs[11], job{ID: 111, RunID: 11, Name: RecordJob, HeadSHA: testTarget, Status: "completed", Conclusion: "success"})
	both.logs[111] = recordLog(testTree, testTarget, testHead, 4)
	if d := push(testBase, []run{passedQueueRun(11)}, both); d.Mode != "REUSE" || d.Scope != "FULL" || *d.RecordJobID != 111 {
		t.Fatalf("decision = %+v, want the full record", d)
	}
}
