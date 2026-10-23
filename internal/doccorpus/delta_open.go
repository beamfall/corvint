package doccorpus

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/gitstatus"
)

// OpenDeltaCurrent opens a retained artifact as the current side of `docs corpus delta`
// (DCP-V1-425). While every declared additional checkout holds its identity at each pinned
// revision it is the verified read of OpenRead, without decoding the artifact twice, and the
// receipt says the current side was verified rather than rederived (DCP-V1-432); rederive, or a
// running builder whose label names no immutable build, runs Open instead. When a checkout does
// not hold its identity, the artifact cannot be rederived, so it is taken from its canonical,
// digest-checked bytes instead: every input of an available repository must still name the blob
// its path holds at its pinned revision, the unavailable repositories' anchors classify
// unresolved with the reason, and the receipt says the current side was not rederived.
func OpenDeltaCurrent(ctx context.Context, root string, data []byte, rederive bool) (*Artifact, error) {
	a, err := ParseArtifact(data)
	if err != nil {
		return nil, err
	}
	missing, err := unavailableCheckouts(ctx, root, a.Manifest)
	if err != nil {
		return nil, err
	}
	if len(missing) == 0 {
		if rederives(ReadOptions{Rederive: rederive}) {
			return Open(ctx, root, data)
		}
		if err := verifyRecorded(ctx, root, a); err != nil {
			return nil, err
		}
		a.validation = ValidationVerified
		a.deltaUnverified = []string{deltaVerifiedCurrent}
		return a, nil
	}
	if err := verifyAvailableInputs(ctx, root, a.Manifest, missing); err != nil {
		return nil, err
	}
	ids := sortedKeys(missing)
	a.deltaUnverified = []string{"current artifact was not rederived because the checkout of additional repository " + strings.Join(ids, ", ") + " is unavailable; its records are taken from the canonical artifact bytes, every input of an available repository was checked against its pinned Git tree, and anchors in the unavailable repository are unresolved"}
	return a, nil
}

// deltaVerifiedCurrent is the delta receipt's limitation for a current side admitted by the
// verified read (DCP-V1-432).
const deltaVerifiedCurrent = "current artifact was admitted by the verified read: its canonical bytes, recorded digest, builder, manifest and profile digests and pinned revisions were checked; its records were not rederived from the pinned inputs"

// unavailableCheckouts maps each declared additional repository whose checkout does not hold its
// declared identity at every pinned revision to the reason. Only cancellation is an error.
func unavailableCheckouts(ctx context.Context, root string, m Manifest) (map[string]string, error) {
	ctx = gitstatus.WithIsolation(ctx)
	missing := map[string]string{}
	for _, r := range m.AdditionalRepositories {
		dir := filepath.Join(root, filepath.FromSlash(r.Root))
		for _, revision := range r.pinned() {
			if got, err := contextindex.CorpusRepositoryID(ctx, dir, revision); err != nil || got != r.ID {
				if ctx.Err() != nil {
					return nil, ctx.Err()
				}
				missing[r.ID] = "checkout does not hold revision " + revision
				break
			}
		}
	}
	return missing, nil
}

// verifyAvailableInputs checks the primary identity and that every input outside the missing
// repositories names the regular blob its path holds at its revision, with one batched path walk
// per revision.
func verifyAvailableInputs(ctx context.Context, root string, m Manifest, missing map[string]string) (err error) {
	defer func() { err = gitCapacity(err, "while verifying the current artifact's pinned inputs") }()
	ctx = gitstatus.WithIsolation(ctx)
	if got, err := contextindex.CorpusRepositoryID(ctx, root, m.Repository.Revision); err != nil || got != m.Repository.ID {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fail("artifact cannot be rederived: the primary checkout does not hold its declared identity and revision")
	}
	byRevision := map[string][]Input{}
	for _, in := range m.Inputs {
		if id, _ := m.revisionRepository(root, in.Revision); missing[id] == "" {
			byRevision[in.Revision] = append(byRevision[in.Revision], in)
		}
	}
	for _, revision := range sortedKeys(byRevision) {
		auth, err := gitauth.Open(m.RevisionRoot(root, revision), newGitBudget(ctx))
		if err != nil {
			return err
		}
		release := auth.BeginObjectSession()
		pre := newPathPrefetch(revision)
		paths := make([]string, 0, len(byRevision[revision]))
		for _, in := range byRevision[revision] {
			paths = append(paths, in.Path)
		}
		err = pre.resolve(ctx, auth, paths)
		for _, in := range byRevision[revision] {
			if err != nil {
				break
			}
			entry, ok, lookupErr := pre.lookup(ctx, auth, revision, in.Path)
			if lookupErr != nil {
				err = lookupErr
			} else if !ok || !regularBlob(entry) || entry.OID != in.Blob {
				err = fail("artifact cannot be rederived: input " + in.Path + " at " + revision + " does not name the blob its path holds")
			}
		}
		release()
		if err != nil {
			return err
		}
	}
	return nil
}
