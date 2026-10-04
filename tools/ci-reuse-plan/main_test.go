// SPDX-License-Identifier: AGPL-3.0-or-later
package main

import "testing"

const (
	testRepo = "beamfall/corvint"
	testTree = "1111111111111111111111111111111111111111"
	testHead = "2222222222222222222222222222222222222222"
)

func passedRun(id int64) run {
	r := run{ID: id, HeadSHA: testHead, Event: "pull_request", Status: "completed", Conclusion: "success", Path: workflowPath}
	r.Repository.FullName = testRepo
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

func listing(by map[int64][]artifact) func(int64) ([]artifact, bool) {
	return func(id int64) ([]artifact, bool) { a, ok := by[id]; return a, ok }
}

func TestAFPV0024ReusesOnlyAnExactTreeRecordedByEveryShard(t *testing.T) {
	d := Decide(testRepo, testTree, testHead, 4, []run{passedRun(7)}, listing(map[int64][]artifact{7: fullRecords(7, testTree, 4)}))
	if d.Mode != "REUSE" || d.RunID == nil || *d.RunID != 7 || len(d.Records) != 4 || d.Tree != testTree || d.Head != testHead || d.Profile != profile {
		t.Fatalf("decision = %+v", d)
	}
	// An earlier run of the same head against an older base recorded another tree; the
	// later run against the merged base is the one reused.
	other := "3333333333333333333333333333333333333333"
	d = Decide(testRepo, testTree, testHead, 4, []run{passedRun(5), passedRun(9)},
		listing(map[int64][]artifact{5: fullRecords(5, other, 4), 9: fullRecords(9, testTree, 4)}))
	if d.Mode != "REUSE" || *d.RunID != 9 {
		t.Fatalf("decision = %+v", d)
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
		"no runs":                   {nil, good, 4},
		"failed run":                {alter(func(r *run) { r.Conclusion = "failure" }), good, 4},
		"cancelled run":             {alter(func(r *run) { r.Conclusion = "cancelled" }), good, 4},
		"in progress":               {alter(func(r *run) { r.Status = "in_progress" }), good, 4},
		"push event":                {alter(func(r *run) { r.Event = "push" }), good, 4},
		"other workflow":            {alter(func(r *run) { r.Path = ".github/workflows/other.yml" }), good, 4},
		"run of another repository": {alter(func(r *run) { r.Repository.FullName = "other/corvint" }), good, 4},
		"other head":                {alter(func(r *run) { r.HeadSHA = testTree }), good, 4},
		"no run id":                 {alter(func(r *run) { r.ID = 0 }), good, 4},
		"artifacts unavailable":     {[]run{passedRun(7)}, nil, 4},
		"other tree":                {[]run{passedRun(7)}, map[int64][]artifact{7: fullRecords(7, testHead, 4)}, 4},
		"missing shard":             {[]run{passedRun(7)}, map[int64][]artifact{7: fullRecords(7, testTree, 4)[:3]}, 4},
		"other shard count":         {[]run{passedRun(7)}, map[int64][]artifact{7: fullRecords(7, testTree, 2)}, 4},
		"records of another run":    {[]run{passedRun(7)}, map[int64][]artifact{7: foreign}, 4},
		"records of another head":   {[]run{passedRun(7)}, map[int64][]artifact{7: wrongHead}, 4},
		"documentation-only run":    {[]run{passedRun(7)}, map[int64][]artifact{7: {{Name: "docs-ci-plan"}}}, 4},
	}
	for name, c := range cases {
		d := Decide(testRepo, testTree, testHead, c.shards, c.runs, listing(c.artifacts))
		if d.Mode != "FULL" || d.RunID != nil || len(d.Records) != 0 || d.Reason == "" {
			t.Errorf("%s: decision = %+v", name, d)
		}
	}
}
