package contextindex

import (
	"os"
	"sync/atomic"
)

// snapshotMapping owns one read-only mapping of a pack or sectioned snapshot
// file and unmaps it when its last reference is released (proposed
// IDX-SNAP-V0-028, V1-0983). The retention ring holds one reference while the
// file is retained, and every handed-out Index that aliases the mapping holds
// one through its snapshotLease. Reachability never releases a reference:
// strings and slices into a mapping do not keep any Go object alive, so a
// collector-driven unmap faults on an alias a caller kept after dropping its
// Index. A mapping is therefore unmapped only after the ring evicted it and
// every Index read from it was explicitly released.
type snapshotMapping struct {
	bytes []byte
	refs  atomic.Int64
}

// liveSnapshotMappings counts the snapshot mappings mapped and not yet
// unmapped in this process.
var liveSnapshotMappings atomic.Int64

// mapSnapshot maps file and returns its owner holding the opener's reference,
// or nil where the platform has no mapping or the map failed.
func mapSnapshot(file *os.File, size int64) *snapshotMapping {
	bytes := mapReadOnly(file, size)
	if bytes == nil {
		return nil
	}
	liveSnapshotMappings.Add(1)
	mapping := &snapshotMapping{bytes: bytes}
	mapping.refs.Store(1)
	return mapping
}

// data is the mapped bytes, nil for a nil owner.
func (mapping *snapshotMapping) data() []byte {
	if mapping == nil {
		return nil
	}
	return mapping.bytes
}

// lease adds a reference for an Index. The caller already holds one, or
// holds the retention lock while the ring holds one, so the count is never
// raised from zero.
func (mapping *snapshotMapping) lease() *snapshotLease {
	if mapping == nil {
		return nil
	}
	mapping.refs.Add(1)
	return &snapshotLease{mapping: mapping}
}

// release drops one reference and unmaps on the last.
func (mapping *snapshotMapping) release() {
	if mapping == nil {
		return
	}
	switch remaining := mapping.refs.Add(-1); {
	case remaining == 0:
		unmapReadOnly(mapping.bytes)
		liveSnapshotMappings.Add(-1)
	case remaining < 0:
		panic("contextindex: snapshot mapping released more often than referenced")
	}
}

// snapshotLease is an Index's reference to the mapping it aliases. Copies of
// an Index share the lease, and releasing it twice drops one reference.
type snapshotLease struct {
	mapping  *snapshotMapping
	released atomic.Bool
}

func (lease *snapshotLease) release() {
	if lease != nil && lease.released.CompareAndSwap(false, true) {
		lease.mapping.release()
	}
}

// attachLease hands lease to index, or releases it when the load aliases
// nothing (a compact load copies its one identity field).
func attachLease(index *Index, lease *snapshotLease, load snapshotLoad) {
	if load.tables() == loadCompact {
		lease.release()
		return
	}
	index.lease = lease
}

// Release ends the caller's use of a pack or sectioned snapshot read. After
// it, the caller must not read the index's Source.Data, any Source.Text
// result, or its Vocabulary, nor any value it derived from them without
// copying: those alias the mapped file, which is unmapped once the retention
// ring has also evicted it. Every string field the read handed out (Source
// Path, BlobHash and Mode, Symbol fields, Tracked and Skipped keys,
// Vocabulary.Paths, co-change history) is heap-backed and stays valid.
// Release is idempotent, a no-op for a gob read or a build, and optional: an
// Index never released keeps its mapping valid for the process's life.
func (index *Index) Release() {
	if index != nil {
		index.lease.release()
	}
}
