// SPDX-License-Identifier: Apache-2.0

// Package docworktree reports whether the worktree differs from the Git content a docs check read
// (DMF-V1-005). A check keeps answering only about committed or indexed content; this package
// observes the difference and never reads worktree bytes as evidence.
package docworktree

import (
	"bytes"
	"context"
	"slices"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/gokernel"
)

// States of the worktree member.
const (
	Clean   = "clean"
	Dirty   = "dirty"
	Unknown = "unknown"
)

// Bases a check reads from.
const (
	// Head is a check that reads committed content at HEAD: a staged or unstaged change differs.
	Head = "HEAD"
	// Index is a check that reads the Git index: only an unstaged change differs.
	Index = "index"
)

// MaxPaths bounds the paths a dirty report names.
const MaxPaths = 5

// Status is the `worktree` member of every docs check report (DMF-V1-005): `clean`, or `dirty`
// with the count of changed tracked paths the check read and the first of them, sorted; `unknown`
// with a reason when Git cannot say.
type Status struct {
	State   string   `json:"state"`
	Changed int      `json:"changed,omitempty"`
	Paths   []string `json:"paths,omitempty"`
	Reason  string   `json:"reason,omitempty"`
}

// Observe compares the worktree with base (Head or Index) for the tracked paths read accepts; a nil
// read accepts every tracked path. Untracked files are not read by any check, so they never count.
func Observe(ctx context.Context, root, base string, read func(path string) bool) Status {
	raw, err := gitrun.Run(ctx, gitrun.NewBudget(1, 30*time.Second),
		gitrun.Options{Dir: root, Env: gokernel.SanitizedGitEnvironment(), StdoutLimit: 16 << 20},
		"--no-optional-locks", "-c", "core.fsmonitor=false", "-c", "core.quotePath=false",
		"status", "--porcelain=v1", "-z", "--untracked-files=no", "--ignore-submodules=all")
	if err != nil {
		return Status{State: Unknown, Reason: "git status failed: " + err.Error()}
	}
	return parse(raw, base, read)
}

// parse reads `git status --porcelain=v1 -z` records: XY, a space and the path, with a rename's or
// copy's source path as the next record.
func parse(raw []byte, base string, read func(path string) bool) Status {
	changed := []string{}
	records := bytes.Split(bytes.TrimSuffix(raw, []byte{0}), []byte{0})
	for i := 0; i < len(records); i++ {
		record := string(records[i])
		if len(record) < 4 {
			continue
		}
		x, y, path := record[0], record[1], record[3:]
		if x == 'R' || x == 'C' {
			i++ // the source path record
		}
		differs := y != ' '
		if base == Head {
			differs = differs || x != ' '
		}
		if differs && (read == nil || read(path)) {
			changed = append(changed, path)
		}
	}
	if len(changed) == 0 {
		return Status{State: Clean}
	}
	slices.Sort(changed)
	changed = slices.Compact(changed)
	return Status{State: Dirty, Changed: len(changed), Paths: changed[:min(len(changed), MaxPaths)]}
}

// Set is a read predicate over an explicit list of paths.
func Set(paths ...string) func(string) bool {
	set := map[string]bool{}
	for _, p := range paths {
		set[p] = true
	}
	return func(p string) bool { return set[p] }
}

// Under is a read predicate over every path under one of the directories, or equal to one of the
// files, given.
func Under(entries ...string) func(string) bool {
	return func(p string) bool {
		for _, e := range entries {
			e = strings.TrimSuffix(e, "/")
			if p == e || strings.HasPrefix(p, e+"/") {
				return true
			}
		}
		return false
	}
}
