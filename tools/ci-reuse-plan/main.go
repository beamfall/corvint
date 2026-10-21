// SPDX-License-Identifier: AGPL-3.0-or-later
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

type decision struct {
	Profile string `json:"profile"`
	Mode    string `json:"mode"`
	Reason  string `json:"reason"`
	Tree    string `json:"tree"`
	Head    string `json:"head"`
	Shards  int    `json:"shards"`
	RunID   *int64 `json:"runId"`
	// RecordJobID names the reused run's ci-tested-tree-record job whose log named the tree.
	RecordJobID *int64   `json:"recordJobId"`
	Records     []string `json:"records"`
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
	listed, ok := jobs(r.ID)
	if !ok {
		return 0
	}
	var record *job
	for i := range listed {
		if listed[i].Name != RecordJob {
			continue
		}
		if record != nil {
			return 0
		}
		record = &listed[i]
	}
	if record == nil || record.ID <= 0 || record.RunID != r.ID || record.HeadSHA != r.HeadSHA || record.Status != "completed" || record.Conclusion != "success" {
		return 0
	}
	text, ok := logs(record.ID)
	if !ok {
		return 0
	}
	var found []string
	for _, line := range strings.Split(strings.ReplaceAll(string(text), "\r", ""), "\n") {
		if m := recordLine.FindStringSubmatch(line); m != nil {
			if found != nil {
				return 0
			}
			found = m
		}
	}
	if found == nil || found[1] != tree || found[3] != head || found[4] != strconv.Itoa(shards) || (r.Event == "merge_group" && found[2] != r.HeadSHA) {
		return 0
	}
	return record.ID
}

// Decide returns REUSE only when one admitted run's record job vouched for tree over every
// shard; anything else is FULL. head echoes the tested commit's second parent for either
// event; the reused run is named by runId and its record job by recordJobId.
func Decide(repository, event, tree, head, target string, shards int, runs []run, jobs func(int64) ([]job, bool), logs func(int64) ([]byte, bool)) decision {
	d := decision{Profile: profile, Mode: "FULL", Tree: tree, Head: head, Shards: shards, Records: []string{}}
	admitted := 0
	for _, r := range runs {
		if !admits(r, repository, event, head, target) {
			continue
		}
		admitted++
		jobID := recorded(r, tree, head, shards, jobs, logs)
		if jobID == 0 {
			continue
		}
		records := make([]string, 0, shards)
		for shard := 0; shard < shards; shard++ {
			records = append(records, RecordName(tree, shard, shards))
		}
		id := r.ID
		d.Mode, d.RunID, d.RecordJobID, d.Records = "REUSE", &id, &jobID, records
		if r.Event == "merge_group" {
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
	var runsPaths pathList
	flag.Var(&runsPaths, "runs", "retained workflow-runs API response (repeatable)")
	jobsDir := flag.String("jobs", "", "directory of retained RUN_ID.json latest run-jobs API responses")
	logsDir := flag.String("logs", "", "directory of retained JOB_ID.txt job logs")
	flag.Parse()
	if flag.NArg() != 0 || *repository == "" || (*event != "push" && *event != "merge_group") || !oidPattern.MatchString(*tree) ||
		!oidPattern.MatchString(*head) || !oidPattern.MatchString(*target) || *shards < 1 || *shards > maxShards || len(runsPaths) == 0 || *jobsDir == "" || *logsDir == "" {
		fmt.Fprintln(os.Stderr, "usage: ci-reuse-plan --repository OWNER/NAME --event push|merge_group --tree TREE --head COMMIT --target COMMIT --shards N --runs FILE [--runs FILE] --jobs DIR --logs DIR")
		os.Exit(2)
	}
	var d decision
	if runs, ok := readRuns(runsPaths); !ok {
		d = decision{Profile: profile, Mode: "FULL", Reason: "workflow runs response unavailable", Tree: *tree, Head: *head, Shards: *shards, Records: []string{}}
	} else {
		d = Decide(*repository, *event, *tree, *head, *target, *shards, runs, func(id int64) ([]job, bool) {
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
