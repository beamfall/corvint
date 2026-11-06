// SPDX-License-Identifier: Apache-2.0
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// CIN-V0-7600: run status is not tested-tree evidence; only the successful record
// and the latest required jobs bound to its run and head can establish reuse.
func TestCINV07600RecordAdmitsBeforeAdvisoryCompletion(t *testing.T) {
	for _, narrowed := range []bool{false, true} {
		for _, state := range []struct{ status, conclusion string }{{"in_progress", ""}, {"completed", "failure"}} {
			r := passedRun(7)
			r.Status, r.Conclusion = state.status, state.conclusion
			e := prEvidence(evidence{}, 7, testTree, 4)
			wantScope := "FULL"
			if narrowed {
				e = narrowedEvidence(7, NarrowedRecordJob, narrowedLog(testTree, prMerge, testBase, testHead, 4))
				wantScope = "NARROWED"
			}
			e.jobs[7] = append(e.jobs[7], job{ID: 79, RunID: 7, Name: "go-macos", HeadSHA: testHead, Status: state.status, Conclusion: state.conclusion})
			d := Decide(testRepo, "merge_group", testTree, testHead, testTarget, testBase, 4, []run{r}, e.listJobs, e.readLog)
			if d.Mode != "REUSE" || d.Scope != wantScope || d.RunID == nil || *d.RunID != 7 || d.RecordJobID == nil || *d.RecordJobID != 70 || len(d.Records) != 4 {
				t.Fatalf("narrowed=%v run=%s/%s: %+v", narrowed, state.status, state.conclusion, d)
			}
		}
	}
}

// Each refusal checks the precise diagnostic as well as FULL, so the former
// whole-run refusal cannot masquerade as validation of the record or required jobs.
func TestCINV07600RecordAdmissionFailsClosed(t *testing.T) {
	for _, narrowed := range []bool{false, true} {
		for _, tc := range []struct {
			name, want string
			edit       func(*run, *evidence)
		}{
			{"record missing", ": missing job", func(_ *run, e *evidence) { e.jobs[7] = e.jobs[7][:len(e.jobs[7])-1] }},
			{"record running", "requires completed success", func(_ *run, e *evidence) {
				j := e.jobs[7]
				j[len(j)-1].Status = "in_progress"
				j[len(j)-1].Conclusion = ""
			}},
			{"record failed", "requires completed success", func(_ *run, e *evidence) { j := e.jobs[7]; j[len(j)-1].Conclusion = "failure" }},
			{"required failed", "go-product-shard (0): requires completed success", func(_ *run, e *evidence) { e.jobs[7][0].Conclusion = "failure" }},
			{"required cancelled", "go-product-shard (0): requires completed success", func(_ *run, e *evidence) { e.jobs[7][0].Conclusion = "cancelled" }},
			{"required running", "go-product-shard (0): requires completed success", func(_ *run, e *evidence) { e.jobs[7][0].Status = "in_progress" }},
			{"required skipped", "go-product-shard (0): requires completed success", func(_ *run, e *evidence) { e.jobs[7][0].Conclusion = "skipped" }},
			{"required missing", "go-product-shard (0): missing job", func(_ *run, e *evidence) { e.jobs[7] = e.jobs[7][1:] }},
			{"run head mismatch", "run identity mismatch", func(r *run, _ *evidence) { r.HeadSHA = testTarget }},
			{"job head mismatch", "job id, run or head mismatch", func(_ *run, e *evidence) { e.jobs[7][0].HeadSHA = testTarget }},
			{"job run mismatch", "job id, run or head mismatch", func(_ *run, e *evidence) { e.jobs[7][0].RunID = 8 }},
			{"job id missing", "job id, run or head mismatch", func(_ *run, e *evidence) { e.jobs[7][0].ID = 0 }},
			{"duplicate required name", "ambiguous job name", func(_ *run, e *evidence) { e.jobs[7] = append(e.jobs[7], e.jobs[7][0]) }},
			{"duplicate required id", "ambiguous job id", func(_ *run, e *evidence) { e.jobs[7] = append(e.jobs[7], job{ID: e.jobs[7][0].ID, Name: "go-macos"}) }},
			{"duplicate record", "ambiguous job name", func(_ *run, e *evidence) { j := e.jobs[7]; e.jobs[7] = append(j, j[len(j)-1]) }},
			{"extra shard", "ambiguous shard job identity", func(_ *run, e *evidence) {
				j := e.jobs[7][0]
				j.ID = 799
				j.Name = "go-product-shard (4)"
				e.jobs[7] = append(e.jobs[7], j)
			}},
			{"noncanonical shard", "ambiguous shard job identity", func(_ *run, e *evidence) { e.jobs[7][0].Name = "go-product-shard (00)" }},
			{"tree mismatch", "record tree", func(_ *run, e *evidence) {
				e.logs[70] = strings.ReplaceAll(e.logs[70], "tree="+testTree, "tree="+testTarget)
			}},
		} {
			t.Run(tc.name+map[bool]string{false: "/full", true: "/narrowed"}[narrowed], func(t *testing.T) {
				r := passedRun(7)
				r.Status, r.Conclusion = "in_progress", ""
				e := prEvidence(evidence{}, 7, testTree, 4)
				if narrowed {
					e = narrowedEvidence(7, NarrowedRecordJob, narrowedLog(testTree, prMerge, testBase, testHead, 4))
				}
				tc.edit(&r, &e)
				d := Decide(testRepo, "merge_group", testTree, testHead, testTarget, testBase, 4, []run{r}, e.listJobs, e.readLog)
				if d.Mode != "FULL" || d.RunID != nil || d.RecordJobID != nil || len(d.Records) != 0 || (d.Delta != nil && tc.name != "tree mismatch") || !strings.Contains(d.Reason, tc.want) {
					t.Fatalf("%+v; want FULL with reason containing %q and no candidate", d, tc.want)
				}
			})
		}
	}
}

func TestCINV07600NarrowedRecordRequiresItsAggregateAndGuards(t *testing.T) {
	for _, name := range []string{"go-product", "docs-plan", "narrowing-verdict"} {
		e := narrowedEvidence(7, NarrowedRecordJob, narrowedLog(testTree, prMerge, testBase, testHead, 4))
		for i := range e.jobs[7] {
			if e.jobs[7][i].Name == name {
				e.jobs[7][i].Conclusion = "failure"
			}
		}
		d := Decide(testRepo, "merge_group", testTree, testHead, testTarget, testBase, 4, []run{passedRun(7)}, e.listJobs, e.readLog)
		if d.Mode != "FULL" || d.Delta != nil || !strings.Contains(d.Reason, name+": requires completed success") {
			t.Fatalf("%s: %+v", name, d)
		}
	}
}

// Retained API projections and the exact record line from PR #99, run 42106672783.
// go-macos failed; all jobs attested by the narrowed record passed.
func TestCINV07600ReplayPR99(t *testing.T) {
	const dir = "testdata/record-admission"
	runs, ok := readRuns([]string{filepath.Join(dir, "runs.json")})
	if !ok || len(runs) != 1 {
		t.Fatal("missing run fixture")
	}
	var page struct {
		Jobs []job `json:"jobs"`
	}
	b, err := os.ReadFile(filepath.Join(dir, "jobs/42106672783.json"))
	if err != nil || json.Unmarshal(b, &page) != nil {
		t.Fatal("missing jobs fixture", err)
	}
	b, err = os.ReadFile(filepath.Join(dir, "logs/127535388505.txt"))
	if err != nil {
		t.Fatal(err)
	}
	e := evidence{jobs: map[int64][]job{42106672783: page.Jobs}, logs: map[int64]string{127535388505: string(b)}}
	m := narrowedLine.FindStringSubmatch(strings.TrimSpace(string(b)))
	if m == nil {
		t.Fatal("invalid record fixture")
	}
	for _, status := range []string{"completed", "in_progress"} {
		r := runs[0]
		r.Status = status
		if status == "in_progress" {
			r.Conclusion = ""
		}
		d := Decide(testRepo, "merge_group", m[1], m[4], m[2], m[3], 16, []run{r}, e.listJobs, e.readLog)
		if d.Mode != "REUSE" || d.Scope != "NARROWED" || d.RecordJobID == nil || *d.RecordJobID != 127535388505 {
			t.Fatalf("%s: %+v", status, d)
		}
	}
}
