// SPDX-License-Identifier: Apache-2.0
package main

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/Beamfall/corvint/internal/testinventory"
)

// readInventoryVerdict judges the committed test inventory for a read
// command (TIV-V1-002, TIV-V1-006 to TIV-V1-009): against HEAD's tree, or
// the working tree when worktree is set. It runs no lister and writes
// nothing; a tree it cannot read judges the inventory invalid, never fresh.
func readInventoryVerdict(ctx context.Context, root string, worktree bool) testinventory.Verdict {
	invalid := func(err error) testinventory.Verdict {
		return testinventory.Verdict{Profile: testinventory.VerdictProfile, Path: testinventory.Path, State: testinventory.StateInvalid,
			Reason: testinventory.ReasonInvalid, Detail: "cannot read the planned tree: " + err.Error(), Instances: []testinventory.InstanceVerdict{}}
	}
	executable, err := exec.LookPath("git")
	if err != nil {
		return invalid(err)
	}
	git := testinventory.Git{Executable: executable, Root: root}
	var tree testinventory.Tree
	if worktree {
		tree, err = testinventory.NewWorkTree(ctx, git)
	} else {
		var head string
		if head, err = git.Head(ctx); err == nil {
			tree, err = testinventory.NewCommitTree(ctx, git, head)
		}
	}
	if err != nil {
		return invalid(err)
	}
	return testinventory.Judge(tree)
}

// inventoryVerdictLine is the verdict in one bounded line: the file state,
// every reason it prints, how many declared instances run in full and how
// many test files are unassigned.
func inventoryVerdictLine(verdict testinventory.Verdict) string {
	full, unassigned := 0, 0
	for _, instance := range verdict.Instances {
		if instance.Full {
			full++
		}
		unassigned += len(instance.Unassigned)
	}
	reasons := strings.Join(verdict.Reasons(), ",")
	if reasons == "" {
		reasons = "none"
	}
	return fmt.Sprintf("%s reasons=%s full=%d/%d unassigned=%d", verdict.State, reasons, full, len(verdict.Instances), unassigned)
}
