package doccorpus

import (
	"context"
	"sort"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/contextindex"
)

// inputLoader loads pinned inputs the revision index does not hold as text
// with a Git cost independent of the input count (DCP-V1-156): per revision,
// one verified tree walk resolves every such path, then one header batch
// admits every regular blob and one body batch reads them all.
type inputLoader struct {
	auth    *gitauth.Repository
	limit   int
	indexes map[string]*contextindex.Index
	staged  map[string]stagedInput // by inputKey
	sizes   map[string]int         // admitted header size by blob OID
	bodies  map[string][]byte      // verified body by blob OID
}

type stagedInput struct {
	entry  gitauth.TreeEntry
	exists bool
}

// indexedText returns the text the revision index already loaded for p
// within limit, without copying it.
func indexedText(index *contextindex.Index, p string, limit int) (contextindex.Source, string, bool) {
	if source, ok := index.Sources[p]; ok {
		text, valid, loaded := source.Text()
		if loaded && valid && len(text) <= limit {
			return source, text, true
		}
	}
	return contextindex.Source{}, "", false
}

func indexedInput(index *contextindex.Index, p string, limit int) (contextindex.Source, bool) {
	source, text, ok := indexedText(index, p, limit)
	if ok {
		source.Data = []byte(text)
	}
	return source, ok
}

func regularBlob(entry gitauth.TreeEntry) bool {
	return entry.Type == "blob" && (entry.Mode == "100644" || entry.Mode == "100755")
}

// resolvePathsPerPathBound resolves paths through one verified walk
// (DCP-V1-155) and refuses, as the per-path lookup does, a path whose
// LookupTreeEntry would stream more than gitauth.MaxTreeBytes of tree records,
// so the walk's aggregate bound never admits a path the per-path lookup
// refuses (DCP-V1-243).
func resolvePathsPerPathBound(ctx context.Context, auth *gitauth.Repository, commit string, paths []string) (map[string]gitauth.TreeEntry, error) {
	entries, lookups, err := auth.ResolvePathLookups(ctx, commit, paths)
	if err != nil {
		return nil, err
	}
	for _, p := range paths {
		if lookups[p] > gitauth.MaxTreeBytes {
			return nil, cemcode.New(cemcode.GitOutputExceeded, "Git stdout exceeded its byte bound")
		}
	}
	return entries, nil
}

// stageInputs resolves the inputs (revision -> paths) that indexes do not
// hold as text and reads the headers of their regular blobs. No body is read.
func stageInputs(ctx context.Context, auth *gitauth.Repository, indexes map[string]*contextindex.Index, inputs map[string][]string, limit int) (*inputLoader, error) {
	l := &inputLoader{auth: auth, limit: limit, indexes: indexes, staged: map[string]stagedInput{}, sizes: map[string]int{}, bodies: map[string][]byte{}}
	revisions := make([]string, 0, len(inputs))
	for rev := range inputs {
		revisions = append(revisions, rev)
	}
	sort.Strings(revisions)
	oids := []string{}
	seen := map[string]bool{}
	for _, rev := range revisions {
		index := indexes[rev]
		paths := []string{}
		for _, p := range inputs[rev] {
			if _, _, ok := indexedText(index, p, limit); !ok {
				if _, dup := l.staged[inputKey(rev, p)]; !dup {
					l.staged[inputKey(rev, p)] = stagedInput{}
					paths = append(paths, p)
				}
			}
		}
		if len(paths) == 0 {
			continue
		}
		entries, err := resolvePathsPerPathBound(ctx, auth, index.CommitRevision, paths)
		if err != nil {
			return nil, err
		}
		for _, p := range paths {
			entry, exists := entries[p]
			l.staged[inputKey(rev, p)] = stagedInput{entry: entry, exists: exists}
			if exists && regularBlob(entry) && !seen[entry.OID] {
				seen[entry.OID] = true
				oids = append(oids, entry.OID)
			}
		}
	}
	sort.Strings(oids)
	sizes, err := auth.BlobHeaders(ctx, oids)
	if err != nil {
		return nil, err
	}
	l.sizes = sizes
	return l, nil
}

// admit returns one input's byte size and blob OID, or loadInput's refusal,
// without reading its body.
func (l *inputLoader) admit(rev, p string) (int, string, error) {
	if source, text, ok := indexedText(l.indexes[rev], p, l.limit); ok {
		return len(text), source.BlobHash, nil
	}
	staged := l.staged[inputKey(rev, p)]
	if !staged.exists {
		return 0, "", &Error{Code: "corpus-input-unavailable", Message: "declared input missing"}
	}
	if !regularBlob(staged.entry) {
		return 0, "", fail("input is not a regular Git blob")
	}
	if size := l.sizes[staged.entry.OID]; size > l.limit || size > gitauth.MaxBlobBytes {
		return 0, "", fail("input bound exceeded")
	}
	return l.sizes[staged.entry.OID], staged.entry.OID, nil
}

// oversize reports the header size of a staged input over the loader's per-input bound or
// gitauth.MaxBlobBytes, read before any body (DCI-V1-011). An indexed text input is within the
// bound by construction. It also returns the staged blob, which pins an over-bound input
// without its body (DCI-V1-024).
func (l *inputLoader) oversize(rev, p string) (string, int, bool) {
	if _, _, ok := indexedText(l.indexes[rev], p, l.limit); ok {
		return "", 0, false
	}
	staged := l.staged[inputKey(rev, p)]
	if !staged.exists || !regularBlob(staged.entry) {
		return "", 0, false
	}
	size := l.sizes[staged.entry.OID]
	return staged.entry.OID, size, size > l.limit || size > gitauth.MaxBlobBytes
}

// read reads every admitted staged body in one batch. Call it only after
// every input it will serve was admitted.
func (l *inputLoader) read(ctx context.Context) error {
	oids := []string{}
	for _, staged := range l.staged {
		oid := staged.entry.OID
		if staged.exists && regularBlob(staged.entry) && l.sizes[oid] <= min(l.limit, gitauth.MaxBlobBytes) && l.bodies[oid] == nil {
			l.bodies[oid] = []byte{}
			oids = append(oids, oid)
		}
	}
	sort.Strings(oids)
	bodies, err := l.auth.BlobBatch(ctx, oids, l.sizes)
	if err != nil {
		return err
	}
	l.bodies = bodies
	return nil
}

// source returns one admitted input; read must have run.
func (l *inputLoader) source(rev, p string) (contextindex.Source, error) {
	if source, ok := indexedInput(l.indexes[rev], p, l.limit); ok {
		return source, nil
	}
	if _, _, err := l.admit(rev, p); err != nil {
		return contextindex.Source{}, err
	}
	entry := l.staged[inputKey(rev, p)].entry
	return contextindex.Source{Path: p, BlobHash: entry.OID, Data: l.bodies[entry.OID], Mode: entry.Mode}, nil
}

func loadInput(ctx context.Context, auth *gitauth.Repository, index *contextindex.Index, p string, limit int) (contextindex.Source, error) {
	rev := index.CommitRevision
	l, err := stageInputs(ctx, auth, map[string]*contextindex.Index{rev: index}, map[string][]string{rev: {p}}, limit)
	if err != nil {
		return contextindex.Source{}, err
	}
	if _, _, err := l.admit(rev, p); err != nil {
		return contextindex.Source{}, err
	}
	if err := l.read(ctx); err != nil {
		return contextindex.Source{}, err
	}
	return l.source(rev, p)
}
