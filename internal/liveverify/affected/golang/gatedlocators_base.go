// SPDX-License-Identifier: Apache-2.0
package golang

import (
	"context"
	"fmt"

	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// GatedLocatorChangesAt compares the gated root-locator declaration of commit,
// read from verified immutable Git objects of the repository at root, with the
// declaration of the tree at tree (the worktree, or a snapshot's materialized
// tree), and returns the package directories whose entries were added, removed
// or changed (TSP-V0-608). An absent declaration on either side is an empty
// declaration. It returns nil, unknown, when the declaration is not among the
// changed paths, when either side is unreadable or invalid, or when Git fails:
// the caller then keeps TSP-V0-003's UNKNOWN. The head declaration is read
// with the same validity rules as the plugin's, whose own go:gated-root-
// locators-invalid frontier also keeps the plan UNKNOWN; the base
// declaration's package existence is not re-proven here, since a removed
// package's directory holds no head unit and the planner then keeps the edit
// unknown too.
func GatedLocatorChangesAt(ctx context.Context, root, tree, commit string, changed []string) []string {
	touched := false
	for _, path := range changed {
		touched = touched || path == affected.GatedRootLocatorsPath
	}
	if !touched {
		return nil
	}
	directories, err := gatedLocatorChanges(ctx, root, tree, commit)
	if err != nil {
		return nil
	}
	return directories
}

func gatedLocatorChanges(ctx context.Context, root, tree, commit string) ([]string, error) {
	head, err := readGatedLocators(affected.DiskSourceContext(ctx, tree))
	if err != nil {
		return nil, err
	}
	repository, err := gitauth.Open(root, gitrun.NewBudget(gitrun.DefaultOperations, gitrun.DefaultTotalBudget))
	if err != nil {
		return nil, err
	}
	defer repository.BeginObjectSession()()
	source, err := repository.RevisionFS(ctx, commit, affected.MaxSourceBytes)
	if err != nil {
		return nil, fmt.Errorf("%s at %s: %w", affected.GatedRootLocatorsPath, commit, err)
	}
	base, err := readGatedLocators(affected.FSSource(source))
	if err != nil {
		return nil, err
	}
	if base == nil {
		base = map[string]affected.GatedLocator{}
	}
	if head == nil {
		head = map[string]affected.GatedLocator{}
	}
	return affected.GatedLocatorChanges(base, head), nil
}
