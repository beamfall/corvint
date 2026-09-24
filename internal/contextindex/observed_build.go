package contextindex

import (
	"context"
	"errors"
)

// LoaderObservation is the paired identity-and-status read a snapshot load made,
// carried out of a miss so the build that follows can open its stability
// window on it. IDX-SNAP-V0-002 already says the loader reads the repository
// exactly as a build's opening observation does; before this the miss threw
// that pair away and spawned it again, two of the eight Git processes a miss
// with the snapshot store present cost.
type LoaderObservation struct{ observed repositoryObservation }

var errNoEngine = errors.New("engine identity unavailable")

// Observation is the loader's pair in Observe's exported shape, for a caller
// outside the package that checks a read against it. A pair carried out of a
// hit has no error on either side: the loader returned them instead.
func (opening *LoaderObservation) Observation() Observation {
	observed := opening.observed
	return Observation{
		ObjectFormat:   observed.identity.objectFormat,
		CommitRevision: observed.identity.commitRevision,
		Revision:       observed.identity.treeRevision,
		StatusSHA256:   observed.statusSHA256,
		DirtyPaths:     observed.dirty,
	}
}

// LoadContextSnapshot is LoadSnapshot for the context verb: the same read,
// the same hit, and on a miss the loader's observation. The observation is
// nil when the loader spawned nothing (no snapshot directory) or when HEAD
// moved during the load, so a build never opens on a pair that straddles two
// revisions. The hit path is unchanged: decode overlaps the status scan and a
// closing identity read still guards the hit.
func LoadContextSnapshot(ctx context.Context, root string) (*Index, bool, *LoaderObservation, error) {
	return loadSnapshot(ctx, root, loadFull)
}

// LoadContextSnapshotDeferred is LoadContextSnapshot for a caller that hands
// the index only to TaskContext: a pack hit verifies every section but the
// bodies when it opens, and each body when the packet first reads it. A body
// that fails makes TaskContext return ErrSnapshotRefused instead of a packet,
// and the caller reloads through LoadContextSnapshot. Other formats are
// unchanged.
func LoadContextSnapshotDeferred(ctx context.Context, root string) (*Index, bool, *LoaderObservation, error) {
	return loadSnapshot(ctx, root, loadContext)
}

// LoadSnapshotDeferred is LoadSnapshot whose pack hit verifies each body when
// a read first touches it. The caller checks SnapshotRefusal after its reads
// and, on a refusal, discards the result and reloads through LoadSnapshot.
func LoadSnapshotDeferred(ctx context.Context, root string) (*Index, bool, error) {
	index, hit, _, err := loadSnapshot(ctx, root, loadContext)
	return index, hit, err
}

// LoadEventSnapshotDeferred is LoadEventSnapshot whose pack hit verifies each
// body when a read first touches it, under the same SnapshotRefusal contract
// as LoadSnapshotDeferred; the fallback is LoadEventSnapshot.
func LoadEventSnapshotDeferred(ctx context.Context, root string, compact bool) (*Index, bool, error) {
	load := loadEventDeferred
	if compact {
		load = loadCompact | loadDeferredBodies
	}
	index, hit, _, err := loadSnapshot(ctx, root, load)
	return index, hit, err
}

// BuildContextObserved is BuildContext opening its stability window on the
// loader's observation instead of spawning the identity and status pair
// again. A nil opening is BuildContext. The closing observation is still
// taken fresh, so the window it closes is the one the loader opened.
func BuildContextObserved(ctx context.Context, root, subject string, opening *LoaderObservation) (*Index, error) {
	if blobShardsEnabled() {
		if index, err := buildWithBlobShards(ctx, root, opening, contextCompile(subject)); err == nil {
			return index, nil
		}
	}
	if opening == nil {
		return BuildContext(ctx, root, subject)
	}
	return buildStableFrom(ctx, root, &opening.observed, contextCompile(subject))
}

// contextCompile is the complete compile pass BuildContext and
// BuildContextObserved share. The test relation can use imports for any
// anchor, including packets without a subject, so subject-only extraction
// would give cold reads different evidence from a full snapshot.
func contextCompile(subject string) func(*Index) {
	return func(index *Index) {
		index.compileTables(nil)
		index.sortUnparsed()
		index.Vocabulary = index.buildVocabulary()
		index.Vocabulary.SymbolWindows = index.buildSymbolWindows()
		index.Vocabulary.IdentGraph = index.buildIdentGraph(index.Vocabulary)
	}
}
