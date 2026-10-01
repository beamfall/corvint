package cemcandidate

import (
	"context"
	"fmt"
	"path"
	"path/filepath"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/publish"
	cw "github.com/Beamfall/corvint/internal/cem/wire"
	tr "github.com/Beamfall/corvint/internal/testrunner"
)

// Read uses the existing bounded no-follow CEM reader. Parent components are
// checked against a publication root; the selected filename cannot be Git metadata.
func Read(name string) ([]byte, error) {
	absolute, e := filepath.Abs(name)
	if e != nil {
		return nil, e
	}
	relative := strings.TrimPrefix(filepath.ToSlash(absolute), "/")
	if !literal(relative) {
		return nil, fmt.Errorf("unsafe input path")
	}
	root, e := publish.OpenRoot(string(filepath.Separator))
	if e != nil {
		return nil, e
	}
	return root.ReadBounded(relative, MaxDocument, "candidate-input")
}
func readPinned(ctx context.Context, p FileRef) ([]byte, error) {
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	raw, e := Read(p.Path)
	if e != nil {
		return nil, e
	}
	if tr.Digest(raw) != p.Sha256 {
		return nil, fmt.Errorf("input digest changed: %s", p.Path)
	}
	return raw, nil
}
func sourceBinding(ctx context.Context, repo *gitauth.Repository, r Request, plan tr.PlanDocument) (string, error) {
	if len(plan.Request.InputFiles) == 0 || len(plan.Request.InputFiles) > 4096 {
		return "", fmt.Errorf("runner input inventory bound")
	}
	if _, e := repo.CommitTree(ctx, r.Target); e != nil {
		return "", e
	}
	total := 0
	for name, want := range plan.Request.InputFiles {
		if !literal(name) || !cw.IsSha256(want) {
			return "", fmt.Errorf("invalid declared source input")
		}
		p := name
		if r.SourcePrefix != "" {
			p = path.Join(r.SourcePrefix, name)
		}
		entry, ok, e := repo.LookupTreeEntry(ctx, r.Target, p)
		if e != nil {
			return "", e
		}
		if !ok || entry.Type != "blob" || (entry.Mode != "100644" && entry.Mode != "100755") {
			return "", fmt.Errorf("declared input is not a regular immutable blob: %s", p)
		}
		raw, e := repo.BlobBytes(ctx, entry.OID)
		if e != nil {
			return "", e
		}
		total += len(raw)
		if total > 128<<20 {
			return "", fmt.Errorf("source input byte bound")
		}
		if tr.Digest(raw) != want {
			return "", fmt.Errorf("declared input differs from target: %s", p)
		}
	}
	return tr.Identity(plan.Request.InputFiles), nil
}
