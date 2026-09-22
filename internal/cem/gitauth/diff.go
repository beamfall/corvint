package gitauth

import (
	"context"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/patch"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

// CanonicalDiff derives the canonical CEM patch bytes from base to target over
// the whole repository, excluding exactly the frozen sidecar path.
//
// Every attribute source Git consults is neutralized: worktree and tree
// attributes via --attr-source pinned to the empty tree, global and system
// files via the scrubbed environment and config pins, and repository
// info/attributes by the fail-closed refusal in Open. The diff shape flags are
// pinned so repository and ambient diff configuration cannot alter the bytes.
//
// Git's diff machinery reads trees and blobs without checking them against
// their names, so the patch it emits is only accepted once it is proved to be
// the canonical derivation of the verified objects: the change set is walked
// from self-hashed tree bodies on both sides (verifiedChangeSet), every blob a
// section derives from is self-hashed, and the patch must match that set
// section for section and replay the old bytes into the new bytes
// (requirePatchProvenance). Because the proof is against verified content
// rather than a second Git read, an object swapped between the diff read and
// the verification read yields a patch that fails the proof. The bodies read
// for the proof are neither retained nor charged to the blob budget.
//
// Failures keep their types: derivation maps a Git exit to git-diff-failed and
// a per-operation timeout to git-diff-timeout, while cancellation, output, and
// budget failures keep their own registered codes.
func (r *Repository) CanonicalDiff(ctx context.Context, baseOID, targetOID string) ([]byte, error) {
	if r.ObjectFormat == "" {
		if err := r.LoadObjectFormat(ctx); err != nil {
			return nil, err
		}
	}
	key, eligible, err := r.requestKey(memoDiff, baseOID, targetOID)
	if err != nil {
		return nil, err
	}
	if eligible {
		value, hit, err := r.recalled(ctx, key)
		if err != nil {
			return nil, canonicalDiffError(err)
		}
		if hit {
			return value.data, nil
		}
	}
	arguments := []string{
		"--attr-source=" + r.EmptyTreeOID(),
		"diff", "--full-index", "--no-color", "--no-ext-diff", "--no-textconv",
		"--no-renames", "--no-indent-heuristic", "--diff-algorithm=myers",
		"--unified=3", "--inter-hunk-context=0", "--src-prefix=a/", "--dst-prefix=b/",
		"--ignore-submodules=none", "--end-of-options", baseOID, targetOID,
		"--", ".", ":(exclude)" + wire.ExcludedCEMPath,
	}
	out, err := r.git(ctx, patch.MaxPatchBytes, arguments...)
	if err != nil {
		return nil, canonicalDiffError(err)
	}
	changed, err := r.verifiedChangeSet(ctx, baseOID, targetOID)
	if err != nil {
		return nil, canonicalDiffError(err)
	}
	if err := r.requirePatchProvenance(ctx, out, changed); err != nil {
		return nil, canonicalDiffError(err)
	}
	if eligible {
		if err := r.remember(ctx, key, memoValue{data: out}); err != nil {
			return nil, canonicalDiffError(err)
		}
	}
	return out, nil
}

func canonicalDiffError(err error) error {
	switch cemcode.CodeOf(err) {
	case cemcode.GitExitFailure, cemcode.GitStartFailed:
		return cemcode.New(cemcode.GitDiffFailed, "Git could not derive the canonical CEM patch: %v", err)
	case cemcode.GitTimeout:
		return cemcode.New(cemcode.GitDiffTimeout, "canonical CEM patch derivation timed out")
	default:
		return err
	}
}
