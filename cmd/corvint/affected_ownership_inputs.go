// SPDX-License-Identifier: Apache-2.0
package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// Observe membership, not diff text: a rename changes both old and new names.
// Failed observations conservatively select the gate for every changed path.
func affectedOwnershipInputs(ctx context.Context, in affectedPlanInputs, graph *affected.Graph, dirty []string) *affected.Graph {
	base := affectedMeasuredCommit(ctx, in.gitExecutable, in.repository, in.base, in.revision)
	list := func(args ...string) (map[string]bool, bool) {
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
	before, baseOK := list("ls-tree", "-r", "-z", "--name-only", "--full-tree", base)
	committed, headOK := list("ls-tree", "-r", "-z", "--name-only", "--full-tree", in.revision)
	current := committed
	currentOK := headOK
	if in.tree == in.repository {
		current, currentOK = list("ls-files", "--cached", "-z")
	}
	var changes, atHead []string
	for _, path := range dirty {
		if committed[path] {
			atHead = append(atHead, path)
		}
		_, err := os.Lstat(filepath.Join(in.tree, filepath.FromSlash(path)))
		exists := err == nil
		if !baseOK || !currentOK || before[path] != (current[path] && exists) || (err != nil && !os.IsNotExist(err)) {
			changes = append(changes, path)
		}
	}
	return graph.WithOwnershipChanges(changes, atHead)
}
