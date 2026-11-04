// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// commitsOf answers a retained commits API response for each listed commit.
func commitsOf(m map[string]commit) func(string) (commit, bool) {
	return func(sha string) (commit, bool) { c, ok := m[sha]; return c, ok }
}

func mergeCommit(sha, first, second string) commit {
	c := commit{SHA: sha}
	for _, p := range []string{first, second} {
		c.Parents = append(c.Parents, struct {
			SHA string `json:"sha"`
		}{p})
	}
	return c
}

// TestAFPV0100NamesTheFullRecordOfAMovedBase: a merge-queue entry whose head's full pull-request
// record names another tree stays FULL and names that record as a FULL-scope delta whose base is
// the first parent of the record's target, read only from the retained commit response. Without
// a commits reader, for a push, without a base, for another head or shard count, for a commit
// response that does not bind the target to the head, and for an exact reuse, no delta is named.
func TestAFPV0100NamesTheFullRecordOfAMovedBase(t *testing.T) {
	other := "3333333333333333333333333333333333333333"
	recordBase := "6666666666666666666666666666666666666666"
	moved := prEvidence(evidence{}, 7, other, 4)
	good := commitsOf(map[string]commit{prMerge: mergeCommit(prMerge, recordBase, testHead)})
	decideWithCommits := func(event, base string, e evidence, commits func(string) (commit, bool)) decision {
		return decideWith(testRepo, event, testTree, testHead, testTarget, base, 4, []run{passedRun(7)}, e.listJobs, e.readLog, commits)
	}
	d := decideWithCommits("merge_group", testBase, moved, good)
	want := delta{RunID: 7, RecordJobID: 70, Tree: other, Target: prMerge, Base: recordBase, Head: testHead, Scope: "FULL"}
	if d.Mode != "FULL" || d.Delta == nil || *d.Delta != want {
		t.Fatalf("decision = %+v, delta = %+v", d, d.Delta)
	}
	if b, err := json.Marshal(d); err != nil || !strings.Contains(string(b), `"head":"`+testHead+`","scope":"FULL"}`) {
		t.Fatalf("encoded = %s, %v", b, err)
	}
	for name, tc := range map[string]struct {
		event, base string
		e           evidence
		commits     func(string) (commit, bool)
	}{
		"no commits reader":       {"merge_group", testBase, moved, nil},
		"a main push":             {"push", testBase, moved, good},
		"no entry base":           {"merge_group", "", moved, good},
		"another head":            {"merge_group", testBase, withRecord(evidence{}, 7, 70, testHead, recordLog(other, prMerge, other, 4)), good},
		"another shard count":     {"merge_group", testBase, prEvidence(evidence{}, 7, other, 8), good},
		"two record lines":        {"merge_group", testBase, withRecord(evidence{}, 7, 70, testHead, recordLog(other, prMerge, testHead, 4)+recordLog(other, prMerge, testHead, 4)), good},
		"an unread commit":        {"merge_group", testBase, moved, commitsOf(nil)},
		"another commit's answer": {"merge_group", testBase, moved, commitsOf(map[string]commit{prMerge: mergeCommit(other, recordBase, testHead)})},
		"a second parent not the head": {"merge_group", testBase, moved,
			commitsOf(map[string]commit{prMerge: mergeCommit(prMerge, recordBase, other)})},
		"a single-parent target": {"merge_group", testBase, moved, commitsOf(map[string]commit{prMerge: {SHA: prMerge, Parents: mergeCommit(prMerge, recordBase, testHead).Parents[:1]}})},
		"a malformed first parent": {"merge_group", testBase, moved,
			commitsOf(map[string]commit{prMerge: mergeCommit(prMerge, "HEAD", testHead)})},
		"an exact reuse": {"merge_group", testBase, prEvidence(evidence{}, 7, testTree, 4), good},
	} {
		if d := decideWithCommits(tc.event, tc.base, tc.e, tc.commits); d.Delta != nil || (d.Mode == "REUSE") != (name == "an exact reuse") {
			t.Errorf("%s: decision = %+v, delta = %+v", name, d, d.Delta)
		}
	}
	// An AFP-V0-812 narrowed candidate of an earlier run is preferred and keeps its shape.
	narrowed := narrowedEvidence(8, NarrowedRecordJob, narrowedLog(other, prMerge, other, testHead, 4))
	for id, jobs := range moved.jobs {
		narrowed.jobs[id] = jobs
	}
	for id, l := range moved.logs {
		narrowed.logs[id] = l
	}
	d = decideWith(testRepo, "merge_group", testTree, testHead, testTarget, testBase, 4, []run{passedRun(8), passedRun(7)}, narrowed.listJobs, narrowed.readLog, good)
	if d.Delta == nil || d.Delta.Scope != "" || d.Delta.RunID != 8 {
		t.Fatalf("narrowed and full: delta = %+v, want the narrowed run 8", d.Delta)
	}
}

// TestAFPV0100WithoutCommitsIsByteIdentical: the same evidence decided with no commits reader
// encodes exactly as Decide, the previous tool's entry point, so a docs-plan step that does not
// pass --commits sees no change.
func TestAFPV0100WithoutCommitsIsByteIdentical(t *testing.T) {
	other := "3333333333333333333333333333333333333333"
	for _, e := range []evidence{prEvidence(evidence{}, 7, other, 4), prEvidence(evidence{}, 7, testTree, 4),
		narrowedEvidence(7, NarrowedRecordJob, narrowedLog(other, prMerge, other, testHead, 4))} {
		for _, event := range []string{"push", "merge_group"} {
			a, _ := json.Marshal(Decide(testRepo, event, testTree, testHead, testTarget, testBase, 4, []run{passedRun(7)}, e.listJobs, e.readLog))
			b, _ := json.Marshal(decideWith(testRepo, event, testTree, testHead, testTarget, testBase, 4, []run{passedRun(7)}, e.listJobs, e.readLog, nil))
			if !bytes.Equal(a, b) {
				t.Fatalf("%s: %s != %s", event, a, b)
			}
		}
	}
}
