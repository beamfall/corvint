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
	var changes, atHead []string
	for _, path := range dirty {
		if committed[path] {
			atHead = append(atHead, path)
		}
		_, err := os.Lstat(filepath.Join(in.tree, filepath.FromSlash(path)))
		exists := err == nil
		// Indexed membership can change even when an added path has since
		// disappeared from the worktree. Observe that change independently of
		// missing tracked files, which also need the ratchet.
		if !baseOK || !currentOK || before[path] != current[path] || (current[path] && !exists) || (err != nil && !os.IsNotExist(err)) {
			changes = append(changes, path)
		}
	}
	return graph.WithOwnershipChanges(changes, atHead)
}
