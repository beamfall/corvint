package journal

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"math"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/archive"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// ProfileWriterBase names the sealed base of the writer checkpoint
// (JSG-V0-004). It holds the canonical inventory, the request index and the
// note references at its sequence, in fixed-width records a writer searches
// without decoding. The delta (ProfileWriterCheckpoint) binds it by SHA-256;
// like the delta it is derived state, and an absent or differing base only
// sends the next writer through the complete audit.
//
// Version 1 appends the checkpoint header and the aggregates at the base's
// sequence and the SHA-256 of every sealed segment it covers, so a seal can
// resume the complete validators from the base (JSG-V0-009). A version 0
// base is refused as another version and the writer takes the complete route.
const ProfileWriterBase = "taskman-writer-base/1"

const baseMagic = ProfileWriterBase + "\n"

// WriterBaseTail is the most receipts a delta carries past its base before a
// writer rewrites the base from the walked view (JSG-V0-004, before a store
// is segmented).
const WriterBaseTail = 512

// baseHeaderBytes is the magic and five counts: sequence, entries, requests,
// notes and path bytes.
const baseHeaderBytes = len(baseMagic) + 5*8

// baseRecordBytes is one entry record: path offset (uint32), path length
// (uint16), flags (bit 0: live), a zero pad byte, sequence, afterimage size
// and digest.
const baseRecordBytes = 4 + 2 + 1 + 1 + 8 + 8 + sha256.Size

// maxBasePathBytes bounds one entry path.
const maxBasePathBytes = 1 << 12

// maxBaseCheckpointBytes bounds the encoded checkpoint header in the base.
const maxBaseCheckpointBytes = 16 * wire.KiB

// baseAggregates is the base's aggregates at its sequence: receipt bytes,
// the four archive cost fields, then the five intent aggregate fields.
const baseAggregates = 10

// maxBaseSegments bounds the sealed segment digests a base carries.
const maxBaseSegments = wire.MaxArchiveFiles/snapshot.SegmentReceipts + 1

// MaxWriterBaseBytes bounds the base a writer is willing to consume. It is
// an int64 so the bound compiles on 32-bit targets too (V1-3581), where it
// exceeds int; MaxWriterBaseReadBytes is the bound a read can apply there.
const MaxWriterBaseBytes = int64(baseHeaderBytes) + (baseRecordBytes+maxBasePathBytes)*maxWriterEntries + writerRequestBytes*wire.MaxArchiveFiles + writerNoteBytes*wire.MaxTicketsPerQueue + 8 + maxBaseCheckpointBytes + baseAggregates*8 + 8 + sha256.Size*maxBaseSegments

// MaxWriterBaseReadBytes is MaxWriterBaseBytes clamped to the platform int.
const MaxWriterBaseReadBytes = int(min(MaxWriterBaseBytes, math.MaxInt))

// WriterBasePath is the sibling of the writer checkpoint that holds its base.
func WriterBasePath(stateDir string) string { return WriterCheckpointPath(stateDir) + ".base" }

// baseTable is a decoded base. Decoding checks its layout, path order and
// counts in one pass that allocates only the path text; entries are turned
// into latest values, and their paths checked against PostBound, only where
// a walk uses them.
type baseTable struct {
	sha    [sha256.Size]byte
	seq    uint64
	n      int
	recs   []byte
	paths  string
	reqs   []byte
	notes  []byte
	raw    []byte
	intent struct{ tickets, releases, payload uint64 }
	// meta is the checkpoint the base was rendered from, without entries,
	// sizes, requests or notes: its header and aggregates at seq, and the
	// sealed segments it covers (JSG-V0-009).
	meta baseMeta
}

// baseMeta is a base's checkpoint header, aggregates and segment digests.
type baseMeta struct {
	header       Checkpoint
	receiptBytes uint64
	cost         archive.FileSetCost
	intent       IntentAggregate
	segments     [][sha256.Size]byte
}

func (b *baseTable) rec(i int) []byte { return b.recs[i*baseRecordBytes : (i+1)*baseRecordBytes] }

func (b *baseTable) path(i int) string {
	r := b.rec(i)
	off, n := binary.BigEndian.Uint32(r), binary.BigEndian.Uint16(r[4:])
	return b.paths[off : off+uint32(n)]
}

func (b *baseTable) live(i int) bool { return b.rec(i)[6]&1 != 0 }

func (b *baseTable) size(i int) uint64 { return binary.BigEndian.Uint64(b.rec(i)[16:]) }

// search returns the index of p, or -1.
func (b *baseTable) search(p string) int {
	i := b.seek(p)
	if i < b.n && b.path(i) == p {
		return i
	}
	return -1
}

// seek returns the first index whose path is at or after p.
func (b *baseTable) seek(p string) int {
	return sort.Search(b.n, func(i int) bool { return b.path(i) >= p })
}

// entry is record i as a latest, with its path checked and its size bounded
// as the decoder of an entry table does.
func (b *baseTable) entry(i int) (latest, error) {
	r, p := b.rec(i), b.path(i)
	bound, err := snapshot.PostBound(p)
	if err != nil {
		return latest{}, errCheckpoint(p, "base entry path: "+err.Error())
	}
	size := b.size(i)
	if size > uint64(bound) {
		return latest{}, errCheckpoint(p, "base entry size")
	}
	l := latest{seq: wire.SizeOf(binary.BigEndian.Uint64(r[8:])), bytes: int64(size)}
	if b.live(i) {
		d := wire.Digest(hex.EncodeToString(r[24:]))
		l.digest = &d
	}
	return l, nil
}

func (b *baseTable) requests() uint64 { return uint64(len(b.reqs) / writerRequestBytes) }

// request returns the afterimage digest the base holds for path digest d.
func (b *baseTable) request(d []byte) ([]byte, bool) { return findRequest(b.reqs, d) }

// note returns the note digest of entry i, if it carries one.
func (b *baseTable) note(i int) ([sha256.Size]byte, bool) {
	k := len(b.notes) / writerNoteBytes
	j := sort.Search(k, func(j int) bool { return binary.BigEndian.Uint32(b.notes[j*writerNoteBytes:]) >= uint32(i) })
	var d [sha256.Size]byte
	if j == k || binary.BigEndian.Uint32(b.notes[j*writerNoteBytes:]) != uint32(i) {
		return d, false
	}
	copy(d[:], b.notes[j*writerNoteBytes+4:(j+1)*writerNoteBytes])
	return d, true
}

// findRequest binary-searches strictly ordered request records for path
// digest d and returns its afterimage digest.
func findRequest(reqs, d []byte) ([]byte, bool) {
	n := len(reqs) / writerRequestBytes
	i := sort.Search(n, func(i int) bool {
		return bytes.Compare(reqs[i*writerRequestBytes:i*writerRequestBytes+sha256.Size], d) >= 0
	})
	if i == n || !bytes.Equal(reqs[i*writerRequestBytes:i*writerRequestBytes+sha256.Size], d) {
		return nil, false
	}
	return reqs[i*writerRequestBytes+sha256.Size : (i+1)*writerRequestBytes], true
}

// encodeBase renders the base of a full (base-free) checkpoint w.
func encodeBase(w *WriterCheckpoint) ([]byte, error) {
	pathBytes := 0
	for _, e := range w.Entries {
		if len(e.Path) == 0 || len(e.Path) > maxBasePathBytes {
			return nil, errCheckpoint(e.Path, "base entry path length")
		}
		pathBytes += len(e.Path)
	}
	header := w.Checkpoint
	header.Entries = nil
	cp := header.Encode()
	if len(cp) > maxBaseCheckpointBytes || len(w.segments) > maxBaseSegments {
		return nil, errCheckpoint("writer checkpoint base", "header or segments exceed the base bound")
	}
	buf := make([]byte, 0, baseHeaderBytes+baseRecordBytes*len(w.Entries)+pathBytes+len(w.requests)+writerNoteBytes*len(w.notes)+8+len(cp)+baseAggregates*8+8+sha256.Size*len(w.segments))
	buf = append(buf, baseMagic...)
	for _, v := range []uint64{w.Seq.Uint64(), uint64(len(w.Entries)), w.Requests(), uint64(len(w.notes)), uint64(pathBytes)} {
		buf = binary.BigEndian.AppendUint64(buf, v)
	}
	off := 0
	for i, e := range w.Entries {
		buf = binary.BigEndian.AppendUint32(buf, uint32(off))
		buf = binary.BigEndian.AppendUint16(buf, uint16(len(e.Path)))
		off += len(e.Path)
		var digest []byte
		if e.Sha256 != nil {
			d, err := hex.DecodeString(string(*e.Sha256))
			if err != nil || len(d) != sha256.Size {
				return nil, errCheckpoint(e.Path, "entry digest is not canonical")
			}
			buf, digest = append(buf, 1, 0), d
		} else {
			buf, digest = append(buf, 0, 0), make([]byte, sha256.Size)
		}
		buf = binary.BigEndian.AppendUint64(buf, e.Seq.Uint64())
		buf = binary.BigEndian.AppendUint64(buf, w.sizes[i])
		buf = append(buf, digest...)
	}
	for _, e := range w.Entries {
		buf = append(buf, e.Path...)
	}
	buf = append(buf, w.requests...)
	for _, n := range w.notes {
		buf = binary.BigEndian.AppendUint32(buf, n.entry)
		buf = append(buf, n.digest[:]...)
	}
	buf = binary.BigEndian.AppendUint64(buf, uint64(len(cp)))
	buf = append(buf, cp...)
	for _, v := range []uint64{w.ReceiptBytes, uint64(w.Cost.Files), w.Cost.PayloadBytes, w.Cost.EntryBytes, w.Cost.TarBytes, uint64(w.Intent.Tickets), uint64(w.Intent.Releases), w.Intent.Cost.PayloadBytes, w.Intent.Cost.EntryBytes, w.Intent.Cost.TarBytes} {
		buf = binary.BigEndian.AppendUint64(buf, v)
	}
	buf = binary.BigEndian.AppendUint64(buf, uint64(len(w.segments)))
	for _, d := range w.segments {
		buf = append(buf, d[:]...)
	}
	return buf, nil
}

// decodeBase checks and indexes base bytes: the exact length, contiguous
// strictly ordered paths, flags, sequences at or before the base, a deletion
// with neither size nor digest, no request paths, strictly ordered requests,
// and notes on live tickets in entry order. It also sums the live ticket and
// release records the delta's intent aggregate is checked against, and
// checks the trailing checkpoint header, aggregates and segment digests
// against the table: the header at the base's sequence, the intent
// aggregate equal to the sums, one file per receipt and request, and
// segments only over receipts the base covers.
func decodeBase(raw []byte) (*baseTable, error) {
	bad := func(why string) error { return errCheckpoint("writer checkpoint base", why) }
	if int64(len(raw)) > MaxWriterBaseBytes {
		return nil, bad("larger than its bound")
	}
	if name, _, _ := strings.Cut(ProfileWriterBase, "/"); bytes.HasPrefix(raw, []byte(name+"/")) && !bytes.HasPrefix(raw, []byte(baseMagic)) {
		return nil, wire.Errorf(wire.CodeUnsupportedVersion, "writer checkpoint base", "another %s version", name)
	}
	if len(raw) < baseHeaderBytes || !bytes.HasPrefix(raw, []byte(baseMagic)) {
		return nil, bad("profile differs or truncated")
	}
	var h [5]uint64
	for i := range h {
		h[i] = binary.BigEndian.Uint64(raw[len(baseMagic)+8*i:])
	}
	seq, n, nreq, nnotes, pathBytes := h[0], h[1], h[2], h[3], h[4]
	if seq < 1 || n > uint64(maxCheckpointEntries()) || nreq > uint64(wire.MaxArchiveFiles) || nnotes > uint64(wire.MaxTicketsPerQueue) || pathBytes > n*maxBasePathBytes {
		return nil, bad("counts out of range")
	}
	table := n*baseRecordBytes + pathBytes + nreq*writerRequestBytes + nnotes*writerNoteBytes
	if uint64(len(raw)-baseHeaderBytes) < table+8 {
		return nil, bad("length differs from its counts")
	}
	b := &baseTable{sha: sha256.Sum256(raw), seq: seq, n: int(n), raw: raw}
	rest := raw[baseHeaderBytes:]
	b.recs, rest = rest[:n*baseRecordBytes], rest[n*baseRecordBytes:]
	b.paths, rest = string(rest[:pathBytes]), rest[pathBytes:]
	b.reqs, rest = rest[:nreq*writerRequestBytes], rest[nreq*writerRequestBytes:]
	b.notes, rest = rest[:nnotes*writerNoteBytes], rest[nnotes*writerNoteBytes:]
	var zero [sha256.Size]byte
	off := uint64(0)
	for i := 0; i < b.n; i++ {
		r := b.rec(i)
		l := uint64(binary.BigEndian.Uint16(r[4:]))
		if uint64(binary.BigEndian.Uint32(r)) != off || l == 0 || off+l > pathBytes {
			return nil, bad("entry paths are not contiguous")
		}
		off += l
		p := b.path(i)
		if i > 0 && b.path(i-1) >= p {
			return nil, bad("entries are not strictly path-ordered")
		}
		if s := binary.BigEndian.Uint64(r[8:]); s < 1 || s > seq || r[6] > 1 || r[7] != 0 || strings.HasPrefix(p, "requests/") {
			return nil, bad("entry outside the base")
		}
		if r[6] == 0 {
			if binary.BigEndian.Uint64(r[16:]) != 0 || !bytes.Equal(r[24:], zero[:]) {
				return nil, bad("a deletion carries a size or digest")
			}
			continue
		}
		switch {
		case strings.HasPrefix(p, "intent/tickets/"):
			b.intent.tickets++
		case strings.HasPrefix(p, "intent/releases/"):
			b.intent.releases++
		default:
			continue
		}
		b.intent.payload += binary.BigEndian.Uint64(r[16:])
	}
	if off != pathBytes {
		return nil, bad("entry paths are not contiguous")
	}
	if err := orderedRequests(b.reqs); err != nil {
		return nil, err
	}
	for j := 0; j < int(nnotes); j++ {
		e := binary.BigEndian.Uint32(b.notes[j*writerNoteBytes:])
		if (j > 0 && e <= binary.BigEndian.Uint32(b.notes[(j-1)*writerNoteBytes:])) || uint64(e) >= n {
			return nil, bad("note entries are not strictly ordered base entries")
		}
		if !b.live(int(e)) || !strings.HasPrefix(b.path(int(e)), "intent/tickets/") {
			return nil, bad("note entry is not a live ticket")
		}
	}
	if err := b.decodeMeta(rest); err != nil {
		return nil, err
	}
	return b, nil
}

// decodeMeta checks the base's trailing checkpoint header, aggregates and
// segment digests; see decodeBase.
func (b *baseTable) decodeMeta(rest []byte) error {
	bad := func(why string) error { return errCheckpoint("writer checkpoint base", why) }
	n := binary.BigEndian.Uint64(rest)
	rest = rest[8:]
	if n > maxBaseCheckpointBytes || uint64(len(rest)) < n+baseAggregates*8+8 {
		return bad("checkpoint header length")
	}
	cp, err := decodeEmbeddedCheckpoint(rest[:n])
	if err != nil {
		return err
	}
	if cp.Seq.Uint64() != b.seq || len(cp.Entries) != 0 {
		return bad("checkpoint header is not at the base")
	}
	rest = rest[n:]
	var v [baseAggregates]uint64
	for i := range v {
		v[i] = binary.BigEndian.Uint64(rest[8*i:])
	}
	rest = rest[baseAggregates*8:]
	nseg := binary.BigEndian.Uint64(rest)
	rest = rest[8:]
	if nseg > maxBaseSegments || nseg*snapshot.SegmentReceipts > b.seq || uint64(len(rest)) != nseg*sha256.Size {
		return bad("segment digests")
	}
	if v[1] != b.seq+b.requests() || v[0] > v[2] || v[5] != b.intent.tickets || v[6] != b.intent.releases || v[7] != b.intent.payload {
		return bad("aggregates differ from the table")
	}
	m := &b.meta
	m.header, m.receiptBytes = *cp, v[0]
	m.cost = archive.FileSetCost{Files: int(v[1]), PayloadBytes: v[2], EntryBytes: v[3], TarBytes: v[4]}
	m.intent = IntentAggregate{Tickets: int(v[5]), Releases: int(v[6]), Cost: archive.FileSetCost{Files: int(v[5] + v[6]), PayloadBytes: v[7], EntryBytes: v[8], TarBytes: v[9]}}
	m.segments = make([][sha256.Size]byte, nseg)
	for i := range m.segments {
		copy(m.segments[i][:], rest[i*sha256.Size:])
	}
	return nil
}

// bindsDelta checks a base against the delta that binds it: the same queue,
// worktree and init, aggregates that never shrink from the base to the
// delta, and at the base's own sequence the same header and aggregates.
// v is the delta's seven aggregates and iv its intent aggregate.
func (b *baseTable) bindsDelta(cp *Checkpoint, v, iv []uint64) error {
	bad := func(why string) error { return errCheckpoint("writer checkpoint", why) }
	h := &b.meta.header
	if h.QueueID != cp.QueueID || h.PrimaryWorktree != cp.PrimaryWorktree || h.InitSha256 != cp.InitSha256 || h.Generation.Uint64() > cp.Generation.Uint64() {
		return bad("base header differs from the delta's")
	}
	m := &b.meta
	if m.receiptBytes > v[1] || uint64(m.cost.Files) > v[2] || m.cost.PayloadBytes > v[3] || m.cost.EntryBytes > v[4] || m.cost.TarBytes > v[5] {
		return bad("base aggregates exceed the delta's")
	}
	if b.seq != cp.Seq.Uint64() {
		return nil
	}
	if h.ReceiptSha256 != cp.ReceiptSha256 || h.Generation != cp.Generation || h.SemanticCoverage != cp.SemanticCoverage || m.receiptBytes != v[1] || uint64(m.cost.Files) != v[2] || m.cost.PayloadBytes != v[3] || m.cost.EntryBytes != v[4] || m.cost.TarBytes != v[5] {
		return bad("base differs from the delta at its own sequence")
	}
	if uint64(m.intent.Tickets) != iv[0] || uint64(m.intent.Releases) != iv[1] || m.intent.Cost.PayloadBytes != iv[2] || m.intent.Cost.EntryBytes != iv[3] || m.intent.Cost.TarBytes != iv[4] {
		return bad("base intent aggregate differs from the delta's")
	}
	return nil
}
