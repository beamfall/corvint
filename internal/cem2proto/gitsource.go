package cem2proto

import (
	"context"

	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
)

// GitSource reads a projection's facts through the authenticated Git layer
// the native verifier uses. Open one per map: it carries one default budget.
type GitSource struct{ repository *gitauth.Repository }

// NewGitSource opens root with a fresh default budget.
func NewGitSource(root string) (*GitSource, error) {
	repository, err := gitauth.Open(root, gitrun.NewDefaultBudget())
	if err != nil {
		return nil, err
	}
	return &GitSource{repository: repository}, nil
}

// Patch returns the canonical patch from base to the resolved target.
func (g *GitSource) Patch(ctx context.Context, base, target string) ([]byte, error) {
	resolved, err := g.repository.Resolve(ctx, target)
	if err != nil {
		return nil, err
	}
	return g.repository.CanonicalDiff(ctx, base, resolved)
}

// Blobs resolves paths at base and reads each distinct blob once.
func (g *GitSource) Blobs(ctx context.Context, base string, paths []string) (map[string]Blob, error) {
	entries, err := g.repository.ResolvePaths(ctx, base, paths)
	if err != nil {
		return nil, err
	}
	data := map[string][]byte{}
	out := make(map[string]Blob, len(entries))
	for path, entry := range entries {
		if entry.Type != "blob" {
			continue
		}
		bytes, ok := data[entry.OID]
		if !ok {
			if bytes, err = g.repository.BlobBytes(ctx, entry.OID); err != nil {
				return nil, err
			}
			data[entry.OID] = bytes
		}
		out[path] = Blob{Mode: entry.Mode, OID: entry.OID, Data: bytes}
	}
	return out, nil
}
