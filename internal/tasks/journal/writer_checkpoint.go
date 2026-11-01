package journal

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"math"
	"os"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/archive"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// ProfileWriterCheckpoint names the derived writer checkpoint (CAL-V0-115,
// proposed). Like the read checkpoint it is not journal state: no receipt
// posts it, no archive carries it, and a missing, stale, torn or foreign file
// only sends the next writer through the complete audit.
//
// Version 2 added the per-entry afterimage sizes and the live intent record
// aggregate a liveness write charges instead of listing the intent tree
// (JSG-V0-001). Version 3 is the delta over a sealed base
// (ProfileWriterBase) that it binds by SHA-256, so a writer decodes only the
// entries its walk uses (JSG-V0-004). Earlier versions are refused as
// another version.
const ProfileWriterCheckpoint = "taskman-writer-checkpoint/3"

// ModeWriter is Result.Mode for a writer audit that resumed at a writer
// checkpoint and walked only the receipts after it (CAL-V0-116, proposed).
const ModeWriter = "WRITER_CHECKPOINT_PLUS_TAIL"

const (
	// MaxWriterTail is the most receipts a writer walks after its checkpoint.
	MaxWriterTail = 256
	// WriterAdvanceTail is the tail at which a writer re-bases the checkpoint
	// on what it just walked.
	WriterAdvanceTail = 64
	// WriterRefreshInterval is the receipt distance between scheduled
	// complete audits that re-derive the checkpoint from receipt 1.
	WriterRefreshInterval = 512
	// WriterFullBound is the most receipts any writer accepts since the last
	// complete audit the checkpoint descends from.
	WriterFullBound = 4096
	// WriterSegmentedBound replaces WriterFullBound on a segmented store,
	// where each seal re-runs the complete validators over the receipts it
	// seals and advances FullSeq to them (JSG-V0-010): two segments.
	WriterSegmentedBound = 2 * snapshot.SegmentReceipts
)

const writerMagic = ProfileWriterCheckpoint + "\n"

// MaxWriterCheckpointBytes bounds the file a writer is willing to consume.
const MaxWriterCheckpointBytes = len(writerMagic) + sha256.Size + 8 + MaxCheckpointBytes + 7*8 + writerIntentFields*8 + 8*maxWriterEntries + writerRequestBytes*wire.MaxArchiveFiles + 8 + writerNoteBytes*wire.MaxTicketsPerQueue + 2*sha256.Size

// writerIntentFields is the encoded IntentAggregate: tickets, releases, and
// the payload, entry and tar bytes of their archive cost.
const writerIntentFields = 5

// maxWriterEntries is maxCheckpointEntries as a constant: one size each.
const maxWriterEntries = wire.MaxArchiveScanEntries + intent.MaxIntentRootEntries + wire.MaxTicketsPerQueue + wire.MaxReleasesPerQueue

// writerRequestBytes is one encoded request: the SHA-256 of its path, then
// the SHA-256 of its afterimage (CAL-V0-190, proposed).
const writerRequestBytes = 2 * sha256.Size

// writerNoteBytes is one encoded note: a uint32 entry index and a digest.
const writerNoteBytes = 4 + sha256.Size

// WriterCheckpointPath is a sibling of the state directory, like
// CheckpointPath, so state scans, archive export and older runtimes never see
// it. Deleting it forces the next writer through the complete audit.
func WriterCheckpointPath(stateDir string) string { return stateDir + ".writer-checkpoint" }

// WriterCheckpoint is what a complete settled audit established through Seq,
// plus the aggregates a writer needs about receipts 1..Seq and their request
// afterimages without listing them: their archive cost and the sorted request
// path digests a new request must not repeat. A writer rebinds the named
// receipt and every entry before use; the aggregates are trusted derived
// state, re-derived from receipt 1 by every complete audit (CAL-V0-117).
// It also carries the operator-note reference of every live ticket that has
// one at Seq, which the tail binds as the complete audit does (ON-V0-006),
// and the digest of the invalidation token it was published under.
type WriterCheckpoint struct {
	Checkpoint
	// FullSeq is the head of the complete audit this checkpoint descends from.
	FullSeq uint64
	// ReceiptBytes is the payload of receipts 1..Seq.
	ReceiptBytes uint64
	// Cost is the archive cost of receipts 1..Seq and of every request
	// afterimage they posted.
	Cost archive.FileSetCost
	// Invalidation is the SHA-256 of the store's invalidation token when
	// this checkpoint's audit began. The journal only carries it: the writer
	// uses the checkpoint only while the token still has this digest
	// (CAL-V0-117, proposed).
	Invalidation [sha256.Size]byte
	// Intent is the count and archive cost of the live canonical ticket and
	// release records at Seq (JSG-V0-001).
	Intent   IntentAggregate
	sizes    []uint64 // afterimage size of Entries[i]; 0 for a deletion
	requests []byte   // writerRequestBytes records, strictly ordered by path digest
	notes    []writerNote
	// base, when set, holds every entry, request and note the delta does
	// not: Entries, sizes, requests and notes are then only the delta's, and
	// total counts the merged entries (JSG-V0-004).
	base  *baseTable
	total int
	// freshBase marks a base a seal rebuilt at its new sequence: Encode
	// writes it before the delta although w carries it (JSG-V0-009).
	freshBase bool
	// segments is the SHA-256 of every sealed segment at Seq, which a fresh
	// base records (JSG-V0-009).
	segments [][sha256.Size]byte
}

// IntentAggregate is the count and archive cost of the live canonical
// ticket and release records at a sequence. A liveness write charges it in
// place of listing intent/tickets/ and intent/releases/ (JSG-V0-001).
type IntentAggregate struct {
	Tickets, Releases int
	Cost              archive.FileSetCost
}

// intentRecord reports a path the IntentAggregate covers.
func intentRecord(p string) bool {
	return strings.HasPrefix(p, "intent/tickets/") || strings.HasPrefix(p, "intent/releases/")
}

// search returns the index of p in Entries, or -1.
func (w *WriterCheckpoint) search(p string) int {
	e := sort.Search(len(w.Entries), func(i int) bool { return w.Entries[i].Path >= p })
	if e == len(w.Entries) || w.Entries[e].Path != p {
		return -1
	}
	return e
}

// deltaLatest is Entries[e] as a latest; its size is -1 when the
// checkpoint carries no sizes (a retired-prefix anchor).
func (w *WriterCheckpoint) deltaLatest(e int) latest {
	size := int64(-1)
	if len(w.sizes) == len(w.Entries) {
		size = int64(w.sizes[e])
	}
	return latest{seq: w.Entries[e].Seq, digest: w.Entries[e].Sha256, bytes: size}
}

// lookup returns the latest canonical afterimage of p at Seq: the delta's
// entry, else the base's, whose path and size are checked here, where a walk
// first uses them (JSG-V0-004).
func (w *WriterCheckpoint) lookup(p string) (latest, bool, error) {
	if e := w.search(p); e >= 0 {
		return w.deltaLatest(e), true, nil
	}
	if w.base != nil {
		if i := w.base.search(p); i >= 0 {
			l, err := w.base.entry(i)
			return l, err == nil, err
		}
	}
	return latest{}, false, nil
}

// holds reports whether p has a latest entry at Seq, live or deleted.
func (w *WriterCheckpoint) holds(p string) bool {
	return w.search(p) >= 0 || (w.base != nil && w.base.search(p) >= 0)
}

// liveIntent reports whether p is a live ticket or release record at Seq.
func (w *WriterCheckpoint) liveIntent(p string) bool {
	if !intentRecord(p) {
		return false
	}
	if e := w.search(p); e >= 0 {
		return w.Entries[e].Sha256 != nil
	}
	if w.base != nil {
		if i := w.base.search(p); i >= 0 {
			return w.base.live(i)
		}
	}
	return false
}

// count is the number of latest entries at Seq, request paths excluded.
func (w *WriterCheckpoint) count() int {
	if w.base == nil {
		return len(w.Entries)
	}
	return w.total
}

// entrySize returns the afterimage size carried for p at Seq and whether p
// is a live entry.
func (w *WriterCheckpoint) entrySize(p string) (uint64, bool) {
	if e := w.search(p); e >= 0 {
		if w.Entries[e].Sha256 == nil || len(w.sizes) != len(w.Entries) {
			return 0, false
		}
		return w.sizes[e], true
	}
	if w.base != nil {
		if i := w.base.search(p); i >= 0 && w.base.live(i) {
			return w.base.size(i), true
		}
	}
	return 0, false
}

// each calls fn for every latest entry at Seq in path order, the delta's
// entry in place of the base's, skipping every path under a prefix in skip
// (each ends in '/') without visiting it.
func (w *WriterCheckpoint) each(skip []string, fn func(string, latest) error) error {
	nb := 0
	if w.base != nil {
		nb = w.base.n
	}
	d, b := 0, 0
	for {
		for d < len(w.Entries) {
			pfx := skipped(w.Entries[d].Path, skip)
			if pfx == "" {
				break
			}
			past := pastPrefix(pfx)
			d = sort.Search(len(w.Entries), func(i int) bool { return w.Entries[i].Path >= past })
		}
		for b < nb {
			pfx := skipped(w.base.path(b), skip)
			if pfx == "" {
				break
			}
			b = w.base.seek(pastPrefix(pfx))
		}
		if d == len(w.Entries) && b == nb {
			return nil
		}
		var p string
		var l latest
		if b == nb || (d < len(w.Entries) && w.Entries[d].Path <= w.base.path(b)) {
			p, l = w.Entries[d].Path, w.deltaLatest(d)
			if b < nb && w.base.path(b) == p {
				b++
			}
			d++
		} else {
			p = w.base.path(b)
			var err error
			if l, err = w.base.entry(b); err != nil {
				return err
			}
			b++
		}
		if err := fn(p, l); err != nil {
			return err
		}
	}
}

// skipped returns the prefix in skip that p falls under, or "".
func skipped(p string, skip []string) string {
	for _, pfx := range skip {
		if strings.HasPrefix(p, pfx) {
			return pfx
		}
	}
	return ""
}

// pastPrefix is the least path after every path under pfx, which ends in '/'.
func pastPrefix(pfx string) string { return pfx[:len(pfx)-1] + "0" }

// full returns w with its base merged in: every entry, size, request and
// note in one base-free checkpoint. A base-free w is returned as it is.
func (w *WriterCheckpoint) full() (*WriterCheckpoint, error) {
	if w.base == nil {
		return w, nil
	}
	f := *w
	f.base, f.total = nil, 0
	f.Checkpoint.Entries = make([]CheckpointEntry, 0, w.total)
	f.sizes = make([]uint64, 0, w.total)
	f.notes = nil
	err := w.each(nil, func(p string, l latest) error {
		f.Entries = append(f.Entries, CheckpointEntry{Path: p, Seq: l.seq, Sha256: l.digest})
		f.sizes = append(f.sizes, uint64(l.bytes))
		if d, ok := w.note(p); ok {
			f.notes = append(f.notes, writerNote{entry: uint32(len(f.Entries) - 1), digest: d})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	f.requests = mergeRequests(w.base.reqs, w.requests)
	return &f, nil
}

// mergeRequests merges two strictly ordered, disjoint request record sets.
func mergeRequests(a, b []byte) []byte {
	out := make([]byte, 0, len(a)+len(b))
	for len(a) > 0 || len(b) > 0 {
		if len(b) == 0 || (len(a) > 0 && bytes.Compare(a[:sha256.Size], b[:sha256.Size]) < 0) {
			out, a = append(out, a[:writerRequestBytes]...), a[writerRequestBytes:]
		} else {
			out, b = append(out, b[:writerRequestBytes]...), b[writerRequestBytes:]
		}
	}
	return out
}

// orderedRequests refuses request records that are not strictly ordered by
// path digest.
func orderedRequests(reqs []byte) error {
	for i := writerRequestBytes; i < len(reqs); i += writerRequestBytes {
		if bytes.Compare(reqs[i-writerRequestBytes:i-sha256.Size], reqs[i:i+sha256.Size]) >= 0 {
			return errCheckpoint("writer checkpoint", "request digests are not strictly ordered")
		}
	}
	return nil
}

// decodeNotes parses note records over entries: strictly ordered entry
// indexes, each a live ticket.
func decodeNotes(raw []byte, entries []CheckpointEntry) ([]writerNote, error) {
	notes := make([]writerNote, len(raw)/writerNoteBytes)
	for i := range notes {
		n := &notes[i]
		n.entry = binary.BigEndian.Uint32(raw[i*writerNoteBytes:])
		copy(n.digest[:], raw[i*writerNoteBytes+4:(i+1)*writerNoteBytes])
		if (i > 0 && n.entry <= notes[i-1].entry) || n.entry >= uint32(len(entries)) {
			return nil, errCheckpoint("writer checkpoint", "note entries are not strictly ordered checkpoint entries")
		}
		if e := entries[n.entry]; !strings.HasPrefix(e.Path, "intent/tickets/") || e.Sha256 == nil {
			return nil, errCheckpoint("writer checkpoint", "note entry is not a live ticket")
		}
	}
	return notes, nil
}

// writerNote is the SHA-256 of the encoded operator-note reference of the
// ticket at Entries[entry]; notes are strictly ordered by entry.
type writerNote struct {
	entry  uint32
	digest [sha256.Size]byte
}

func noteDigest(ref *ticket.OperatorNoteReference) [sha256.Size]byte {
	return sha256.Sum256(wire.Encode(ref.Value()))
}

// note returns the digest of the note reference the ticket at p carried at
// Seq, if it carried one: the delta's when the delta holds p, else the
// base's.
func (w *WriterCheckpoint) note(p string) ([sha256.Size]byte, bool) {
	if e := w.search(p); e >= 0 {
		i := sort.Search(len(w.notes), func(i int) bool { return w.notes[i].entry >= uint32(e) })
		if i == len(w.notes) || w.notes[i].entry != uint32(e) {
			return [sha256.Size]byte{}, false
		}
		return w.notes[i].digest, true
	}
	if w.base != nil {
		if i := w.base.search(p); i >= 0 {
			return w.base.note(i)
		}
	}
	return [sha256.Size]byte{}, false
}

// sameNote reports whether ref is the note reference the ticket at p carried
// at Seq; nil matches a ticket that carried none.
func (w *WriterCheckpoint) sameNote(p string, ref *ticket.OperatorNoteReference) bool {
	d, ok := w.note(p)
	if ref == nil {
		return !ok
	}
	return ok && d == noteDigest(ref)
}

// LastPosted returns the receipt that last posted non-request path p at or
// before Seq, from the delta or its base; zero when none did.
func (w *WriterCheckpoint) LastPosted(p string) (uint64, error) {
	l, ok, err := w.lookup(p)
	if err != nil || !ok {
		return 0, err
	}
	return l.seq.Uint64(), nil
}

// Requests is the number of request afterimages receipts 1..Seq posted.
func (w *WriterCheckpoint) Requests() uint64 {
	n := uint64(len(w.requests) / writerRequestBytes)
	if w.base != nil {
		n += w.base.requests()
	}
	return n
}

// HasRequest reports whether p is a request path posted at or before Seq.
func (w *WriterCheckpoint) HasRequest(p string) bool {
	_, ok := w.RequestAfterimage(p)
	return ok
}

// RequestAfterimage returns the digest of the latest afterimage of request
// path p that receipts 1..Seq posted, as the audit deriving the checkpoint
// read it (CAL-V0-190, proposed). The base's records are searched in place.
func (w *WriterCheckpoint) RequestAfterimage(p string) (wire.Digest, bool) {
	d, ok := requestDigest(p)
	if !ok {
		return "", false
	}
	after, ok := findRequest(w.requests, d)
	if !ok && w.base != nil {
		after, ok = w.base.request(d)
	}
	if !ok {
		return "", false
	}
	return wire.Digest(hex.EncodeToString(after)), true
}

func requestDigest(p string) ([]byte, bool) {
	rest, ok := strings.CutPrefix(p, "requests/")
	if !ok || len(rest) != 3+64+5 || rest[2] != '/' || !strings.HasSuffix(rest, ".json") {
		return nil, false
	}
	hexed := rest[3 : 3+64]
	if rest[:2] != hexed[:2] || strings.ToLower(hexed) != hexed {
		return nil, false
	}
	d, err := hex.DecodeString(hexed)
	return d, err == nil
}

// FreshBase reports whether Encode renders a new base: the checkpoint was
// derived base-free, so its base file must be written before its delta.
// Otherwise the base is the one the checkpoint was decoded with.
func (w *WriterCheckpoint) FreshBase() bool { return w.base == nil || w.freshBase }

// Encode renders the two files (JSG-V0-004). The base is w's own when w was
// decoded with one or derived on it; a base-free w renders a new base of all
// its entries and an empty delta. The delta is a magic line, the base's
// SHA-256, the read checkpoint encoding of the delta entries, the
// aggregates over everything, the intent aggregate, the delta sizes, the
// delta request records, the notes on delta entries, the invalidation
// digest, and a SHA-256 trailer over all of it.
func (w *WriterCheckpoint) Encode() (base, delta []byte, err error) {
	d := w
	if w.base == nil {
		if base, err = encodeBase(w); err != nil {
			return nil, nil, err
		}
		d = &WriterCheckpoint{Checkpoint: w.Checkpoint, FullSeq: w.FullSeq, ReceiptBytes: w.ReceiptBytes, Cost: w.Cost, Invalidation: w.Invalidation, Intent: w.Intent}
		d.Checkpoint.Entries = nil
	} else {
		base = w.base.raw
	}
	sum := sha256.Sum256(base)
	cp := d.Checkpoint.Encode()
	buf := make([]byte, 0, len(writerMagic)+sha256.Size+8+len(cp)+7*8+writerIntentFields*8+8*len(d.sizes)+8+len(d.requests)+8+writerNoteBytes*len(d.notes)+2*sha256.Size)
	buf = append(buf, writerMagic...)
	buf = append(buf, sum[:]...)
	buf = binary.BigEndian.AppendUint64(buf, uint64(len(cp)))
	buf = append(buf, cp...)
	for _, v := range []uint64{w.FullSeq, w.ReceiptBytes, uint64(w.Cost.Files), w.Cost.PayloadBytes, w.Cost.EntryBytes, w.Cost.TarBytes, w.Requests()} {
		buf = binary.BigEndian.AppendUint64(buf, v)
	}
	for _, v := range []uint64{uint64(w.Intent.Tickets), uint64(w.Intent.Releases), w.Intent.Cost.PayloadBytes, w.Intent.Cost.EntryBytes, w.Intent.Cost.TarBytes} {
		buf = binary.BigEndian.AppendUint64(buf, v)
	}
	for _, v := range d.sizes {
		buf = binary.BigEndian.AppendUint64(buf, v)
	}
	buf = binary.BigEndian.AppendUint64(buf, uint64(len(d.requests)/writerRequestBytes))
	buf = append(buf, d.requests...)
	buf = binary.BigEndian.AppendUint64(buf, uint64(len(d.notes)))
	for _, n := range d.notes {
		buf = binary.BigEndian.AppendUint32(buf, n.entry)
		buf = append(buf, n.digest[:]...)
	}
	buf = append(buf, w.Invalidation[:]...)
	trailer := sha256.Sum256(buf)
	return base, append(buf, trailer[:]...), nil
}

// DecodeWriterCheckpoint parses the delta and the base it binds. Any error
// means only that the checkpoint is unusable; the writer runs the complete
// audit. An absent base, or one whose SHA-256 differs from the delta's,
// declines (JSG-V0-004). The base's entries are not turned into values
// here: a walk checks each one it uses.
func DecodeWriterCheckpoint(raw, baseRaw []byte) (*WriterCheckpoint, error) {
	bad := func(why string) error { return errCheckpoint("writer checkpoint", why) }
	if len(raw) > MaxWriterCheckpointBytes {
		return nil, bad("larger than its bound")
	}
	// Another version of this profile is refused before any layout check
	// (CAL-V0-131); the caller still only declines to the complete audit.
	if name, _, _ := strings.Cut(ProfileWriterCheckpoint, "/"); bytes.HasPrefix(raw, []byte(name+"/")) && !bytes.HasPrefix(raw, []byte(writerMagic)) {
		return nil, wire.Errorf(wire.CodeUnsupportedVersion, "writer checkpoint", "another %s version", name)
	}
	if len(raw) < len(writerMagic)+sha256.Size+8+7*8+writerIntentFields*8+8+8+2*sha256.Size {
		return nil, bad("truncated")
	}
	body, trailer := raw[:len(raw)-sha256.Size], raw[len(raw)-sha256.Size:]
	if sum := sha256.Sum256(body); !bytes.Equal(sum[:], trailer) {
		return nil, bad("trailer digest differs")
	}
	if !bytes.HasPrefix(body, []byte(writerMagic)) {
		return nil, bad("profile differs")
	}
	rest := body[len(writerMagic):]
	if baseRaw == nil {
		return nil, bad("base absent")
	}
	if sum := sha256.Sum256(baseRaw); !bytes.Equal(sum[:], rest[:sha256.Size]) {
		return nil, bad("base differs from the one the delta binds")
	}
	rest = rest[sha256.Size:]
	b, err := decodeBase(baseRaw)
	if err != nil {
		return nil, err
	}
	n := binary.BigEndian.Uint64(rest)
	rest = rest[8:]
	if n > MaxCheckpointBytes || n > uint64(len(rest)) {
		return nil, bad("embedded checkpoint length")
	}
	cp, err := decodeEmbeddedCheckpoint(rest[:n])
	if err != nil {
		return nil, err
	}
	seq := cp.Seq.Uint64()
	if b.seq > seq {
		return nil, bad("base is past the delta")
	}
	rest = rest[n:]
	if uint64(len(rest)) < 7*8+writerIntentFields*8+8*uint64(len(cp.Entries)) {
		return nil, bad("truncated aggregates or sizes")
	}
	v := make([]uint64, 7)
	for i := range v {
		v[i] = binary.BigEndian.Uint64(rest[i*8:])
	}
	rest = rest[7*8:]
	iv := make([]uint64, writerIntentFields)
	for i := range iv {
		iv[i] = binary.BigEndian.Uint64(rest[i*8:])
	}
	rest = rest[writerIntentFields*8:]
	if iv[0] > uint64(wire.MaxTicketsPerQueue) || iv[1] > uint64(wire.MaxReleasesPerQueue) || iv[2] > uint64(wire.MaxIntentTreeBytes) {
		return nil, bad("intent aggregate bounds")
	}
	sizes := make([]uint64, len(cp.Entries))
	tickets, releases, payload := b.intent.tickets, b.intent.releases, b.intent.payload
	added := 0
	for i := range sizes {
		e := cp.Entries[i]
		sizes[i] = binary.BigEndian.Uint64(rest[i*8:])
		if bound, err := snapshot.PostBound(e.Path); (e.Sha256 == nil && sizes[i] != 0) || (err == nil && sizes[i] > uint64(bound)) {
			return nil, bad("entry size")
		}
		if e.Seq.Uint64() <= b.seq {
			return nil, bad("delta entry is not after its base")
		}
		// The intent aggregate is the base's live intent records, less the
		// ones the delta replaces, plus the delta's own (JSG-V0-001).
		if j := b.search(e.Path); j < 0 {
			added++
		} else if b.live(j) && intentRecord(e.Path) {
			if strings.HasPrefix(e.Path, "intent/tickets/") {
				tickets--
			} else {
				releases--
			}
			payload -= b.size(j)
		}
		if e.Sha256 != nil && intentRecord(e.Path) {
			if strings.HasPrefix(e.Path, "intent/tickets/") {
				tickets++
			} else {
				releases++
			}
			payload += sizes[i]
		}
	}
	if tickets != iv[0] || releases != iv[1] || payload != iv[2] {
		return nil, bad("intent aggregate differs from the entries")
	}
	rest = rest[8*len(sizes):]
	if err := b.bindsDelta(cp, v, iv); err != nil {
		return nil, err
	}
	if b.n+added > maxCheckpointEntries() {
		return nil, bad("entries exceed the checkpoint bound")
	}
	w := &WriterCheckpoint{Checkpoint: *cp, FullSeq: v[0], ReceiptBytes: v[1], Cost: archive.FileSetCost{PayloadBytes: v[3], EntryBytes: v[4], TarBytes: v[5]}, sizes: sizes, base: b, total: b.n + added}
	w.Intent = IntentAggregate{Tickets: int(iv[0]), Releases: int(iv[1]), Cost: archive.FileSetCost{Files: int(iv[0] + iv[1]), PayloadBytes: iv[2], EntryBytes: iv[3], TarBytes: iv[4]}}
	if len(rest) < 8 {
		return nil, bad("truncated request count")
	}
	requests := binary.BigEndian.Uint64(rest)
	rest = rest[8:]
	if v[2] > math.MaxInt || requests > uint64(wire.MaxArchiveFiles) || uint64(len(rest)) < requests*writerRequestBytes+8+sha256.Size {
		return nil, bad("request digest count")
	}
	w.Cost.Files = int(v[2])
	if v[6] != b.requests()+requests || seq > uint64(wire.MaxArchiveFiles) || v[2] != seq+v[6] {
		return nil, bad("file count differs from receipts plus requests")
	}
	if w.FullSeq < 1 || w.FullSeq > seq || w.ReceiptBytes > w.Cost.PayloadBytes {
		return nil, bad("aggregate bounds")
	}
	digests, rest := rest[:requests*writerRequestBytes], rest[requests*writerRequestBytes:]
	if err := orderedRequests(digests); err != nil {
		return nil, err
	}
	// A request the base holds again in the delta would hide a repeat.
	for i := 0; i < len(digests); i += writerRequestBytes {
		if _, ok := b.request(digests[i : i+sha256.Size]); ok {
			return nil, bad("delta request repeats one the base holds")
		}
	}
	w.requests = append([]byte(nil), digests...)
	notes := binary.BigEndian.Uint64(rest)
	rest = rest[8:]
	// The invalidation digest closes the body; a file written before it
	// existed fails here and the writer falls back.
	if notes > uint64(wire.MaxTicketsPerQueue) || uint64(len(rest)) != notes*writerNoteBytes+sha256.Size {
		return nil, bad("note count")
	}
	copy(w.Invalidation[:], rest[notes*writerNoteBytes:])
	if w.notes, err = decodeNotes(rest[:notes*writerNoteBytes], cp.Entries); err != nil {
		return nil, err
	}
	return w, nil
}

// WriterCheckpoint derives the writer checkpoint this result supports from
// the bytes its audit physically read. A complete audit (ModeFull) derives it
// from receipt 1 and sets FullSeq to its head; a writer audit (ModeWriter)
// extends the checkpoint it resumed from by the tail it walked and keeps that
// FullSeq. Every other observation, and any receipt or request afterimage the
// audit did not read, returns an error: nothing is derived from inference.
//
// A writer audit keeps its checkpoint's base while the head is fewer than
// WriterBaseTail receipts past it, carrying the tail in the delta; otherwise,
// and for every complete audit, the result has no base and Encode renders a
// new one from every entry (JSG-V0-004).
func (res *Result) WriterCheckpoint(physical map[string]PhysicalFile) (*WriterCheckpoint, error) {
	if res == nil || res.chain == nil || res.Pending || res.StagingPresent || res.Head == nil || res.LastSeq != res.Head.LastSeq || physical == nil {
		return nil, errCheckpoint("writer checkpoint", "observation is not a settled audited head")
	}
	if res.chain.seal != nil {
		// A sealed store has no checkpoint route (JRT-V0-005).
		return nil, errCheckpoint(RetiredPrefixFile, "store carries a retired prefix")
	}
	var w, old *WriterCheckpoint
	from := uint64(1)
	switch {
	case (res.Mode == ModeFull || res.Mode == ModePrefix) && res.StructuralConsistency == "CONSISTENT":
		w = &WriterCheckpoint{FullSeq: res.LastSeq.Uint64()}
		if res.manifest != nil {
			for _, e := range res.manifest.Segments {
				d, err := hex.DecodeString(string(e.Sha256))
				if err != nil || len(d) != sha256.Size {
					return nil, errCheckpoint("writer checkpoint", "segment digest is not canonical")
				}
				w.segments = append(w.segments, [sha256.Size]byte(d))
			}
		}
	case res.Mode == ModeWriter && res.StructuralConsistency == ModeWriter && res.writerBase != nil:
		old = res.writerBase
		w = &WriterCheckpoint{FullSeq: old.FullSeq, ReceiptBytes: old.ReceiptBytes, Cost: old.Cost}
		if old.base != nil {
			w.segments = old.base.meta.segments
		}
		from = old.Seq.Uint64() + 1
	default:
		return nil, errCheckpoint("writer checkpoint", "observation is not a complete or writer audit")
	}
	// A segmented store keeps its base until the next seal rebuilds it at
	// the sealed sequence, so its delta stays within two segments
	// (JSG-V0-009); an unsegmented one rebuilds it every WriterBaseTail.
	keep := old != nil && old.base != nil && !res.sealRebase && (res.Head.Segments != nil || res.LastSeq.Uint64()-old.base.seq < WriterBaseTail)
	w.Checkpoint = res.chainHead()
	var err error
	if keep {
		err = w.extendDelta(res, old)
	} else {
		err = res.chain.each([]string{"requests/"}, func(p string, l latest) error {
			if l.bytes < 0 {
				return errCheckpoint(p, "afterimage size was not walked")
			}
			w.Entries = append(w.Entries, CheckpointEntry{Path: p, Seq: l.seq, Sha256: l.digest})
			w.sizes = append(w.sizes, uint64(l.bytes))
			return nil
		})
	}
	if err != nil {
		return nil, err
	}
	if w.Intent, err = res.intentAggregate(); err != nil {
		return nil, err
	}
	if err := w.deriveNotes(res, old, keep); err != nil {
		return nil, err
	}
	for seq := from; seq <= res.LastSeq.Uint64(); seq++ {
		name, err := snapshot.ReceiptName(seq)
		if err != nil {
			return nil, err
		}
		if err := w.charge(physical, "receipts/"+name, nil); err != nil {
			return nil, err
		}
	}
	var digests [][]byte
	for _, p := range sortedPaths(res.chain.canonical) {
		l := res.chain.canonical[p]
		if !strings.HasPrefix(p, "requests/") {
			continue
		}
		d, ok := requestDigest(p)
		if !ok || l.digest == nil {
			return nil, errCheckpoint(p, "request afterimage is not a canonical retained file")
		}
		after, err := hex.DecodeString(string(*l.digest))
		if err != nil || len(after) != sha256.Size {
			return nil, errCheckpoint(p, "request afterimage digest is not canonical")
		}
		if err := w.charge(physical, p, l.digest); err != nil {
			return nil, err
		}
		digests = append(digests, append(d, after...))
	}
	// The tail's requests join the old delta's when the base is kept, and
	// every request the old checkpoint carried when it is not.
	var carried []byte
	switch {
	case keep:
		carried = old.requests
	case old != nil && old.base != nil:
		carried = mergeRequests(old.base.reqs, old.requests)
	case old != nil:
		carried = old.requests
	}
	for i := 0; i < len(carried); i += writerRequestBytes {
		digests = append(digests, carried[i:i+writerRequestBytes])
	}
	sort.Slice(digests, func(i, j int) bool { return bytes.Compare(digests[i], digests[j]) < 0 })
	w.requests = make([]byte, 0, len(digests)*writerRequestBytes)
	for i, d := range digests {
		if i > 0 && bytes.Equal(digests[i-1][:sha256.Size], d[:sha256.Size]) {
			return nil, errCheckpoint("requests", "request path repeats")
		}
		if keep {
			if _, ok := old.base.request(d[:sha256.Size]); ok {
				return nil, errCheckpoint("requests", "request path repeats")
			}
		}
		w.requests = append(w.requests, d...)
	}
	return w, nil
}

// IncrementalWriterCheckpoint is the incremental scheduled refresh's
// checkpoint (PSR-V0-891, extending CAL-V0-265): from a writer audit resumed
// from a bound checkpoint and walked to its head with the complete audit's
// per-receipt validators, it derives the checkpoint WriterCheckpoint derives
// for that audit, but always on a new base rendered from every entry and with
// FullSeq at the audited head, so it is the checkpoint a complete refresh at
// that head retains. Any other observation returns an error.
func (res *Result) IncrementalWriterCheckpoint(physical map[string]PhysicalFile) (*WriterCheckpoint, error) {
	if res == nil || res.Mode != ModeWriter || res.writerBase == nil || res.Head == nil || res.Head.Segments != nil {
		return nil, errCheckpoint("writer checkpoint", "observation is not an unsegmented writer audit")
	}
	rebase := res.sealRebase
	res.sealRebase = true
	w, err := res.WriterCheckpoint(physical)
	res.sealRebase = rebase
	if err != nil {
		return nil, err
	}
	w.FullSeq = res.LastSeq.Uint64()
	return w, nil
}

// extendDelta sets w's entries to old's delta with every non-request path
// the tail posted merged in over it, on old's base.
func (w *WriterCheckpoint) extendDelta(res *Result, old *WriterCheckpoint) error {
	tail := make([]string, 0, len(res.chain.canonical))
	for p := range res.chain.canonical {
		if !strings.HasPrefix(p, "requests/") {
			tail = append(tail, p)
		}
	}
	sort.Strings(tail)
	n := len(old.Entries) + len(tail)
	w.Entries, w.sizes = make([]CheckpointEntry, 0, n), make([]uint64, 0, n)
	i, j := 0, 0
	for i < len(old.Entries) || j < len(tail) {
		if j == len(tail) || (i < len(old.Entries) && old.Entries[i].Path < tail[j]) {
			w.Entries, w.sizes = append(w.Entries, old.Entries[i]), append(w.sizes, old.sizes[i])
			i++
			continue
		}
		p := tail[j]
		if i < len(old.Entries) && old.Entries[i].Path == p {
			i++
		}
		l := res.chain.canonical[p]
		if l.bytes < 0 {
			return errCheckpoint(p, "afterimage size was not walked")
		}
		w.Entries, w.sizes = append(w.Entries, CheckpointEntry{Path: p, Seq: l.seq, Sha256: l.digest}), append(w.sizes, uint64(l.bytes))
		j++
	}
	w.base, w.total = old.base, old.base.n
	for _, e := range w.Entries {
		if old.base.search(e.Path) < 0 {
			w.total++
		}
	}
	return nil
}

// deriveNotes records the note reference of every live ticket in w.Entries
// that carries one at Seq: the reference the audit walked, or, for a ticket
// a writer audit's tail did not post, the one its old checkpoint carries. A
// ticket the tail posted must have been walked; anything else is not
// derived. On a kept base the base's notes stay there; otherwise every note
// the old checkpoint carried is a candidate.
func (w *WriterCheckpoint) deriveNotes(res *Result, old *WriterCheckpoint, keep bool) error {
	var refs map[string]*ticket.OperatorNoteReference
	if res.chain.notes != nil {
		refs = res.chain.notes.refs
	}
	candidates := make([]string, 0, len(refs))
	for p, ref := range refs {
		if ref != nil {
			candidates = append(candidates, p)
		}
	}
	if old != nil {
		for _, n := range old.notes {
			candidates = append(candidates, old.Entries[n.entry].Path)
		}
		if !keep && old.base != nil {
			for k := 0; k < len(old.base.notes); k += writerNoteBytes {
				candidates = append(candidates, old.base.path(int(binary.BigEndian.Uint32(old.base.notes[k:]))))
			}
		}
	}
	var notes []writerNote
	for _, p := range candidates {
		e := w.search(p)
		if e < 0 || w.Entries[e].Sha256 == nil || !strings.HasPrefix(p, "intent/tickets/") {
			continue
		}
		var digest [sha256.Size]byte
		if old != nil && w.Entries[e].Seq.Uint64() <= old.Seq.Uint64() {
			d, ok := old.note(p)
			if !ok {
				return errCheckpoint(p, "note reference was neither walked nor carried")
			}
			digest = d
		} else {
			ref, walked := refs[p]
			if !walked {
				return errCheckpoint(p, "note reference was not walked by this audit")
			}
			if ref == nil {
				continue
			}
			digest = noteDigest(ref)
		}
		notes = append(notes, writerNote{entry: uint32(e), digest: digest})
	}
	// A ticket both carried and walked is derived once from the walk.
	sort.Slice(notes, func(i, j int) bool { return notes[i].entry < notes[j].entry })
	w.notes = make([]writerNote, 0, len(notes))
	for _, n := range notes {
		if k := len(w.notes); k > 0 && w.notes[k-1].entry == n.entry {
			if w.notes[k-1].digest != n.digest {
				return errCheckpoint(w.Entries[n.entry].Path, "note reference derived twice")
			}
			continue
		}
		w.notes = append(w.notes, n)
	}
	return nil
}

// charge adds one physically read retained file to the aggregates. A request
// afterimage must also carry the digest its canonical latest names.
func (w *WriterCheckpoint) charge(physical map[string]PhysicalFile, p string, want *wire.Digest) error {
	f, ok := physical[p]
	if !ok || f.Bytes < 0 || (want != nil && f.Sha256 != *want) {
		return errCheckpoint(p, "retained file was not read by this audit")
	}
	one, err := archive.EntryCost(p, uint64(f.Bytes))
	if err != nil {
		return err
	}
	if w.Cost, err = w.Cost.Add(one); err != nil {
		return err
	}
	if strings.HasPrefix(p, "receipts/") {
		if w.ReceiptBytes > math.MaxUint64-uint64(f.Bytes) {
			return errCheckpoint(p, "receipt bytes overflow")
		}
		w.ReceiptBytes += uint64(f.Bytes)
	}
	return nil
}

// intentAggregate is the IntentAggregate at this result's head. A complete
// or prefix walk sums every live ticket and release afterimage it walked; a
// writer walk adjusts its base checkpoint's aggregate by only the paths its
// tail posted, so the work is bounded by the tail (JSG-V0-001).
func (res *Result) intentAggregate() (IntentAggregate, error) {
	st := res.chain
	var agg IntentAggregate
	charge := func(p string, size uint64, sign int) error {
		one, err := archive.EntryCost(p, size)
		if err != nil {
			return err
		}
		n := sign
		if sign > 0 {
			agg.Cost, err = agg.Cost.Add(one)
		} else {
			agg.Cost, err = subCost(agg.Cost, one)
		}
		if err != nil {
			return errCheckpoint(p, "intent aggregate out of range")
		}
		if strings.HasPrefix(p, "intent/tickets/") {
			agg.Tickets += n
		} else {
			agg.Releases += n
		}
		return nil
	}
	live := func(p string) (uint64, bool, error) {
		l := st.canonical[p]
		if l.digest == nil {
			return 0, false, nil
		}
		if l.bytes < 0 {
			return 0, false, errCheckpoint(p, "afterimage size was not walked")
		}
		return uint64(l.bytes), true, nil
	}
	if res.Mode == ModeWriter {
		base := res.writerBase
		if base == nil || st.posted == nil || len(base.sizes) != len(base.Entries) {
			return agg, errCheckpoint("writer checkpoint", "no sized base")
		}
		agg = base.Intent
		for p := range st.posted {
			if size, ok := base.entrySize(p); ok && intentRecord(p) {
				if err := charge(p, size, -1); err != nil {
					return agg, err
				}
			}
		}
		for p := range st.posted {
			if !intentRecord(p) {
				continue
			}
			size, ok, err := live(p)
			if err != nil {
				return agg, err
			}
			if ok {
				if err := charge(p, size, 1); err != nil {
					return agg, err
				}
			}
		}
	} else {
		for p := range st.canonical {
			if !intentRecord(p) {
				continue
			}
			size, ok, err := live(p)
			if err != nil {
				return agg, err
			}
			if ok {
				if err := charge(p, size, 1); err != nil {
					return agg, err
				}
			}
		}
	}
	if agg.Tickets < 0 || agg.Releases < 0 || agg.Tickets > wire.MaxTicketsPerQueue || agg.Releases > wire.MaxReleasesPerQueue || agg.Cost.Files != agg.Tickets+agg.Releases || agg.Cost.PayloadBytes > wire.MaxIntentTreeBytes {
		return agg, errCheckpoint("intent", "intent aggregate out of range")
	}
	return agg, nil
}

// subCost is a - b, refusing any field that would go negative.
func subCost(a, b archive.FileSetCost) (archive.FileSetCost, error) {
	if b.Files > a.Files || b.PayloadBytes > a.PayloadBytes || b.EntryBytes > a.EntryBytes || b.TarBytes > a.TarBytes {
		return a, errCheckpoint("intent", "aggregate underflow")
	}
	return archive.FileSetCost{Files: a.Files - b.Files, PayloadBytes: a.PayloadBytes - b.PayloadBytes, EntryBytes: a.EntryBytes - b.EntryBytes, TarBytes: a.TarBytes - b.TarBytes}, nil
}

// IntentSummary is what a liveness write knows about the live canonical
// ticket and release records without listing or reading them
// (JSG-V0-001): their count and archive cost, and their paths by lookup.
type IntentSummary struct {
	IntentAggregate
	// Has reports whether p is a live canonical ticket or release record.
	Has func(p string) bool
}

// LivenessIntent is the IntentSummary of a liveness writer audit's head:
// its base checkpoint's aggregate adjusted by the tail it walked. Every
// other result returns an error, and the caller declines the route.
func (res *Result) LivenessIntent() (IntentSummary, error) {
	if res == nil || !res.livenessWrite || res.Mode != ModeWriter || res.chain == nil {
		return IntentSummary{}, errCheckpoint("intent", "not a liveness writer audit")
	}
	agg, err := res.intentAggregate()
	if err != nil {
		return IntentSummary{}, err
	}
	st := res.chain
	return IntentSummary{IntentAggregate: agg, Has: func(p string) bool {
		if !intentRecord(p) {
			return false
		}
		if l, ok := st.canonical[p]; ok {
			return l.digest != nil
		}
		return st.elided.liveIntent(p)
	}}, nil
}

// chainHead is the read checkpoint body for this result's chain without
// its entries.
func (res *Result) chainHead() Checkpoint {
	return Checkpoint{QueueID: res.Head.QueueID, PrimaryWorktree: res.Head.PrimaryWorktree, Seq: res.LastSeq, ReceiptSha256: res.LastReceiptSha256, Generation: res.Head.Generation, InitSha256: res.Head.InitSha256, SemanticCoverage: res.SemanticCoverage}
}

// chainCheckpoint is the read checkpoint body for this result's chain: every
// latest entry, request paths excluded, and the filter of every request path
// the chain posted (CAL-V0-321).
func (res *Result) chainCheckpoint() (Checkpoint, error) {
	cp := res.chainHead()
	err := res.chain.each([]string{"requests/"}, func(p string, l latest) error {
		cp.Entries = append(cp.Entries, CheckpointEntry{Path: p, Seq: l.seq, Sha256: l.digest})
		return nil
	})
	if err != nil {
		return cp, err
	}
	cp.Requests, err = res.chain.requestFilter()
	return cp, err
}

// requestFilter is the filter of every request path the chain holds: the
// walk's own and, on a view walk, the writer checkpoint's.
func (st *chain) requestFilter() (*RequestFilter, error) {
	digests := [][]byte{}
	for p := range st.canonical {
		if !strings.HasPrefix(p, "requests/") {
			continue
		}
		d, ok := requestDigest(p)
		if !ok {
			return nil, errCheckpoint(p, "request path is not canonical")
		}
		digests = append(digests, d)
	}
	n := len(digests)
	if st.view {
		n += st.elided.requestCount()
	}
	f := newRequestFilter(n)
	for _, d := range digests {
		f.add(d)
	}
	if st.view {
		st.elided.eachRequest(f.add)
	}
	return f, nil
}

// requestCount is the number of request records w and its base carry; a
// path both carry counts twice, which only sizes a filter up.
func (w *WriterCheckpoint) requestCount() int {
	n := len(w.requests) / writerRequestBytes
	if w.base != nil {
		n += len(w.base.reqs) / writerRequestBytes
	}
	return n
}

// eachRequest calls fn with the path digest of every request record w and
// its base carry.
func (w *WriterCheckpoint) eachRequest(fn func([]byte)) {
	for _, reqs := range [][]byte{w.requests, w.baseRequests()} {
		for i := 0; i+writerRequestBytes <= len(reqs); i += writerRequestBytes {
			fn(reqs[i : i+sha256.Size])
		}
	}
}

func (w *WriterCheckpoint) baseRequests() []byte {
	if w.base == nil {
		return nil
	}
	return w.base.reqs
}

// WriterBase is the checkpoint a ModeWriter result resumed from, or nil.
func (res *Result) WriterBase() *WriterCheckpoint {
	if res == nil {
		return nil
	}
	return res.writerBase
}

// WriterCheckpoint derives the writer checkpoint this mutation audit's
// complete observation supports; see Result.WriterCheckpoint.
func (m *MutationAudit) WriterCheckpoint() (*WriterCheckpoint, error) {
	if m == nil || m.result == nil {
		return nil, errCheckpoint("writer checkpoint", "no settled mutation audit")
	}
	return m.result.WriterCheckpoint(m.Physical.Files)
}

// SealedWriterCheckpoint is WriterCheckpoint with a base at the sealed
// sequence where the audit can keep one; see Result.SealedWriterCheckpoint
// (JSG-V0-780).
func (m *MutationAudit) SealedWriterCheckpoint() (*WriterCheckpoint, error) {
	if m == nil || m.result == nil {
		return nil, errCheckpoint("writer checkpoint", "no settled mutation audit")
	}
	return m.result.SealedWriterCheckpoint(m.Physical.Files)
}

// AuditForWriter is the writer's audit resumed at wc (CAL-V0-116, proposed).
// It rebinds wc to the retained receipt it names, requires the receipt names
// on disk to be exactly 1..head, walks and validates every receipt after wc
// exactly as the complete audit does, refuses a request path wc already
// holds, and compares every latest afterimage with its projection: intent
// and private state by digest, evidence and pinned blobs by their
// content-addressed names. It does not re-read receipts 1..wc.Seq, their
// request afterimages or evidence bodies; Result.Mode says so.
//
// forMutation selects what AuditForMutation selects (queue, policy and every
// observed ticket and release); otherwise it selects what AuditForWrite
// does. A requestID retained in the tail or before wc does not fail the
// audit: WriterReplay then finds it (CAL-V0-190, proposed).
//
// Every error, refusal or not, means only that this route cannot serve the
// observation: the caller runs the complete audit, which derives the refusal
// itself. A nil error returns physical metadata for every file it read.
func (r Reader) AuditForWriter(wc *WriterCheckpoint, requestID string, forMutation bool) (*Result, PhysicalObservation, error) {
	observation := PhysicalObservation{}
	if wc == nil {
		return nil, observation, errCheckpoint("writer checkpoint", "absent")
	}
	switch r.Source.(type) {
	case Native, *Native:
	default:
		return nil, observation, errCheckpoint("source", "writer checkpoint needs a native source")
	}
	if requestID != "" {
		if _, err := snapshot.RequestPath(requestID); err != nil {
			return nil, observation, err
		}
	}
	r.writer, r.physical, r.Checkpoint, r.handoffPolicy = wc, &observation, nil, nil
	r.writerCache, r.observedIntent = !forMutation, forMutation
	for attempt := 0; attempt < 2; attempt++ {
		result, err := r.auditAttempt(nil, requestID, profileLimits, true, nil)
		if wire.CodeOf(err) == wire.CodeSnapshotMoved {
			continue
		}
		if err != nil {
			return nil, observation, err
		}
		return result, observation, nil
	}
	return nil, observation, moved("/")
}

// AuditForLiveness is AuditForWriter (forMutation false) for a HEARTBEAT or
// RENEW. Both captures list receipts/ exactly, as AuditForWriter does: on a
// segmented journal it holds at most 1,023 names, so the listing no longer
// grows with the history (JSG-V0-007, replacing the CAL-V0-224 probe).
//
// It also lists no intent directory and reads no ticket or release
// (JSG-V0-001): only queue.json and policy.json, by name, whose digests the
// inventory binds. The result reports no intent tree digest (JSG-V0-002),
// its ProjectionAgreement is IntentRecordsNotCompared, and LivenessIntent
// summarizes the live ticket and release records from the checkpoint.
func (r Reader) AuditForLiveness(wc *WriterCheckpoint, requestID string) (*Result, PhysicalObservation, error) {
	r.liveness = true
	return r.AuditForWriter(wc, requestID, false)
}

// ModePrefix is Result.Mode for the scheduled writer-checkpoint refresh's
// prefix walk (CAL-V0-265, proposed): every receipt 1..H of the head H its
// capture observed, walked exactly as the complete audit walks them, and
// every request, evidence and pinned afterimage they leave latest read and
// compared by digest. It does not compare the private and intent
// projections or list strays at H: every writer audit resumed from the
// checkpoint compares them at its own head.
const ModePrefix = "COMPLETE_PREFIX"

// AuditThrough walks receipts 1..seq only (TCL-V0-820): the CAL-V0-265
// prefix walk bounded at seq, so receipts after seq are not validated and
// the result's LastSeq is seq. It is a bounded verdict, never a complete
// audit or a checkpoint; a moving journal is re-walked up to four times.
func (r Reader) AuditThrough(seq uint64) (*Result, error) {
	switch r.Source.(type) {
	case Native, *Native:
	default:
		return nil, errCheckpoint("source", "a bounded audit needs a native source")
	}
	if seq == 0 {
		return nil, wire.Errorf(wire.CodeMalformed, "--through", "sequence must be at least 1")
	}
	observation := PhysicalObservation{}
	r.physical, r.prefix, r.through, r.Checkpoint, r.handoffPolicy = &observation, true, seq, nil, nil
	for attempt := 0; attempt < 4; attempt++ {
		result, err := r.auditAttempt(nil, "", profileLimits, true, nil)
		if wire.CodeOf(err) == wire.CodeSnapshotMoved {
			continue
		}
		return result, err
	}
	return nil, moved("receipts")
}

// AuditWriterRefresh is the scheduled refresh's audit (CAL-V0-117,
// CAL-V0-265, proposed). It first runs one attempt of AuditForWriteObserved;
// any result other than SNAPSHOT_MOVED is returned as that audit returns it,
// so on a journal that holds still the refresh accepts, refuses and retains
// exactly as before. Only when the journal moved under that attempt does it
// walk the prefix to the head it captured (ModePrefix): a prefix walk that
// succeeded stands when every receipt 1..H is still the same file
// afterwards, and a refusal only when the whole observation is unchanged,
// so no refusal is derived from a moving journal. A prefix result supports
// only Result.WriterCheckpoint: it is not a complete audit and never a read
// checkpoint.
func (r Reader) AuditWriterRefresh() (*Result, PhysicalObservation, error) {
	observation := PhysicalObservation{}
	switch r.Source.(type) {
	case Native, *Native:
	default:
		return nil, observation, errCheckpoint("source", "writer refresh needs a native source")
	}
	full := r
	full.writerCache, full.physical, full.fullAttempts = true, &observation, 1
	result, err := full.audit(nil, "", profileLimits, true)
	if wire.CodeOf(err) != wire.CodeSnapshotMoved {
		return result, observation, err
	}
	observation = PhysicalObservation{}
	r.physical, r.prefix, r.Checkpoint, r.handoffPolicy = &observation, true, nil, nil
	for attempt := 0; attempt < 4; attempt++ {
		result, err := r.auditAttempt(nil, "", profileLimits, true, nil)
		if wire.CodeOf(err) == wire.CodeSnapshotMoved {
			continue
		}
		return result, observation, err
	}
	return nil, observation, moved("receipts")
}

// walkPrefix is AuditWriterRefresh's body; see there.
func (r Reader) walkPrefix(o *observation, lim limits) (*Result, error) {
	result := &Result{Identity: o.identity, Head: o.head, Mode: ModePrefix, Records: map[string]Record{}, StructuralConsistency: "NOT_OBSERVED", ProjectionAgreement: "NOT_OBSERVED", SemanticCoverage: "NOT_OBSERVED", HistoricalAcceptance: "NOT_OBSERVED", ActorAuthentication: "NOT_OBSERVED", Liveness: "NOT_OBSERVED", RuntimeQualification: "NOT_OBSERVED"}
	if o.seal != nil {
		return result, errCheckpoint(RetiredPrefixFile, "store carries a retired prefix")
	}
	if o.head == nil {
		return result, errCheckpoint("head.json", "no settled head")
	}
	if o.segErr != nil {
		// A segmented journal is walked through its bound segments; one
		// that does not match its head is refused as the complete walk
		// refuses it (JSG-V0-011).
		return result, o.segErr
	}
	if o.stageErr != nil || o.staging || len(o.stageDigests) > 0 {
		// A temporary or staged name is classified only by a complete audit
		// of a held journal. The prefix walk runs only after the complete
		// attempt moved, so it retains nothing: FullSeq does not advance and
		// WriterFullBound still sends every writer, liveness writes included,
		// to the complete route (decision 0549).
		return result, moved("staging")
	}
	if o.head.QueueID != r.QueueID || o.head.PrimaryWorktree != r.PrimaryWorktree {
		return result, wire.Errorf(wire.CodeJournalForked, "head.json", "selected queue/primary differs from head; relocation needs an explicit RELOCATE receipt")
	}
	headSeq := o.head.LastSeq.Uint64()
	if headSeq == 0 || uint64(len(o.receipts)) < headSeq {
		return result, wire.Errorf(wire.CodeJournalForked, "receipts", "receipt count is below head")
	}
	for i, name := range o.receipts[:headSeq] {
		seq, err := receiptSeq(name)
		if err != nil {
			return result, err
		}
		if seq != uint64(i+1) {
			return result, wire.Errorf(wire.CodeJournalForked, name, "receipt gap or late extra")
		}
	}
	st := &chain{canonical: map[string]latest{}}
	if r.through != 0 {
		// A bounded audit restores elided liveness receipts as the complete
		// walk does; only a walk that reaches the head applies Superseded
		// (TCL-V0-820). The writer refresh keeps refusing them.
		st.restore = &snapshot.ReceiptRestorer{}
	}
	bound := headSeq
	if r.through != 0 {
		if r.through > headSeq {
			return result, wire.Errorf(wire.CodeMalformed, "--through", "sequence %d is above the head %d", r.through, headSeq)
		}
		bound = r.through
	}
	for i, name := range o.receipts[:bound] {
		if err := r.step(o, st, result, name, uint64(i+1), headSeq, nil, "", lim); err != nil {
			return result, err
		}
	}
	result.chain = st
	result.StructuralConsistency = "CONSISTENT"
	if result.SemanticCoverage != "UNKNOWN" {
		result.SemanticCoverage = "KNOWN_CODECS"
	}
	if bound < headSeq {
		// A later receipt may post a request, evidence or pinned path
		// again, so the current projection is not the state at the bound
		// (TCL-V0-820); projectionAgreement stays NOT_OBSERVED.
		return result, nil
	}
	if err := st.restore.Superseded(); err != nil {
		return result, err
	}
	// Request afterimages are written once and evidence and pinned blobs are
	// content-addressed, so their latest afterimage at H is still what a
	// later head projects unless a later receipt posted it again; that
	// mismatch is a refusal only on an unchanged observation (samePrefix).
	for _, p := range sortedPaths(st.canonical) {
		if !strings.HasPrefix(p, "requests/") && !strings.HasPrefix(p, "evidence/") && !strings.HasPrefix(p, "pinned/") {
			continue
		}
		digest, _, err := r.projected(o, p, false)
		if err != nil {
			return result, err
		}
		if !equalDigest(digest, st.canonical[p].digest) {
			return result, wire.Errorf(wire.CodeJournalForked, p, "projection differs from latest canonical afterimage")
		}
	}
	return result, nil
}

// prefixHeld reports whether a successful prefix walk over before still
// stands: a fresh listing of receipts/ shows every walked receipt 1..H as
// the same file with the same mode, size and modification time, and every
// request, evidence and pinned afterimage the walk compared is still the
// file the capture observed (or still absent). Receipts after H, head.json
// and every other path may have moved. On a segmented journal the sealed
// receipts 1..S have no files of their own: the bound manifest and every
// segment it names must instead still be the files the capture observed
// and the walk read (JSG-V0-011). A later seal removes walked receipts and
// so does not hold.
func prefixHeld(source Native, before *observation, result *Result, lim limits) (bool, error) {
	headSeq := before.head.LastSeq.Uint64()
	if uint64(len(before.receipts)) < headSeq {
		return false, nil
	}
	var through uint64
	if m := before.manifest; m != nil {
		through = m.Through
		paths := []string{snapshot.SegmentManifestName(through)}
		for _, e := range m.Segments {
			paths = append(paths, snapshot.SegmentName(e.First, e.Last))
		}
		for _, p := range paths {
			full, err := source.path(p)
			if err != nil {
				return false, err
			}
			b, err := os.Lstat(full)
			if err != nil && !os.IsNotExist(err) {
				return false, err
			}
			if !sameStat(before.files[p], b) {
				return false, nil
			}
		}
	}
	listing, err := source.List("receipts", lim.scan)
	if err != nil {
		return false, err
	}
	now := make(map[string]os.FileInfo, len(listing.Entries))
	for _, e := range listing.Entries {
		now[e.Name] = e.Info
	}
	for _, name := range before.receipts[through:headSeq] {
		a, b := before.files["receipts/"+name], now[name]
		if !sameStat(a, b) {
			return false, nil
		}
	}
	for _, p := range sortedPaths(result.chain.canonical) {
		if !strings.HasPrefix(p, "requests/") && !strings.HasPrefix(p, "evidence/") && !strings.HasPrefix(p, "pinned/") {
			continue
		}
		full, err := source.path(p)
		if err != nil {
			return false, err
		}
		b, err := os.Lstat(full)
		if err != nil && !os.IsNotExist(err) {
			return false, err
		}
		if a := before.files[p]; (a == nil) != (b == nil) || a != nil && !sameStat(a, b) {
			return false, nil
		}
	}
	return true, nil
}

// sameStat reports whether a and b observe the same file with the same mode,
// size and modification time.
func sameStat(a, b os.FileInfo) bool {
	return a != nil && b != nil && os.SameFile(a, b) && a.Mode() == b.Mode() && a.Size() == b.Size() && a.ModTime().Equal(b.ModTime())
}

// walkWriter is AuditForWriter's body; see there.
func (r Reader) walkWriter(o *observation, wc *WriterCheckpoint, selected map[string]bool, request string, lim limits) (*Result, error) {
	result := &Result{Identity: o.identity, Head: o.head, Mode: ModeWriter, Records: map[string]Record{}, StructuralConsistency: "NOT_OBSERVED", ProjectionAgreement: "NOT_OBSERVED", SemanticCoverage: "NOT_OBSERVED", HistoricalAcceptance: "NOT_OBSERVED", ActorAuthentication: "NOT_OBSERVED", Liveness: "NOT_OBSERVED", RuntimeQualification: "NOT_OBSERVED"}
	if o.seal != nil {
		return result, errCheckpoint(RetiredPrefixFile, "store carries a retired prefix")
	}
	if r.writerCache {
		result.RequestDigests = map[string]wire.Digest{}
	}
	if o.head == nil || o.stageErr != nil || o.staging || len(o.stageDigests) > 0 {
		return result, errCheckpoint("/", "journal is not plainly settled")
	}
	head := o.head
	headSeq, from := head.LastSeq.Uint64(), wc.Seq.Uint64()
	if head.QueueID != r.QueueID || head.PrimaryWorktree != r.PrimaryWorktree || wc.QueueID != r.QueueID || wc.PrimaryWorktree != r.PrimaryWorktree || wc.InitSha256 != head.InitSha256 {
		return result, errCheckpoint("/queueId", "identity differs from head")
	}
	if head.VersionSha256 != wire.Sum([]byte(snapshot.VersionBytes)) {
		return result, errCheckpoint("head.json", "head version differs")
	}
	if from > headSeq || wc.Generation.Uint64() > head.Generation.Uint64() || wc.count() > maxCheckpointEntries() {
		return result, errCheckpoint("/seq", "checkpoint is not a prefix of head")
	}
	bound := uint64(WriterFullBound)
	if head.Segments != nil {
		bound = WriterSegmentedBound
	}
	// A walk resumed from a sealed base starts at the sealed receipt S that
	// head binds and walks every active receipt S+1..head, at most
	// WriterSegmentedBound-1 of them (LGL-V0-009).
	sealedFrom := r.resume != nil && r.resume.sealedBase && head.Segments != nil && from == head.Segments.Through.Uint64() && from == r.resume.through && from == wc.FullSeq
	if !sealedFrom && headSeq-from > MaxWriterTail || headSeq-wc.FullSeq >= bound {
		return result, errCheckpoint("/seq", "tail exceeds the writer bound")
	}
	if o.segErr != nil {
		return result, o.segErr
	}
	// Names validated by receiptSeq are canonical and unique, so a count of
	// head-S with first S+1 and last head is exactly S+1..head, S being the
	// sealed prefix (0 when unsegmented, JSG-V0-007). A pending or extra
	// receipt is classified by the complete audit.
	low := uint64(1)
	if head.Segments != nil {
		low = head.Segments.Through.Uint64() + 1
	}
	if from < low && !(sealedFrom && from+1 == low) {
		return result, errCheckpoint("/seq", "checkpoint receipt is sealed")
	}
	if uint64(len(o.receipts)) != headSeq-low+1 {
		return result, errCheckpoint("receipts", "receipt names differ from head")
	}
	first, _ := snapshot.ReceiptName(low)
	last, err := snapshot.ReceiptName(headSeq)
	if err != nil {
		return result, err
	}
	if o.receipts[0] != first || o.receipts[len(o.receipts)-1] != last {
		return result, errCheckpoint("receipts", "receipt names differ from head")
	}
	name, err := snapshot.ReceiptName(from)
	if err != nil {
		return result, err
	}
	raw, err := requiredRead(r.Source, "receipts/"+name, wire.MaxReceiptFileBytes)
	if err != nil {
		return result, err
	}
	if sealedFrom {
		// Receipt S is served from the last bound segment, whose bytes and
		// digest the segment view checked against the manifest; its chain
		// digest is the one receipt S+1 must name.
		rc, digest, _, err := snapshot.DecodeRetainedReceipt(raw)
		if err != nil || digest != wc.ReceiptSha256 || rc.Seq.Uint64() != from || rc.HeadGeneration != wc.Generation {
			return result, errCheckpoint(name, "sealed base receipt differs")
		}
	} else if wire.Sum(raw) != wc.ReceiptSha256 {
		return result, errCheckpoint(name, "retained receipt differs")
	} else if rc, err := snapshot.DecodeReceipt(raw); err != nil || rc.Seq.Uint64() != from || rc.HeadGeneration != wc.Generation {
		return result, errCheckpoint(name, "retained receipt sequence/generation differs")
	}
	if from == headSeq && (*head.LastReceiptSha256 != wc.ReceiptSha256 || head.Generation != wc.Generation) {
		return result, errCheckpoint("head.json", "head digest/generation differs")
	}
	if len(wc.sizes) != len(wc.Entries) {
		return result, errCheckpoint("writer checkpoint", "entry sizes absent")
	}
	// The walk reads the checkpoint's entries where it uses them, never
	// copying the inventory (JSG-V0-004).
	st := &chain{canonical: map[string]latest{}, prev: &wc.ReceiptSha256, generation: wc.Generation.Uint64(), elided: wc, resumed: from, resumedSha: wc.ReceiptSha256, posted: map[string]bool{}, view: true, primary: wc.PrimaryWorktree}
	result.SemanticCoverage = wc.SemanticCoverage
	result.LastSeq, result.LastReceiptSha256 = wc.Seq, wc.ReceiptSha256
	for seq := from + 1; seq <= headSeq; seq++ {
		name, err := snapshot.ReceiptName(seq)
		if err != nil {
			return result, err
		}
		if err := r.step(o, st, result, name, seq, headSeq, selected, request, lim); err != nil {
			return result, err
		}
	}
	result.StructuralConsistency = ModeWriter
	var skip []string
	if r.liveness {
		// Not observed by a liveness write (JSG-V0-001): the ticket and
		// release records are passed over without being visited.
		skip = []string{"intent/releases/", "intent/tickets/"}
	}
	err = st.each(skip, func(p string, record latest) error {
		if strings.HasPrefix(p, "evidence/") || strings.HasPrefix(p, "pinned/") {
			// Content-addressed: the listed name is the digest.
			info, present := o.files[p]
			if present != (record.digest != nil) || (present && !info.Mode().IsRegular()) {
				return errCheckpoint(p, "blob presence differs from latest canonical afterimage")
			}
			return nil
		}
		if r.liveness && strings.HasPrefix(p, "intent/") && p != "intent/queue.json" && p != "intent/policy.json" {
			// Not observed by a liveness write (JSG-V0-001).
			return nil
		}
		_, have := result.Records[p]
		need := !have && r.selects(p, selected)
		digest, raw, err := r.projected(o, p, need)
		if err != nil {
			return err
		}
		if !equalDigest(digest, record.digest) {
			return errCheckpoint(p, "projection differs from latest canonical afterimage")
		}
		if need {
			if len(raw) > lim.selected-st.selectedBytes {
				return errCheckpoint(p, "selected canonical bytes exceed the selection budget")
			}
			st.selectedBytes += len(raw)
			result.Records[p] = Record{Seq: record.seq, Sha256: record.digest, Raw: raw}
		}
		return nil
	})
	if err != nil {
		return result, err
	}
	if err := r.strays(o, st.has, true); err != nil {
		return result, err
	}
	result.listing = map[string]ListedFile{}
	for p, info := range o.files {
		switch {
		case p == "intent" || strings.HasPrefix(p, "intent/"):
		case info.IsDir():
			result.listing[p] = ListedFile{Dir: true}
		case info.Mode().IsRegular():
			result.listing[p] = ListedFile{Bytes: info.Size()}
		}
	}
	result.chain, result.writerBase = st, wc
	result.ProjectionAgreement = "AGREES"
	if r.liveness {
		result.ProjectionAgreement, result.livenessWrite = IntentRecordsNotCompared, true
	}
	return result, nil
}

// IntentRecordsNotCompared is the ProjectionAgreement of a liveness writer
// audit: private state, queue.json and policy.json agree, and no ticket or
// release projection was observed (JSG-V0-001).
const IntentRecordsNotCompared = "INTENT_RECORDS_NOT_COMPARED"

// ListedFile is one path a writer audit listed: a directory, or a regular
// file and its listed size.
type ListedFile struct {
	Dir   bool
	Bytes int64
}

// WriterListing is what a ModeWriter audit listed outside the intent tree:
// every directory, with receipts/ and the request shards listed but not
// entered, and every regular file with its listed size. Evidence and pinned
// blobs are listed but not read; their content-addressed name is their
// digest. The physical observation names every file that was read. It is
// nil for every other result.
func (res *Result) WriterListing() map[string]ListedFile {
	if res == nil || res.Mode != ModeWriter {
		return nil
	}
	return res.listing
}

// WriterReplay reports whether requestID is retained in the journal a
// ModeWriter audit observed, and if so the original index entry and the
// ticket its receipt targeted, as AuditForMutation reports them
// (CAL-V0-190, proposed). A request posted in the walked tail was validated
// by the walk. A request posted at or before the checkpoint is served only
// when every byte the answer rests on is bound to the checkpoint:
//
//   - its afterimage is read from disk and its SHA-256 equals the digest the
//     checkpoint holds for its path;
//   - the receipt that posted it is reached from the checkpoint's receipt by
//     the Prev digests of at most MaxWriterTail receipts, each read and
//     hashed;
//   - that receipt posts exactly this afterimage under this request ID with
//     the request's sequence, outcome and codes.
//
// Every error means only that this route cannot serve the replay: the caller
// runs the complete route, which derives the replay or the refusal itself.
// The one exception is cleanup, a failed native close of these reads, which
// is terminal as it is for the audit (CAL-V0-116): err then carries it too.
func (r Reader) WriterReplay(res *Result, requestID string) (found bool, entry mutation.IndexEntry, ticketID string, cleanup, err error) {
	if res == nil || res.Mode != ModeWriter || res.writerBase == nil {
		return false, entry, "", nil, errCheckpoint("writer replay", "not a writer audit")
	}
	var native *nativeRead
	switch n := r.Source.(type) {
	case Native:
		native = newNativeRead(n)
	case *Native:
		if n != nil {
			native = newNativeRead(*n)
		}
	}
	if native != nil {
		r.Source = native
		defer func() {
			if cleanup = native.close(); cleanup != nil {
				found, entry, ticketID = false, mutation.IndexEntry{}, ""
				err = wire.Errorf(wire.CodeUnsupportedFilesystem, "read lifetime", "%v; close: %v", err, cleanup)
			}
		}()
	}
	if res.request != nil {
		if res.request.Entry.RequestID != requestID {
			return false, entry, "", nil, errCheckpoint("requests", "walked request differs")
		}
		return true, res.request.Entry, res.requestTicket, nil, nil
	}
	rp, err := snapshot.RequestPath(requestID)
	if err != nil {
		return false, entry, "", nil, err
	}
	wc := res.writerBase
	want, ok := wc.RequestAfterimage(rp)
	if !ok {
		return false, entry, "", nil, nil
	}
	bound, err := snapshot.PostBound(rp)
	if err != nil {
		return false, entry, "", nil, err
	}
	raw, err := requiredRead(r.Source, rp, bound)
	if err != nil {
		return false, entry, "", nil, err
	}
	if wire.Sum(raw) != want {
		return false, entry, "", nil, errCheckpoint(rp, "request afterimage differs from the checkpoint")
	}
	req, err := snapshot.DecodeRequest(raw)
	if err != nil {
		return false, entry, "", nil, err
	}
	seq, last := req.Seq.Uint64(), wc.Seq.Uint64()
	if req.Entry.RequestID != requestID || seq < 1 || seq > last || last-seq > MaxWriterTail {
		return false, entry, "", nil, errCheckpoint(rp, "request is not within the bound before the checkpoint")
	}
	digest := wc.ReceiptSha256
	var rc *snapshot.Receipt
	for at := last; ; at-- {
		name, err := snapshot.ReceiptName(at)
		if err != nil {
			return false, entry, "", nil, err
		}
		body, err := requiredRead(r.Source, "receipts/"+name, wire.MaxReceiptFileBytes)
		if err != nil {
			return false, entry, "", nil, err
		}
		if wire.Sum(body) != digest {
			return false, entry, "", nil, errCheckpoint(name, "receipt is not on the checkpoint's chain")
		}
		rc, err = snapshot.DecodeReceipt(body)
		if err != nil {
			return false, entry, "", nil, err
		}
		if rc.Seq.Uint64() != at {
			return false, entry, "", nil, errCheckpoint(name, "receipt sequence differs")
		}
		if at == seq {
			break
		}
		if rc.Prev == nil {
			return false, entry, "", nil, errCheckpoint(name, "receipt chain ends early")
		}
		digest = *rc.Prev
	}
	posted := false
	for _, p := range rc.Post {
		posted = posted || (p.Path == rp && p.Sha256 != nil && *p.Sha256 == want)
	}
	if !posted || rc.RequestID == nil || *rc.RequestID != requestID || req.Entry.Outcome.Outcome != rc.Outcome || strings.Join(req.Entry.Outcome.Codes, "\x00") != strings.Join(rc.Codes, "\x00") {
		return false, entry, "", nil, errCheckpoint(rp, "request afterimage does not bind its receipt")
	}
	if rc.TicketID != nil {
		ticketID = rc.TicketID.Raw
	}
	return true, req.Entry, ticketID, nil, nil
}
