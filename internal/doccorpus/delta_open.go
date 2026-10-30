package doccorpus

import (
	"context"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/gitstatus"
)

// ValidationUnrederived is the timings validation state of a delta current side taken from its
// canonical bytes because a declared additional checkout is unavailable (DCX-V1-425, DCR-V1-460).
// It is diagnostics only and never enters a receipt.
const ValidationUnrederived = "checkout-unavailable"

// ValidationManifestBuilt is the timings validation state of a delta current side built from
// --manifest (DCR-V1-462); diagnostics only.
const ValidationManifestBuilt = "manifest-built"

// OpenDeltaCurrent opens a retained artifact as the current side of `docs corpus delta`
// with the complete rederivation: OpenDeltaRead with Rederive set.
func OpenDeltaCurrent(ctx context.Context, root string, data []byte) (*Artifact, error) {
	return OpenDeltaRead(ctx, root, data, ReadOptions{Rederive: true})
}

// OpenDeltaRead opens a retained artifact as the current side of `docs corpus delta`
// (DCX-V1-425, DCR-V1-460). While every declared additional checkout holds its identity at each
// pinned revision it is OpenRead: the verified read by default, the complete rederivation with
// opts.Rederive or an unattested running builder, and every failure refuses; the verified read
// also checks that every input names the regular blob its path holds at its pinned revision
// before anything is classified (DCR-V1-464). When one does not,
// the artifact cannot be verified or rederived, so it is taken from its canonical, digest-checked
// bytes instead: every input of an available repository must still name the blob its path holds
// at its pinned revision, the unavailable repositories' anchors classify unresolved with the
// reason, and the receipt says the current side was not rederived. Which branch runs depends only
// on the checkouts, never on whether the verified read succeeded. opts.Header is replaced.
func OpenDeltaRead(ctx context.Context, root string, data []byte, opts ReadOptions) (*Artifact, error) {
	t := opts.Timings
	if opts.Rederive || !rebuildAttested(currentBuilder()) {
		start := time.Now()
		a, err := ParseArtifact(data)
		if t != nil {
			t.Parse += time.Since(start)
		}
		if err != nil {
			return nil, err
		}
		// Git runs in the checkouts the manifest names only once the manifest is valid, as on
		// the verified read.
		if err := validateManifest(a.Manifest); err != nil {
			return nil, err
		}
		missing, err := unavailableCheckouts(ctx, root, a.Manifest)
		if err != nil {
			return nil, err
		}
		if len(missing) > 0 {
			return openDeltaUnrederived(ctx, root, a, missing, t)
		}
		opts.Rederive, opts.Header = true, nil
		return OpenRead(ctx, root, data, opts)
	}
	// The checkouts are observed from the header members while the records are decoded and
	// verified, as a read's freshness is (DCR-V1-411).
	// With every checkout available, each input's (revision, path) -> blob binding is checked
	// against its pinned tree there too, because the verified read checks only the pins and the
	// classification trusts the recorded blobs (DCR-V1-464).
	var missing map[string]string
	var missingErr, bindingErr error
	var bindingTime time.Duration
	opts.Header = func(h *Artifact) {
		if missing, missingErr = unavailableCheckouts(ctx, root, h.Manifest); missingErr == nil && len(missing) == 0 {
			start := time.Now()
			bindingErr = verifyAvailableInputs(ctx, root, h.Manifest, nil)
			bindingTime = time.Since(start)
		}
	}
	a, err := OpenRead(ctx, root, data, opts)
	if missingErr != nil {
		return nil, missingErr
	}
	if len(missing) == 0 {
		if t != nil {
			t.Verify += bindingTime
		}
		if err != nil {
			return nil, err
		}
		if bindingErr != nil {
			return nil, bindingErr
		}
		return a, nil
	}
	start := time.Now()
	a, err = ParseArtifact(data)
	if t != nil {
		t.Parse += time.Since(start)
	}
	if err != nil {
		return nil, err
	}
	return openDeltaUnrederived(ctx, root, a, missing, t)
}

// openDeltaUnrederived checks a canonically decoded current side whose missing checkouts
// prevent its rederivation (DCX-V1-425).
func openDeltaUnrederived(ctx context.Context, root string, a *Artifact, missing map[string]string, t *ReadTimings) (*Artifact, error) {
	start := time.Now()
	err := verifyAvailableInputs(ctx, root, a.Manifest, missing)
	if t != nil {
		t.Verify += time.Since(start)
	}
	if err != nil {
		return nil, err
	}
	ids := sortedKeys(missing)
	a.deltaUnverified = []string{"current artifact was not rederived because the checkout of additional repository " + strings.Join(ids, ", ") + " is unavailable; its records are taken from the canonical artifact bytes, every input of an available repository was checked against its pinned Git tree, and anchors in the unavailable repository are unresolved"}
	if t != nil {
		t.Validation = ValidationUnrederived
	}
	return a, nil
}

// OpenDeltaPrevious admits the `--previous` artifact of `docs corpus delta` and
// `carry-forward` (DCX-V1-360, DCR-V1-461) with the verified read's decode: one scan, the
// record elements decoded and compared with their canonical encoding in parallel, and the
// content digest hashed while they are (DCR-V1-414). It admits and refuses exactly what
// ParseArtifact does. Builder identity and pins are not checked, because the worklist must not
// depend on which builder built the previous corpus (DCX-V1-424), and nothing is rederived.
func OpenDeltaPrevious(data []byte, t *ReadTimings) (*Artifact, error) {
	start := time.Now()
	defer func() {
		if t != nil {
			t.Parse += time.Since(start)
		}
	}()
	var digest artifactTail
	var wg sync.WaitGroup
	wg.Go(func() { digest = hashArtifactTail(data) })
	a, err := decodeArtifactParallel(data, nil)
	wg.Wait()
	if err != nil {
		return nil, err
	}
	if err := digest.verify(a); err != nil {
		return nil, err
	}
	return a, nil
}

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
