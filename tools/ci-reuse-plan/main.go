// SPDX-License-Identifier: Apache-2.0
// ci-reuse-plan implements the repository-specific AFP-V0-024 main-push and AFP-V0-046
// merge-queue reuse decisions. It reads retained GitHub API responses and never contacts the
// network or runs a test. The tested tree is taken only from the log of the reused run's
// ci-tested-tree-record job (AFP-V0-170), which runs no repository code; artifact names, which
// any job running repository code can choose, are never read.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

const profile = "corvint-ci-reuse/0"
const workflowPath = ".github/workflows/ci.yml"
const maxInput = 4 << 20
const maxShards = 64

var oidPattern = regexp.MustCompile(`^[0-9a-f]{40}$`)

type run struct {
	ID         int64  `json:"id"`
	HeadSHA    string `json:"head_sha"`
	Event      string `json:"event"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
	Path       string `json:"path"`
	Repository struct {
		FullName string `json:"full_name"`
	} `json:"repository"`
}

// job is one entry of a run's latest-attempt jobs listing.
type job struct {
	ID         int64  `json:"id"`
	RunID      int64  `json:"run_id"`
	Name       string `json:"name"`
	HeadSHA    string `json:"head_sha"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
}

// RecordJob names the ci.yml job that, after every go-product shard passed in full, prints the
// tested tree to its own log (AFP-V0-170).
const RecordJob = "ci-tested-tree-record"

// recordLine is that job's one record line, behind the runner timestamp a job log adds.
var recordLine = regexp.MustCompile(`^(?:[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9:.]+Z )?corvint-ci-tested-tree/1 tree=([0-9a-f]{40}) target=([0-9a-f]{40}) head=([0-9a-f]{40}) shards=([0-9]+)$`)

// NarrowedRecordJob names the ci.yml job that, after go-product passed a pull-request run whose
// shards ran one proven narrowed selection, prints the tested tree and the selection's base
// (AFP-V0-734). Only a merge-queue entry of that base reads it.
const NarrowedRecordJob = "ci-narrowed-tree-record"

// narrowedLine is that job's one record line. Its submatches are ordered as recordLine's, with
// the base last.
var narrowedLine = regexp.MustCompile(`^(?:[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9:.]+Z )?corvint-ci-tested-tree/2 scope=NARROWED tree=([0-9a-f]{40}) target=([0-9a-f]{40}) base=([0-9a-f]{40}) head=([0-9a-f]{40}) shards=([0-9]+)$`)

type decision struct {
	Profile string `json:"profile"`
	Mode    string `json:"mode"`
	Reason  string `json:"reason"`
	Tree    string `json:"tree"`
	Head    string `json:"head"`
	Shards  int    `json:"shards"`
	// Scope is FULL for an AFP-V0-170 record and NARROWED for an AFP-V0-734 one; Base is the
	// merge-queue entry's first parent, which a NARROWED record must name.
	Scope string `json:"scope,omitempty"`
	Base  string `json:"base,omitempty"`
	RunID *int64 `json:"runId"`
	// RecordJobID names the reused run's ci-tested-tree-record job whose log named the tree.
	RecordJobID *int64   `json:"recordJobId"`
	Records     []string `json:"records"`
	// Delta is set only on a merge-queue FULL decision made with a base: the first admitted
	// pull-request run of head whose one narrowed-record line names head and the shard count but
	// not this tree and base (AFP-V0-812). It vouches for nothing here; mq-narrow.sh verifies it
	// against the tested checkout before reusing any package of the record (AFP-V0-813 to 816).
	Delta *delta `json:"delta,omitempty"`
}

// delta is one admitted narrowed pull-request record of the same head: its run and record job,
// and the tree, target, base and head its record line named.
type delta struct {
	RunID       int64  `json:"runId"`
	RecordJobID int64  `json:"recordJobId"`
	Tree        string `json:"tree"`
	Target      string `json:"target"`
	Base        string `json:"base"`
	Head        string `json:"head"`
}

// RecordName names one shard of the full pull-request or merge-queue run whose record job
// vouched for the tested tree. Since AFP-V0-170 it is a decision field only: no artifact of
// that name is uploaded or read.
func RecordName(tree string, shard, shards int) string {
	return fmt.Sprintf("ci-tested-tree-%s-full-%d-of-%d", tree, shard, shards)
}

// admits reports whether r is a completed, successful run of this repository's CI workflow
// that the deciding event may reuse: a pull-request run of head, the tested commit's second
// parent (AFP-V0-024, AFP-V0-046), or, for a main push only, a merge-queue run of target, the
// pushed commit itself (AFP-V0-024, V1-0760).
func admits(r run, repository, event, head, target string) bool {
	if r.ID <= 0 || r.Status != "completed" || r.Conclusion != "success" || r.Path != workflowPath || r.Repository.FullName != repository {
		return false
	}
	switch r.Event {
	case "pull_request":
		return r.HeadSHA == head
	case "merge_group":
		return event == "push" && r.HeadSHA == target
	}
	return false
}

// recorded returns the id of r's one latest ci-tested-tree-record job when that job passed on
// r's head and its log holds exactly one record line naming tree, head and shards, and, for a
// merge-queue run, r's own commit as the target (AFP-V0-170). Anything else is 0.
func recorded(r run, tree, head string, shards int, jobs func(int64) ([]job, bool), logs func(int64) ([]byte, bool)) int64 {
	id, found := recordJobLine(r, RecordJob, recordLine, jobs, logs)
	if found == nil || found[1] != tree || found[3] != head || found[4] != strconv.Itoa(shards) || (r.Event == "merge_group" && found[2] != r.HeadSHA) {
		return 0
	}
	return id
}

// recordedNarrowed returns the id of pull-request run r's one latest ci-narrowed-tree-record job
// when its log holds exactly one AFP-V0-734 line naming tree, base, head and shards. Anything
// else, and any run but a pull-request run, is 0.
func recordedNarrowed(r run, tree, base, head string, shards int, jobs func(int64) ([]job, bool), logs func(int64) ([]byte, bool)) int64 {
	if !oidPattern.MatchString(base) {
		return 0
	}
	id, found := narrowedRecordOf(r, head, shards, jobs, logs)
	if found == nil || found[1] != tree || found[3] != base {
		return 0
	}
	return id
}

// narrowedRecordOf returns pull-request run r's one latest ci-narrowed-tree-record job and the
// submatches of its one AFP-V0-734 line when that line names head and shards, whatever its tree
// and base; anything else, and any run but a pull-request run, is 0 and nil.
func narrowedRecordOf(r run, head string, shards int, jobs func(int64) ([]job, bool), logs func(int64) ([]byte, bool)) (int64, []string) {
	if r.Event != "pull_request" {
		return 0, nil
	}
	id, found := recordJobLine(r, NarrowedRecordJob, narrowedLine, jobs, logs)
	if found == nil || found[4] != head || found[5] != strconv.Itoa(shards) {
		return 0, nil
	}
	return id, found
}

// recordJobLine returns r's one latest job named name, when it passed on r's head, and the
// submatches of the one line of its log that line matches; anything else is 0 and nil.
func recordJobLine(r run, name string, line *regexp.Regexp, jobs func(int64) ([]job, bool), logs func(int64) ([]byte, bool)) (int64, []string) {
	listed, ok := jobs(r.ID)
	if !ok {
		return 0, nil
	}
	var record *job
	for i := range listed {
		if listed[i].Name != name {
			continue
		}
		if record != nil {
			return 0, nil
		}
		record = &listed[i]
	}
	if record == nil || record.ID <= 0 || record.RunID != r.ID || record.HeadSHA != r.HeadSHA || record.Status != "completed" || record.Conclusion != "success" {
		return 0, nil
	}
	text, ok := logs(record.ID)
	if !ok {
		return 0, nil
	}
	var found []string
	for _, l := range strings.Split(strings.ReplaceAll(string(text), "\r", ""), "\n") {
		if m := line.FindStringSubmatch(l); m != nil {
			if found != nil {
				return 0, nil
			}
			found = m
		}
	}
	return record.ID, found
}

// Decide returns REUSE only when one admitted run's record job vouched for tree over every
// shard; anything else is FULL. head echoes the tested commit's second parent for either
// event; the reused run is named by runId and its record job by recordJobId. For a merge-queue
// entry whose first parent is base, a pull-request run's AFP-V0-734 narrowed record of the same
// tree, head and base also vouches (scope NARROWED); a full record of the same run is preferred.
// A main push never reuses a narrowed record. A merge-queue FULL decision made with a base also
// names, as Delta, the first admitted narrowed record of head and the shard count (AFP-V0-812).
func Decide(repository, event, tree, head, target, base string, shards int, runs []run, jobs func(int64) ([]job, bool), logs func(int64) ([]byte, bool)) decision {
	d := decision{Profile: profile, Mode: "FULL", Tree: tree, Head: head, Shards: shards, Records: []string{}}
	admitted := 0
	var candidate *delta
	for _, r := range runs {
		if !admits(r, repository, event, head, target) {
			continue
		}
		admitted++
		if candidate == nil && event == "merge_group" && oidPattern.MatchString(base) {
			if id, found := narrowedRecordOf(r, head, shards, jobs, logs); found != nil {
				candidate = &delta{RunID: r.ID, RecordJobID: id, Tree: found[1], Target: found[2], Base: found[3], Head: found[4]}
			}
		}
		scope := "FULL"
		jobID := recorded(r, tree, head, shards, jobs, logs)
		if jobID == 0 && event == "merge_group" {
			scope, jobID = "NARROWED", recordedNarrowed(r, tree, base, head, shards, jobs, logs)
		}
		if jobID == 0 {
			continue
		}
		records := make([]string, 0, shards)
		for shard := 0; shard < shards; shard++ {
			records = append(records, RecordName(tree, shard, shards))
		}
		id := r.ID
		d.Mode, d.Scope, d.RunID, d.RecordJobID, d.Records = "REUSE", scope, &id, &jobID, records
		if scope == "NARROWED" {
			d.Base = base
			d.Reason = "the narrowed-record job of a successful pull-request run named this exact tree and this entry's base after its shards passed one proven narrowed selection of the same range"
		} else if r.Event == "merge_group" {
			d.Reason = "the record job of a successful merge-queue run of this commit named this exact tree after every shard passed in full"
		} else {
			d.Reason = "the record job of a successful pull-request run named this exact tree after every shard passed in full"
		}
		return d
	}
	if admitted == 0 {
		d.Reason = "no admitted successful run: a pull-request run of the merged head or, for a push, a merge-queue run of this commit"
	} else {
		d.Reason = "no admitted run's record job named this exact tree for every shard"
	}
	d.Delta = candidate
	return d
}

func readFile(path string) ([]byte, bool) {
	f, e := os.Open(path)
	if e != nil {
		return nil, false
	}
	defer f.Close()
	b, e := io.ReadAll(io.LimitReader(f, maxInput+1))
	if e != nil || len(b) > maxInput {
		return nil, false
	}
	return b, true
}

func readBounded(path string, v any) bool {
	b, ok := readFile(path)
	return ok && json.Unmarshal(b, v) == nil
}

// readRuns concatenates the runs of every readable workflow-runs listing in order. An
// unreadable listing contributes no run; ok is false only when none was readable.
func readRuns(paths []string) (runs []run, ok bool) {
	for _, path := range paths {
		var listed struct {
			Runs []run `json:"workflow_runs"`
		}
		if readBounded(path, &listed) {
			runs, ok = append(runs, listed.Runs...), true
		}
	}
	return runs, ok
}

type pathList []string

func (p *pathList) String() string { return fmt.Sprint(*p) }

func (p *pathList) Set(v string) error {
	if v == "" {
		return fmt.Errorf("empty path")
	}
	*p = append(*p, v)
	return nil
}

func main() {
	repository := flag.String("repository", "", "OWNER/NAME of this repository")
	event := flag.String("event", "", "event of the deciding run: push or merge_group")
	tree := flag.String("tree", "", "full tree id of the tested commit")
	head := flag.String("head", "", "full commit id of the tested commit's second parent, the merged pull-request head")
	target := flag.String("target", "", "full commit id of the tested commit (the pushed or queue commit)")
	shards := flag.Int("shards", 0, "shard count of the full run")
	base := flag.String("base", "", "merge_group only: full commit id of the queue commit's first parent, the queue base a narrowed pull-request record must name (AFP-V0-734)")
	var runsPaths pathList
	flag.Var(&runsPaths, "runs", "retained workflow-runs API response (repeatable)")
	jobsDir := flag.String("jobs", "", "directory of retained RUN_ID.json latest run-jobs API responses")
	logsDir := flag.String("logs", "", "directory of retained JOB_ID.txt job logs")
	flag.Parse()
	if flag.NArg() != 0 || *repository == "" || (*event != "push" && *event != "merge_group") || !oidPattern.MatchString(*tree) ||
		!oidPattern.MatchString(*head) || !oidPattern.MatchString(*target) || *shards < 1 || *shards > maxShards || len(runsPaths) == 0 || *jobsDir == "" || *logsDir == "" ||
		(*base != "" && (*event != "merge_group" || !oidPattern.MatchString(*base))) {
		fmt.Fprintln(os.Stderr, "usage: ci-reuse-plan --repository OWNER/NAME --event push|merge_group --tree TREE --head COMMIT --target COMMIT [--base COMMIT] --shards N --runs FILE [--runs FILE] --jobs DIR --logs DIR")
		os.Exit(2)
	}
	var d decision
	if runs, ok := readRuns(runsPaths); !ok {
		d = decision{Profile: profile, Mode: "FULL", Reason: "workflow runs response unavailable", Tree: *tree, Head: *head, Shards: *shards, Records: []string{}}
	} else {
		d = Decide(*repository, *event, *tree, *head, *target, *base, *shards, runs, func(id int64) ([]job, bool) {
			var page struct {
				Jobs []job `json:"jobs"`
			}
			ok := readBounded(filepath.Join(*jobsDir, strconv.FormatInt(id, 10)+".json"), &page)
			return page.Jobs, ok
		}, func(id int64) ([]byte, bool) {
			return readFile(filepath.Join(*logsDir, strconv.FormatInt(id, 10)+".txt"))
		})
	}
	if e := json.NewEncoder(os.Stdout).Encode(d); e != nil {
		os.Exit(1)
	}
}
