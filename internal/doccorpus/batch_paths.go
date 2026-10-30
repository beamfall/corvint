package doccorpus

import (
	"context"
	"sort"

	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

// pathPrefetch holds tree entries and blob bodies read in batches, so a
// reader that consumes many paths one at a time spends a Git operation count
// independent of the path count (DCP-V1-216): one verified path walk per
// commit (DCP-V1-155), one header batch and one body batch per read. It is
// only a cache in front of the per-path reads. A path or blob it does not
// hold, because a batch failed or a bound left it out, is read by the same
// per-path call as before, so every refusal, its code and its order are
// unchanged; a batch failure other than cancellation is never itself a
// result. A nil *pathPrefetch holds nothing.
type pathPrefetch struct {
	commit   string
	entries  map[string]gitauth.TreeEntry
	resolved map[string]bool
	sizes    map[string]int
	bodies   map[string][]byte
}

// pathPrefetchOff reports whether a test context turned every batch into a
// no-op, so the test can compare batched output with the per-path reads byte
// for byte.
func pathPrefetchOff(ctx context.Context) bool {
	return corpusGitHooksOf(ctx).prefetchOff
}

func newPathPrefetch(commit string) *pathPrefetch {
	return &pathPrefetch{commit: commit, entries: map[string]gitauth.TreeEntry{}, resolved: map[string]bool{}, sizes: map[string]int{}, bodies: map[string][]byte{}}
}

// resolve walks every path not yet resolved through one verified batched
// walk and holds the entries of the paths a per-path lookup would admit.
// Only a cancelled context is an error.
func (p *pathPrefetch) resolve(ctx context.Context, auth *gitauth.Repository, paths []string) error {
	pending := []string{}
	seen := map[string]bool{}
	for _, path := range paths {
		if !p.resolved[path] && !seen[path] && wire.ValidatePath(path) == nil {
			seen[path] = true
			pending = append(pending, path)
		}
	}
	if len(pending) == 0 || !wire.IsGitOid(p.commit) || pathPrefetchOff(ctx) {
		return nil
	}
	sort.Strings(pending)
	entries, lookups, err := auth.ResolvePathLookups(ctx, p.commit, pending)
	if err != nil {
		return ctx.Err()
	}
	for _, path := range pending {
		// Only a path the per-path lookup would admit is answered from the
		// walk: one whose parent trees all resolved and stream within
		// MaxTreeBytes. Any other path keeps its per-path lookup and refusal.
		if bytes, ok := lookups[path]; !ok || bytes > gitauth.MaxTreeBytes {
			continue
		}
		p.resolved[path] = true
		if entry, ok := entries[path]; ok {
			p.entries[path] = entry
		}
	}
	return nil
}

// regularOIDs returns the distinct regular-blob OIDs of the resolved paths,
// in the paths' order.
func (p *pathPrefetch) regularOIDs(paths []string) []string {
	oids := []string{}
	for _, path := range paths {
		if entry, ok := p.entries[path]; ok && regularBlob(entry) {
			oids = append(oids, entry.OID)
		}
	}
	return oids
}

// read admits the headers of oids not yet held in one batch and reads, in
// one more, the bodies of those within limit, in the given order until their
// total would pass maxBytes. A blob left out is read per object on demand.
// Only a cancelled context is an error.
func (p *pathPrefetch) read(ctx context.Context, auth *gitauth.Repository, oids []string, limit, maxBytes int) error {
	pending := []string{}
	seen := map[string]bool{}
	for _, oid := range oids {
		if _, held := p.sizes[oid]; !held && !seen[oid] && wire.IsGitOid(oid) {
			seen[oid] = true
			pending = append(pending, oid)
		}
	}
	if len(pending) == 0 || pathPrefetchOff(ctx) {
		return nil
	}
	sorted := append([]string(nil), pending...)
	sort.Strings(sorted)
	sizes, err := auth.BlobHeaders(ctx, sorted)
	if err != nil {
		return ctx.Err()
	}
	admitted, total := []string{}, 0
	for _, oid := range pending {
		size := sizes[oid]
		if size > limit {
			p.sizes[oid] = size // over the bound: refused from the header, never read
			continue
		}
		if total+size > maxBytes {
			continue
		}
		total += size
		admitted = append(admitted, oid)
	}
	sort.Strings(admitted)
	bodies, err := auth.BlobBatch(ctx, admitted, sizes)
	if err != nil {
		return ctx.Err()
	}
	for oid, body := range bodies {
		p.sizes[oid], p.bodies[oid] = sizes[oid], body
	}
	return nil
}

// lookup is auth.LookupTreeEntry answered from the walk when it resolved path.
func (p *pathPrefetch) lookup(ctx context.Context, auth *gitauth.Repository, commit, path string) (gitauth.TreeEntry, bool, error) {
	if p != nil && commit == p.commit && p.resolved[path] {
		entry, ok := p.entries[path]
		return entry, ok, nil
	}
	return auth.LookupTreeEntry(ctx, commit, path)
}

// blobWithin is auth.BlobBytesWithin answered from the batches when they
// hold oid: an over-bound header is over without a body, and a held body was
// verified against its size and object identity by the batch.
func (p *pathPrefetch) blobWithin(ctx context.Context, auth *gitauth.Repository, oid string, limit int) ([]byte, bool, error) {
	if p != nil {
		if size, held := p.sizes[oid]; held && size > limit {
			return nil, true, nil
		}
		if body, held := p.bodies[oid]; held && len(body) <= limit {
			return body, false, nil
		}
	}
	return auth.BlobBytesWithin(ctx, oid, limit)
}

// held returns the body a batch already read for oid.
func (p *pathPrefetch) held(oid string) ([]byte, bool) {
	if p == nil {
		return nil, false
	}
	body, ok := p.bodies[oid]
	return body, ok
}

// heldBytes is the total of the bodies the batches have read.
func (p *pathPrefetch) heldBytes() int {
	if p == nil {
		return 0
	}
	total := 0
	for _, body := range p.bodies {
		total += len(body)
	}
	return total
}

// blob is auth.BlobBytes answered from the batches when they hold oid.
func (p *pathPrefetch) blob(ctx context.Context, auth *gitauth.Repository, oid string) ([]byte, error) {
	if p != nil {
		if body, held := p.bodies[oid]; held {
			return body, nil
		}
	}
	return auth.BlobBytes(ctx, oid)
}
