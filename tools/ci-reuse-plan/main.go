// SPDX-License-Identifier: AGPL-3.0-or-later
// ci-reuse-plan implements the repository-specific AFP-V0-024 main-push and AFP-V0-046
// merge-queue reuse decisions. It reads retained GitHub API responses and never contacts the
// network or runs a test.
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

type artifact struct {
	Name        string `json:"name"`
	WorkflowRun struct {
		ID      int64  `json:"id"`
		HeadSHA string `json:"head_sha"`
	} `json:"workflow_run"`
}

type decision struct {
	Profile string   `json:"profile"`
	Mode    string   `json:"mode"`
	Reason  string   `json:"reason"`
	Tree    string   `json:"tree"`
	Head    string   `json:"head"`
	Shards  int      `json:"shards"`
	RunID   *int64   `json:"runId"`
	Records []string `json:"records"`
}

// RecordName is the artifact name one full pull-request or merge-queue shard retains after
// its complete package set passed on the tested tree.
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

// Decide returns REUSE only when one admitted run retained a full-shard record naming tree
// for every shard; anything else is FULL. head echoes the tested commit's second parent for
// either event; the reused run is named by runId.
func Decide(repository, event, tree, head, target string, shards int, runs []run, artifacts func(int64) ([]artifact, bool)) decision {
	d := decision{Profile: profile, Mode: "FULL", Tree: tree, Head: head, Shards: shards, Records: []string{}}
	admitted := 0
	for _, r := range runs {
		if !admits(r, repository, event, head, target) {
			continue
		}
		admitted++
		listed, ok := artifacts(r.ID)
		if !ok {
			continue
		}
		have := map[string]bool{}
		for _, a := range listed {
			if a.WorkflowRun.ID == r.ID && a.WorkflowRun.HeadSHA == r.HeadSHA {
				have[a.Name] = true
			}
		}
		records := make([]string, 0, shards)
		for shard := 0; shard < shards; shard++ {
			if name := RecordName(tree, shard, shards); have[name] {
				records = append(records, name)
			}
		}
		if len(records) == shards {
			id := r.ID
			d.Mode, d.RunID, d.Records = "REUSE", &id, records
			if r.Event == "merge_group" {
				d.Reason = "a successful merge-queue run of this commit retained a full record of this exact tree for every shard"
			} else {
				d.Reason = "a successful pull-request run retained a full record of this exact tree for every shard"
			}
			return d
		}
	}
	if admitted == 0 {
		d.Reason = "no admitted successful run: a pull-request run of the merged head or, for a push, a merge-queue run of this commit"
	} else {
		d.Reason = "no admitted run retained a full record of this exact tree for every shard"
	}
	return d
}

func readBounded(path string, v any) bool {
	f, e := os.Open(path)
	if e != nil {
		return false
	}
	defer f.Close()
	b, e := io.ReadAll(io.LimitReader(f, maxInput+1))
	if e != nil || len(b) > maxInput {
		return false
	}
	return json.Unmarshal(b, v) == nil
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
	artifactsDir := flag.String("artifacts", "", "directory of retained RUN_ID.json run-artifacts API responses")
	flag.Parse()
	if flag.NArg() != 0 || *repository == "" || (*event != "push" && *event != "merge_group") || !oidPattern.MatchString(*tree) ||
		!oidPattern.MatchString(*head) || !oidPattern.MatchString(*target) || *shards < 1 || *shards > maxShards || len(runsPaths) == 0 || *artifactsDir == "" {
		fmt.Fprintln(os.Stderr, "usage: ci-reuse-plan --repository OWNER/NAME --event push|merge_group --tree TREE --head COMMIT --target COMMIT --shards N --runs FILE [--runs FILE] --artifacts DIR")
		os.Exit(2)
	}
	var d decision
	if runs, ok := readRuns(runsPaths); !ok {
		d = decision{Profile: profile, Mode: "FULL", Reason: "workflow runs response unavailable", Tree: *tree, Head: *head, Shards: *shards, Records: []string{}}
	} else {
		d = Decide(*repository, *event, *tree, *head, *target, *shards, runs, func(id int64) ([]artifact, bool) {
			var page struct {
				Artifacts []artifact `json:"artifacts"`
			}
			ok := readBounded(filepath.Join(*artifactsDir, strconv.FormatInt(id, 10)+".json"), &page)
			return page.Artifacts, ok
		})
	}
	if e := json.NewEncoder(os.Stdout).Encode(d); e != nil {
		os.Exit(1)
	}
}
