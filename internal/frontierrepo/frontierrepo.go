// Package frontierrepo binds the frontier-stable-view/0 seam to real Git
// authority. `internal/frontier` holds `frontier.StableViewer` as an interface
// so its admission order and translation stay testable without a repository;
// this package is the one implementation, over the stable machinery in
// `internal/lrfrepo`, `internal/cem/...` and `internal/tcq`.
//
// The frontier/0 seams (the shared cem/0.2 verifier and the tcq/0
// recomputer) were deleted with the cem/0.x wire (decision 0525 §1,
// CEM-V1-217). Per CF-V0-027 nothing here may be wired into the agent harness,
// and per CF-V0-026 path acquisition stays outside: an Adapter is handed
// artifact BYTES and a repository root, never artifact paths.
package frontierrepo

import (
	"context"
	"crypto/sha256"
	"encoding/hex"

	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/frontier"
	"github.com/Beamfall/corvint/internal/lrfrepo"
	"github.com/Beamfall/corvint/internal/tcq"
)

// Adapter is one repository root bound to the stable view seam. The context
// is held on the value because TCQ's bounded tree reader is a context-free
// interface owned by its producer.
type Adapter struct {
	ctx  context.Context
	root string
}

// New binds an adapter to one repository root. It performs no Git work.
func New(ctx context.Context, root string) *Adapter {
	return &Adapter{ctx: ctx, root: root}
}

// obligations carries the verified OCM rows across the seam field for field.
// Nothing is filtered: dropping an unknown row would silently remove an
// obligation from the view.
func obligations(rows []lrfrepo.Obligation) []frontier.Obligation {
	projected := make([]frontier.Obligation, 0, len(rows))
	for _, row := range rows {
		projected = append(projected, frontier.Obligation{
			ID:          row.ID,
			Disposition: row.Disposition,
			Reason:      row.Reason,
			HunkIDs:     row.HunkIDs,
			ClaimIDs:    row.ClaimIDs,
		})
	}
	return projected
}

// claims carries the producer's per-edge results across the seam field for
// field. Every selected edge crosses it, so a filter here would fabricate a
// closure.
func claims(results []tcq.ClaimResult) []frontier.TCQClaimResult {
	projected := make([]frontier.TCQClaimResult, 0, len(results))
	for _, result := range results {
		projected = append(projected, frontier.TCQClaimResult{
			ObligationID:   result.ObligationID,
			ClaimID:        result.ClaimID,
			Reasons:        result.Reasons,
			Relation:       result.Relation,
			AuthorityClass: result.AuthorityClass,
		})
	}
	return projected
}

func namesRevision(candidate, requested, resolved string) bool {
	return candidate == requested || candidate == resolved
}

// treeReader is the inherited bounded, sanitized, no-fetch Git boundary TCQ
// reads source through. It adds nothing: worktree bytes, sibling repositories,
// implicit fetch and additional-repository discovery stay refused by gitauth.
type treeReader struct {
	adapter    *Adapter
	repository *gitauth.Repository
}

// Blob returns one target-tree blob by OID through Git object identity.
func (reader treeReader) Blob(oid string) ([]byte, error) {
	return reader.repository.BlobBytes(reader.adapter.ctx, oid)
}

// TreeEntry resolves one normalized repository-relative path at revision. An
// absent path is the empty entry with no error.
func (reader treeReader) TreeEntry(revision, path string) (tcq.TreeEntry, error) {
	entry, exists, err := reader.repository.LookupTreeEntry(reader.adapter.ctx, revision, path)
	if err != nil {
		return tcq.TreeEntry{}, err
	}
	if !exists {
		return tcq.TreeEntry{}, nil
	}
	return tcq.TreeEntry{Mode: entry.Mode, Type: entry.Type, Found: true}, nil
}

func digest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
