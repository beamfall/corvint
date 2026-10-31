package main

import (
	"context"

	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/gitancestry"
)

// impactRangeAncestryRefusal refuses a --base range impact in a checkout whose info/grafts can
// invent the base's ancestry, since the range engine's `merge-base --all` honours grafts
// (EEP-V0-770). A shallow boundary only removes parents, so the engine's exact check that the one
// merge base is --base itself stays correct there, and a shallow checkout is not refused here.
// The guard runs at the callers because any contextindex source change re-pins the analyzer
// schema (IDX-SNAP-V0-017).
func impactRangeAncestryRefusal(ctx context.Context, root string) error {
	reason := gitancestry.Rewritten(ctx, func(ctx context.Context, args ...string) ([]byte, error) {
		return sourceGit(ctx, root, 8192, args...)
	})
	if reason == "" || reason == gitancestry.Shallow {
		return nil
	}
	return &contextindex.Error{Code: "unsupported-impact-range", Message: "--base ancestry cannot be trusted: " + reason}
}
