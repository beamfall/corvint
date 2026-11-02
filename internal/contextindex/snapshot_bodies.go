package contextindex

import (
	"encoding/binary"
	"errors"
	"sync/atomic"
)

// The gob snapshot's source bodies (format corvint-index-snapshot/4,
// IDX-SNAP-V0-811). Format /3 carried every body inside the gob messages, so
// every load copied about 117 MB out of the mapping and gob-decoded it, half
// of the decode's CPU, although a packet reads a few dozen bodies. Format /4
// writes each body whose bytes rehash to its BlobHash once, raw, into a body
// region after the gob messages, followed by the region's length as an
// 8-byte footer; the trailer digest covers the gob messages and the footer,
// and each region body is verified by its own BlobHash instead. A body that
// does not rehash to its BlobHash stays inline in the gob messages.
//
// A full load copies every region body out of the mapping under the
// fault guard and verifies it before returning, so it hands out the same
// Index as /3. A deferred load (the SnapshotRefusal contract of
// IDX-SNAP-V0-015) keeps the mapping under a lease and gives each region
// source a body that Source.Text copies out under the guard, verifies against
// BlobHash and caches on first read. Nothing a read returns aliases the
// mapping, so a file truncated in place is a fault inside the guard, recorded
// as a refusal, never a SIGBUS in a reader.

// wireSourceMeta is a Source as the /4 gob messages carry it.
type wireSourceMeta struct {
	Path, BlobHash, Mode string
	Checked, Valid       bool
	// Length is len(Data), or -1 for a nil or empty Data (which /3 also
	// decoded as nil); InRegion says the bytes are the next Length bytes of
	// the body region, else they are Inline.
	Length   int
	InRegion bool
	Inline   []byte
}

// snapshotFooterBytes is the region-length footer before the trailer.
const snapshotFooterBytes = 8

// snapshotBodies is the body region a decode reads from, and what a deferred
// decode handed out.
type snapshotBodies struct {
	region       []byte
	mapping      *snapshotMapping // non-nil when region aliases a mapping
	deferred     bool
	objectFormat string
	// messageBytes is the length of the gob messages the decode reads, the
	// bound of every count the spine declares (IDX-SNAP-V0-812).
	messageBytes int64
	lazy         *gobBodies     // set when the decode handed out lazy bodies
	lease        *snapshotLease // the mapping's reference those bodies hold
}

// attach hands a deferred load's lazy bodies to index: the mapping lease and
// the refusal record.
func (bodies *snapshotBodies) attach(index *Index) {
	if bodies == nil || bodies.lazy == nil {
		return
	}
	index.lease, index.bodies = bodies.lease, bodies.lazy.sink
}

// end releases the lease of a decode whose index was not built.
func (bodies *snapshotBodies) end() {
	if bodies != nil {
		bodies.lease.release()
	}
}

// gobBodies backs every lazy body of one deferred load.
type gobBodies struct {
	region       []byte
	objectFormat string
	sink         *packSection // the load's SnapshotRefusal record
	cache        []atomic.Pointer[[]byte]
}

var errSnapshotBody = errors.New("snapshot body does not match its blob id")

// read returns body ordinal's verified heap copy, copying and verifying it on
// first use.
func (bodies *gobBodies) read(ordinal int, offset, length uint64, blobHash string) ([]byte, error) {
	if cached := bodies.cache[ordinal].Load(); cached != nil {
		return *cached, nil
	}
	data, err := copyRegionBody(bodies.region, offset, length, true)
	if err != nil {
		return nil, err
	}
	if gitBlobHash(data, bodies.objectFormat) != blobHash {
		return nil, errSnapshotBody
	}
	bodies.cache[ordinal].Store(&data)
	return data, nil
}

// copyRegionBody copies region[offset:offset+length] to the heap, under the
// fault guard when the region is a mapping.
func copyRegionBody(region []byte, offset, length uint64, mapped bool) ([]byte, error) {
	if offset > uint64(len(region)) || length > uint64(len(region))-offset {
		return nil, errSnapshotChunks
	}
	data := make([]byte, length)
	if !mapped {
		copy(data, region[offset:offset+length])
		return data, nil
	}
	err := guardMappedRead(func() error {
		copy(data, region[offset:offset+length])
		return nil
	})
	return data, err
}

// rawBytes is the verified bytes of a lazy gob body, the same bytes a full
// read stores in Source.Data, for a reader that needs bytes rather than
// valid text (IDX-SNAP-V0-813). A body that faults or fails its blob id is
// recorded on the load's refusal and reported unavailable.
func (body packBody) rawBytes(source Source) ([]byte, bool) {
	data, err := body.gob.read(body.ordinal, body.offset, body.length, source.BlobHash)
	if err != nil {
		body.gob.sink.failure.CompareAndSwap(nil, &err)
		return nil, false
	}
	return data, true
}

// gobText is Source.Text over a lazy body: a body that faults or fails its
// blob id is recorded on the load's refusal and reported unloaded.
func (body packBody) gobText(source Source) (string, bool, bool) {
	data, err := body.gob.read(body.ordinal, body.offset, body.length, source.BlobHash)
	if err != nil {
		body.gob.sink.failure.CompareAndSwap(nil, &err)
		return "", false, false
	}
	source.Data, source.body = data, packBody{}
	return source.Text()
}

// gobSourceMetas splits values into the /4 metadata and the region parts, in
// order: a body is in the region when it rehashes to its BlobHash under
// objectFormat.
func gobSourceMetas(values []Source, objectFormat string) ([]wireSourceMeta, [][]byte) {
	metas := make([]wireSourceMeta, len(values))
	var region [][]byte
	for position, source := range values {
		meta := wireSourceMeta{Path: source.Path, BlobHash: source.BlobHash, Mode: source.Mode, Checked: source.Checked, Valid: source.Valid, Length: -1}
		if len(source.Data) != 0 {
			meta.Length = len(source.Data)
			if source.BlobHash != "" && gitBlobHash(source.Data, objectFormat) == source.BlobHash {
				meta.InRegion = true
				region = append(region, source.Data)
			} else {
				meta.Inline = source.Data
			}
		}
		metas[position] = meta
	}
	return metas, region
}

// regionFooter is the 8-byte little-endian region length.
func regionFooter(length int64) []byte {
	return binary.LittleEndian.AppendUint64(nil, uint64(length))
}

// sourcesFromMetas rebuilds the sources of metas over bodies: eagerly copied
// and verified, or as lazy bodies on a deferred mapped load.
func sourcesFromMetas(metas []wireSourceMeta, bodies *snapshotBodies) ([]Source, error) {
	var region []byte
	mapped, lazy := false, false
	if bodies != nil {
		region, mapped = bodies.region, bodies.mapping != nil
		lazy = bodies.deferred && mapped
	}
	var shared *gobBodies
	if lazy {
		ordinals := 0
		for _, meta := range metas {
			if meta.InRegion {
				ordinals++
			}
		}
		shared = &gobBodies{region: region, objectFormat: bodies.objectFormat, sink: &packSection{name: "gob bodies"}, cache: make([]atomic.Pointer[[]byte], ordinals)}
	}
	sources := make([]Source, len(metas))
	var offset uint64
	ordinal := 0
	for position, meta := range metas {
		source := Source{Path: meta.Path, BlobHash: meta.BlobHash, Mode: meta.Mode, Checked: meta.Checked, Valid: meta.Valid}
		switch {
		case meta.Length < -1 || meta.Length == 0 || (meta.Length == -1 && (meta.InRegion || meta.Inline != nil)):
			return nil, errSnapshotChunks
		case meta.Length == -1:
		case !meta.InRegion:
			if len(meta.Inline) != meta.Length {
				return nil, errSnapshotChunks
			}
			source.Data = meta.Inline
		default:
			length := uint64(meta.Length)
			if offset > uint64(len(region)) || length > uint64(len(region))-offset {
				return nil, errSnapshotChunks
			}
			if lazy {
				source.body = packBody{offset: offset, length: length, gob: shared, ordinal: ordinal}
			} else {
				data, err := copyRegionBody(region, offset, length, mapped)
				if err != nil {
					return nil, err
				}
				if gitBlobHash(data, bodies.objectFormat) != meta.BlobHash {
					return nil, errSnapshotBody
				}
				source.Data = data
			}
			offset += length
			ordinal++
		}
		sources[position] = source
	}
	if offset != uint64(len(region)) {
		return nil, errSnapshotChunks
	}
	if lazy {
		bodies.lazy = shared
	}
	return sources, nil
}
