// SPDX-License-Identifier: AGPL-3.0-or-later
package main

import (
	"os"
	"path/filepath"
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

func record(runID int64, tree string, shard, shards int) artifact {
	a := artifact{Name: RecordName(tree, shard, shards)}
	a.WorkflowRun.ID, a.WorkflowRun.HeadSHA = runID, testHead
	return a
}

func fullRecords(runID int64, tree string, shards int) []artifact {
	var out []artifact
	for shard := 0; shard < shards; shard++ {
		out = append(out, record(runID, tree, shard, shards))
	}
	return out
}

// queueRecords are full records retained by a merge-queue run, whose head is the queue commit.
func queueRecords(runID int64, tree string, shards int) []artifact {
	out := fullRecords(runID, tree, shards)
	for i := range out {
		out[i].WorkflowRun.HeadSHA = testTarget
	}
	return out
}

func listing(by map[int64][]artifact) func(int64) ([]artifact, bool) {
	return func(id int64) ([]artifact, bool) { a, ok := by[id]; return a, ok }
}

func TestAFPV0024ReusesOnlyAnExactTreeRecordedByEveryShard(t *testing.T) {
	for _, event := range []string{"push", "merge_group"} {
		d := Decide(testRepo, event, testTree, testHead, testTarget, 4, []run{passedRun(7)}, listing(map[int64][]artifact{7: fullRecords(7, testTree, 4)}))
		if d.Mode != "REUSE" || d.RunID == nil || *d.RunID != 7 || len(d.Records) != 4 || d.Tree != testTree || d.Head != testHead || d.Profile != profile {
			t.Fatalf("%s: decision = %+v", event, d)
		}
		// An earlier run of the same head against an older base recorded another tree; the
		// later run against the merged base is the one reused.
		other := "3333333333333333333333333333333333333333"
		d = Decide(testRepo, event, testTree, testHead, testTarget, 4, []run{passedRun(5), passedRun(9)},
			listing(map[int64][]artifact{5: fullRecords(5, other, 4), 9: fullRecords(9, testTree, 4)}))
		if d.Mode != "REUSE" || *d.RunID != 9 {
			t.Fatalf("%s: decision = %+v", event, d)
		}
	}
}

func TestAFPV0024AnythingElseRunsInFull(t *testing.T) {
	alter := func(f func(*run)) []run { r := passedRun(7); f(&r); return []run{r} }
	good := map[int64][]artifact{7: fullRecords(7, testTree, 4)}
	foreign := fullRecords(8, testTree, 4)
	wrongHead := fullRecords(7, testTree, 4)
	for i := range wrongHead {
		wrongHead[i].WorkflowRun.HeadSHA = testTree
	}
	cases := map[string]struct {
		runs      []run
		artifacts map[int64][]artifact
		shards    int
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
		"artifacts unavailable":      {[]run{passedRun(7)}, nil, 4},
		"other tree":                 {[]run{passedRun(7)}, map[int64][]artifact{7: fullRecords(7, testHead, 4)}, 4},
		"missing shard":              {[]run{passedRun(7)}, map[int64][]artifact{7: fullRecords(7, testTree, 4)[:3]}, 4},
		"other shard count":          {[]run{passedRun(7)}, map[int64][]artifact{7: fullRecords(7, testTree, 2)}, 4},
		"records of another run":     {[]run{passedRun(7)}, map[int64][]artifact{7: foreign}, 4},
		"records of another head":    {[]run{passedRun(7)}, map[int64][]artifact{7: wrongHead}, 4},
		"documentation-only run":     {[]run{passedRun(7)}, map[int64][]artifact{7: {{Name: "docs-ci-plan"}}}, 4},
	}
	for _, event := range []string{"push", "merge_group"} {
		for name, c := range cases {
			d := Decide(testRepo, event, testTree, testHead, testTarget, c.shards, c.runs, listing(c.artifacts))
			if d.Mode != "FULL" || d.RunID != nil || len(d.Records) != 0 || d.Reason == "" {
				t.Errorf("%s, %s: decision = %+v", event, name, d)
			}
		}
	}
}

// V1-0760: a main push reuses the merge-queue run of the pushed commit itself.
func TestAFPV0024PushReusesTheQueueRunOfThePushedCommit(t *testing.T) {
	d := Decide(testRepo, "push", testTree, testHead, testTarget, 4, []run{passedQueueRun(11)}, listing(map[int64][]artifact{11: queueRecords(11, testTree, 4)}))
	if d.Mode != "REUSE" || d.RunID == nil || *d.RunID != 11 || len(d.Records) != 4 || d.Head != testHead || d.Tree != testTree {
		t.Fatalf("decision = %+v", d)
	}
	// A pull-request run without matching records listed first does not hide the queue run.
	d = Decide(testRepo, "push", testTree, testHead, testTarget, 4, []run{passedRun(7), passedQueueRun(11)},
		listing(map[int64][]artifact{7: fullRecords(7, "3333333333333333333333333333333333333333", 4), 11: queueRecords(11, testTree, 4)}))
	if d.Mode != "REUSE" || *d.RunID != 11 {
		t.Fatalf("decision = %+v", d)
	}
}

func TestAFPV0024QueueRunRefusalsRunInFull(t *testing.T) {
	alter := func(f func(*run)) run { r := passedQueueRun(11); f(&r); return r }
	good := map[int64][]artifact{11: queueRecords(11, testTree, 4)}
	prHeadRecords := fullRecords(11, testTree, 4) // records naming the PR head, not the queue commit
	cases := map[string]struct {
		run       run
		artifacts map[int64][]artifact
	}{
		"failed":                   {alter(func(r *run) { r.Conclusion = "failure" }), good},
		"cancelled":                {alter(func(r *run) { r.Conclusion = "cancelled" }), good},
		"in progress":              {alter(func(r *run) { r.Status = "in_progress"; r.Conclusion = "" }), good},
		"other workflow path":      {alter(func(r *run) { r.Path = ".github/workflows/ci-control-plane.yml" }), good},
		"other repository":         {alter(func(r *run) { r.Repository.FullName = "fork/corvint" }), good},
		"wrong head":               {alter(func(r *run) { r.HeadSHA = testTree }), good},
		"merged pull-request head": {alter(func(r *run) { r.HeadSHA = testHead }), good},
		"partial records":          {passedQueueRun(11), map[int64][]artifact{11: queueRecords(11, testTree, 4)[1:]}},
		"different shard count":    {passedQueueRun(11), map[int64][]artifact{11: queueRecords(11, testTree, 8)}},
		"records of another head":  {passedQueueRun(11), map[int64][]artifact{11: prHeadRecords}},
		"other tree":               {passedQueueRun(11), map[int64][]artifact{11: queueRecords(11, testHead, 4)}},
		"artifacts unavailable":    {passedQueueRun(11), nil},
	}
	for name, c := range cases {
		d := Decide(testRepo, "push", testTree, testHead, testTarget, 4, []run{c.run}, listing(c.artifacts))
		if d.Mode != "FULL" || d.RunID != nil || len(d.Records) != 0 || d.Reason == "" {
			t.Errorf("%s: decision = %+v", name, d)
		}
	}
}

// AFP-V0-046: a merge-queue decision admits only a pull-request run of the queue commit's
// second parent; a merge-queue run, even of the queue commit itself, is never reused there.
func TestAFPV0046QueueDecisionAdmitsOnlyThePullRequestRun(t *testing.T) {
	d := Decide(testRepo, "merge_group", testTree, testHead, testTarget, 4, []run{passedQueueRun(11)}, listing(map[int64][]artifact{11: queueRecords(11, testTree, 4)}))
	if d.Mode != "FULL" || d.RunID != nil {
		t.Fatalf("queue run admitted on merge_group: %+v", d)
	}
	wrong := passedRun(7)
	wrong.HeadSHA = testTarget
	d = Decide(testRepo, "merge_group", testTree, testHead, testTarget, 4, []run{wrong}, listing(map[int64][]artifact{7: queueRecords(7, testTree, 4)}))
	if d.Mode != "FULL" || d.RunID != nil {
		t.Fatalf("pull-request run of the wrong head admitted: %+v", d)
	}
	for _, event := range []string{"", "pull_request", "release"} {
		if d := Decide(testRepo, event, testTree, testHead, testTarget, 4, []run{passedQueueRun(11)}, listing(map[int64][]artifact{11: queueRecords(11, testTree, 4)})); d.Mode != "FULL" {
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
