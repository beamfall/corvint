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
// Failures keep their types: derivation maps a Git exit to git-diff-failed,
// unless a needed object is missing locally (repository-object-unavailable), and
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
		return nil, r.diffExitError(ctx, err, baseOID, targetOID)
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

// CanonicalDiffWithCreateDestinations derives the same canonical patch as
// CanonicalDiff and returns the authenticated base-side tree entry for every
// create side after the patch provenance has been proved. A zero TreeEntry
// means the path was absent in the verified base tree.
func (r *Repository) CanonicalDiffWithCreateDestinations(ctx context.Context, baseOID, targetOID string) ([]byte, map[string]TreeEntry, error) {
	if r.ObjectFormat == "" {
		if err := r.LoadObjectFormat(ctx); err != nil {
			return nil, nil, err
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
		return nil, nil, r.diffExitError(ctx, err, baseOID, targetOID)
	}
	changed, err := r.verifiedChangeSet(ctx, baseOID, targetOID)
	if err != nil {
		return nil, nil, canonicalDiffError(err)
	}
	if err := r.requirePatchProvenance(ctx, out, changed); err != nil {
		return nil, nil, canonicalDiffError(err)
	}
	createDestinations := make(map[string]TreeEntry)
	for _, entry := range changed {
		if ctx.Err() != nil || (r.budget != nil && r.budget.OuterExpired()) {
			return nil, nil, cemcode.New(cemcode.GitCancelled, "canonical create inventory cancelled")
		}
		// These are the create sides of the already-proved patch sections.
		// entry.base retains actual trees before section normalization.
		if entry.new.OID == "" || (entry.old.OID != "" && entryKind(entry.old.Mode) == entryKind(entry.new.Mode)) {
			continue
		}
		createDestinations[entry.path] = entry.base
	}
	if ctx.Err() != nil || (r.budget != nil && r.budget.OuterExpired()) {
		return nil, nil, cemcode.New(cemcode.GitCancelled, "canonical create inventory cancelled")
	}
	return out, createDestinations, nil
}

// diffExitError classifies a failed diff read. Git exits unsuccessfully when a blob the diff needs
// is a promised object that is not present locally, because the read environment forbids the lazy
// fetch. CEM-CB-019 codes that as repository-object-unavailable, not as a patch-derivation
// failure, so on a Git exit the verified change set and its blobs are walked, without fetching,
// and their missing-object refusal is returned when they have one (V1-0349). A cancellation,
// timeout or budget failure during that walk is returned through the same mapping as a diff
// failure. Any other outcome keeps the diff's own code.
func (r *Repository) diffExitError(ctx context.Context, err error, baseOID, targetOID string) error {
	if cemcode.CodeOf(err) != cemcode.GitExitFailure {
		return canonicalDiffError(err)
	}
	changed, walkErr := r.verifiedChangeSet(ctx, baseOID, targetOID)
	if walkErr == nil {
		walkErr = r.missingPatchBlob(ctx, patchSections(changed))
	}
	switch cemcode.CodeOf(walkErr) {
	case cemcode.RepositoryObjectUnavailable, cemcode.GitCancelled, cemcode.GitTimeout, cemcode.GitBudgetExceeded:
		return canonicalDiffError(walkErr)
	}
	return canonicalDiffError(err)
}

// missingPatchBlob returns the error that reading the sections' blobs fails with. A Git that
// ignores GIT_NO_LAZY_FETCH exits from the batch read instead of reporting the object missing, so
// each distinct blob is then probed alone with cat-file -e, where an exit names the object. The
// probe reads no content, so it charges no blob budget and fills no memo.
func (r *Repository) missingPatchBlob(ctx context.Context, sections []patchSection) error {
	_, err := r.verifiedBlobs(ctx, sections)
	if cemcode.CodeOf(err) != cemcode.GitExitFailure {
		return err
	}
	probed := map[string]bool{}
	for _, section := range sections {
		if section.old.OID == section.new.OID {
			continue
		}
		for _, side := range []TreeEntry{section.old, section.new} {
			if side.OID == "" || side.Type != "blob" || probed[side.OID] {
				continue
			}
			probed[side.OID] = true
			if _, probeErr := r.git(ctx, 64, "cat-file", "-e", side.OID); probeErr != nil {
				if cemcode.CodeOf(probeErr) == cemcode.GitExitFailure {
					return unavailable("patch input %s does not resolve locally", side.OID)
				}
				return probeErr
			}
		}
	}
	return err
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
