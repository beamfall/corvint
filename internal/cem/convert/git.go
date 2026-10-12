package convert

import (
	"context"

	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

// NewGitReader returns the SourceReader over the repository at root. It reads
// through the authenticated object reader with a fresh default budget per call;
// only a regular file (mode 100644 blob) at the sidecar path is present.
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
	if !present || entry.Type != "blob" || entry.Mode != "100644" {
		return "", false, nil
	}
	return entry.OID, true, nil
}
