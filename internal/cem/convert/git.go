package convert

import (
	"context"

	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

// NewGitReader returns the SourceReader over the repository at root. It reads
// through the authenticated object reader with a fresh default budget per call;
// only a regular file (a blob in mode 100644 or 100755) at the sidecar path is present.
func NewGitReader(ctx context.Context, root string) SourceReader {
	return &gitReader{ctx: ctx, root: root}
}

type gitReader struct {
	ctx  context.Context
	root string
}

func (g *gitReader) SidecarBlob(commit string) (string, bool, error) {
	repo, err := gitauth.Open(g.root, gitrun.NewDefaultBudget())
	if err != nil {
		return "", false, err
	}
	entry, present, err := repo.LookupTreeEntry(g.ctx, commit, wire.ExcludedCEMPath)
	if err != nil {
		return "", false, err
	}
	// A regular file is a blob in either file mode; a symlink (120000) or a
	// submodule (160000) is no sidecar.
	if !present || entry.Type != "blob" || (entry.Mode != "100644" && entry.Mode != "100755") {
		return "", false, nil
	}
	return entry.OID, true, nil
}
