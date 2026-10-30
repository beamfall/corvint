package gitauth

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

// IsAncestor reports whether commit ancestor is reachable from commit descendant
// through parent links; a commit is its own ancestor. Both must be full object
// IDs of commits present in the local object store. A shallow repository's
// truncated history cannot prove that a commit is not an ancestor, so it is
// refused as repository-object-unavailable rather than answered.
func (r *Repository) IsAncestor(ctx context.Context, ancestor, descendant string) (bool, error) {
	if !wire.IsGitOid(ancestor) || !wire.IsGitOid(descendant) {
		return false, cemcode.New(cemcode.InvalidArguments, "ancestry revisions must be full lowercase Git OIDs")
	}
	if _, err := os.Lstat(filepath.Join(r.CommonDir, "shallow")); err == nil {
		return false, unavailable("shallow repository history is truncated; commit ancestry is not observable")
	} else if !errors.Is(err, fs.ErrNotExist) {
		return false, unavailable("shallow repository marker is unreadable")
	}
	for _, oid := range []string{ancestor, descendant} {
		if _, err := r.Resolve(ctx, oid); err != nil {
			return false, err
		}
	}
	if _, err := r.git(ctx, 64, "merge-base", "--is-ancestor", ancestor, descendant); err != nil {
		if details, ok := cemcode.GitExitFailureDetails(err); ok && details.ExitCode == 1 {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
