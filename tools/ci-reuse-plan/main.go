// SPDX-License-Identifier: AGPL-3.0-or-later
// ci-reuse-plan implements the repository-specific AFP-V0-024 main-push reuse decision. It
// reads retained GitHub API responses and never contacts the network or runs a test.
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

// RecordName is the artifact name one full pull-request shard retains after its
// complete package set passed on the tested tree.
func RecordName(tree string, shard, shards int) string {
	return fmt.Sprintf("ci-tested-tree-%s-full-%d-of-%d", tree, shard, shards)
}

// admits reports whether r is a completed, successful pull-request run of this
// repository's CI workflow for head.
func admits(r run, repository, head string) bool {
	return r.ID > 0 && r.Event == "pull_request" && r.Status == "completed" && r.Conclusion == "success" &&
		r.Path == workflowPath && r.Repository.FullName == repository && r.HeadSHA == head
}

// Decide returns REUSE only when one admitted run retained a full-shard record
// naming tree for every shard; anything else is FULL.
func Decide(repository, tree, head string, shards int, runs []run, artifacts func(int64) ([]artifact, bool)) decision {
	d := decision{Profile: profile, Mode: "FULL", Tree: tree, Head: head, Shards: shards, Records: []string{}}
	admitted := 0
	for _, r := range runs {
		if !admits(r, repository, head) {
			continue
		}
		admitted++
		listed, ok := artifacts(r.ID)
		if !ok {
			continue
		}
		have := map[string]bool{}
		for _, a := range listed {
			if a.WorkflowRun.ID == r.ID && a.WorkflowRun.HeadSHA == head {
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
			d.Reason = "a successful pull-request run retained a full record of this exact tree for every shard"
			return d
		}
	}
	if admitted == 0 {
		d.Reason = "no successful pull-request run of this workflow for the merged head"
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

func main() {
	repository := flag.String("repository", "", "OWNER/NAME of this repository")
	tree := flag.String("tree", "", "full tree id of the pushed commit")
	head := flag.String("head", "", "full commit id of the merged pull-request head")
	shards := flag.Int("shards", 0, "shard count of the full run")
	runsPath := flag.String("runs", "", "retained workflow-runs API response")
	artifactsDir := flag.String("artifacts", "", "directory of retained RUN_ID.json run-artifacts API responses")
	flag.Parse()
	if flag.NArg() != 0 || *repository == "" || !oidPattern.MatchString(*tree) || !oidPattern.MatchString(*head) ||
		*shards < 1 || *shards > maxShards || *runsPath == "" || *artifactsDir == "" {
		fmt.Fprintln(os.Stderr, "usage: ci-reuse-plan --repository OWNER/NAME --tree TREE --head COMMIT --shards N --runs FILE --artifacts DIR")
		os.Exit(2)
	}
	var listed struct {
		Runs []run `json:"workflow_runs"`
	}
	var d decision
	if !readBounded(*runsPath, &listed) {
		d = decision{Profile: profile, Mode: "FULL", Reason: "workflow runs response unavailable", Tree: *tree, Head: *head, Shards: *shards, Records: []string{}}
	} else {
		d = Decide(*repository, *tree, *head, *shards, listed.Runs, func(id int64) ([]artifact, bool) {
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
