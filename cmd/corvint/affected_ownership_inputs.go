// SPDX-License-Identifier: Apache-2.0
package main

import (
	"context"
	"maps"
	"os"
	"path/filepath"
	"strings"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// affectedOwnershipObservation retains the membership actually used by the
// planner, independently of dirty path names and immutable graph-cache inputs.
type affectedOwnershipObservation struct {
	paths map[string]bool
	ok    bool
}

func affectedOwnershipPaths(ctx context.Context, in affectedPlanInputs, args ...string) (map[string]bool, bool) {
	deadline, cancel := context.WithTimeout(ctx, affectedRevisionDeadline)
	defer cancel()
	out, err := hermeticGitCommand(deadline, in.gitExecutable, in.repository, args...).Output()
	if err != nil {
		return nil, false
	}
	paths := map[string]bool{}
	for _, path := range strings.Split(string(out), "\x00") {
		if path != "" {
			paths[path] = true
		}
	}
	return paths, true
}

func (observed *affectedOwnershipObservation) unchanged(ctx context.Context, in affectedPlanInputs) bool {
	if !observed.ok {
		return false
	}
	current, ok := affectedOwnershipPaths(ctx, in, "ls-files", "--cached", "-z")
	return observed.ok && ok && maps.Equal(observed.paths, current)
}

// Observe membership, not diff text: a rename changes both old and new names.
// Failed observations conservatively select the gate for every changed path.
func affectedOwnershipInputs(ctx context.Context, in affectedPlanInputs, graph *affected.Graph, dirty []string) *affected.Graph {
	base := affectedMeasuredCommit(ctx, in.gitExecutable, in.repository, in.base, in.revision)
	before, baseOK := affectedOwnershipPaths(ctx, in, "ls-tree", "-r", "-z", "--name-only", "--full-tree", base)
	committed, headOK := affectedOwnershipPaths(ctx, in, "ls-tree", "-r", "-z", "--name-only", "--full-tree", in.revision)
	current := committed
	currentOK := headOK
	if in.tree == in.repository {
		current, currentOK = affectedOwnershipPaths(ctx, in, "ls-files", "--cached", "-z")
		if in.ownership != nil {
			*in.ownership = affectedOwnershipObservation{paths: current, ok: currentOK}
		}
	}
	// Compare raw Git names before converting to the representation used by
	// DirtyPaths and RangePaths. Distinct raw names can share a display path.
	membershipChanges := map[string]bool{}
	for path := range before {
		if !current[path] {
			membershipChanges[path] = true
		}
	}
	for path := range current {
		if !before[path] {
			membershipChanges[path] = true
		}
	}
	dirtySet := map[string]bool{}
	for _, path := range dirty {
		dirtySet[path] = true
	}
	// Worktree checks must also use raw names: the display path may not exist.
	for path := range current {
		if !dirtySet[strings.ToValidUTF8(path, "\uFFFD")] {
			continue
		}
		if _, err := os.Lstat(filepath.Join(in.tree, filepath.FromSlash(path))); err != nil {
			membershipChanges[path] = true
		}
	}
	changed := map[string]bool{}
	for path := range membershipChanges {
		changed[strings.ToValidUTF8(path, "\uFFFD")] = true
	}
	var changes, atHead []string
	for path := range committed {
		if path = strings.ToValidUTF8(path, "\uFFFD"); dirtySet[path] {
			atHead = append(atHead, path)
		}
	}
	for _, path := range dirty {
		_, err := os.Lstat(filepath.Join(in.tree, filepath.FromSlash(path)))
		// Indexed membership can change even when an added path has since
		// disappeared from the worktree. Observe that change independently of
		// missing tracked files, which also need the ratchet.
		if !baseOK || !currentOK || changed[path] || (err != nil && !os.IsNotExist(err)) {
			changes = append(changes, path)
		}
	}
	return graph.WithOwnershipChanges(changes, atHead)
}
