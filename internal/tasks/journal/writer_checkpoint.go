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
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// ProfileWriterCheckpoint names the derived writer checkpoint (CAL-V0-115,
// proposed). Like the read checkpoint it is not journal state: no receipt
// posts it, no archive carries it, and a missing, stale, torn or foreign file
// only sends the next writer through the complete audit.
const ProfileWriterCheckpoint = "taskman-writer-checkpoint/0"

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
)

const writerMagic = ProfileWriterCheckpoint + "\n"

// MaxWriterCheckpointBytes bounds the file a writer is willing to consume.
const MaxWriterCheckpointBytes = len(writerMagic) + 8 + MaxCheckpointBytes + 7*8 + 32*wire.MaxArchiveFiles + 8 + writerNoteBytes*wire.MaxTicketsPerQueue + sha256.Size

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
// one at Seq, which the tail binds as the complete audit does (ON-V0-006).
type WriterCheckpoint struct {
	Checkpoint
	// FullSeq is the head of the complete audit this checkpoint descends from.
	FullSeq uint64
	// ReceiptBytes is the payload of receipts 1..Seq.
	ReceiptBytes uint64
	// Cost is the archive cost of receipts 1..Seq and of every request
	// afterimage they posted.
	Cost     archive.FileSetCost
	requests []byte // sorted 32-byte request path digests
	notes    []writerNote
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
// Seq, if it carried one.
func (w *WriterCheckpoint) note(p string) ([sha256.Size]byte, bool) {
	e := sort.Search(len(w.Entries), func(i int) bool { return w.Entries[i].Path >= p })
	if e == len(w.Entries) || w.Entries[e].Path != p {
		return [sha256.Size]byte{}, false
	}
	i := sort.Search(len(w.notes), func(i int) bool { return w.notes[i].entry >= uint32(e) })
	if i == len(w.notes) || w.notes[i].entry != uint32(e) {
		return [sha256.Size]byte{}, false
	}
	return w.notes[i].digest, true
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

// Requests is the number of request afterimages receipts 1..Seq posted.
func (w *WriterCheckpoint) Requests() uint64 { return uint64(len(w.requests) / sha256.Size) }

// HasRequest reports whether p is a request path posted at or before Seq.
func (w *WriterCheckpoint) HasRequest(p string) bool {
	d, ok := requestDigest(p)
	if !ok {
		return false
	}
	n := len(w.requests) / sha256.Size
	i := sort.Search(n, func(i int) bool { return bytes.Compare(w.requests[i*sha256.Size:(i+1)*sha256.Size], d) >= 0 })
	return i < n && bytes.Equal(w.requests[i*sha256.Size:(i+1)*sha256.Size], d)
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

// Encode renders the file bytes: a magic line, the read checkpoint encoding,
// the aggregates, the request digests, the note count and notes, and a
// SHA-256 trailer over all of it.
func (w *WriterCheckpoint) Encode() []byte {
	cp := w.Checkpoint.Encode()
	buf := make([]byte, 0, len(writerMagic)+8+len(cp)+7*8+len(w.requests)+8+writerNoteBytes*len(w.notes)+sha256.Size)
	buf = append(buf, writerMagic...)
	buf = binary.BigEndian.AppendUint64(buf, uint64(len(cp)))
	buf = append(buf, cp...)
	for _, v := range []uint64{w.FullSeq, w.ReceiptBytes, uint64(w.Cost.Files), w.Cost.PayloadBytes, w.Cost.EntryBytes, w.Cost.TarBytes, w.Requests()} {
		buf = binary.BigEndian.AppendUint64(buf, v)
	}
	buf = append(buf, w.requests...)
	buf = binary.BigEndian.AppendUint64(buf, uint64(len(w.notes)))
	for _, n := range w.notes {
		buf = binary.BigEndian.AppendUint32(buf, n.entry)
		buf = append(buf, n.digest[:]...)
	}
	sum := sha256.Sum256(buf)
	return append(buf, sum[:]...)
}

// DecodeWriterCheckpoint parses writer checkpoint bytes. Any error means only
// that the checkpoint is unusable; the writer runs the complete audit.
func DecodeWriterCheckpoint(raw []byte) (*WriterCheckpoint, error) {
	bad := func(why string) error { return errCheckpoint("writer checkpoint", why) }
	if len(raw) > MaxWriterCheckpointBytes {
		return nil, bad("larger than its bound")
	}
	if len(raw) < len(writerMagic)+8+7*8+8+sha256.Size {
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
	n := binary.BigEndian.Uint64(rest)
	rest = rest[8:]
	if n > MaxCheckpointBytes || n > uint64(len(rest)) {
		return nil, bad("embedded checkpoint length")
	}
	cp, err := DecodeCheckpoint(rest[:n])
	if err != nil {
		return nil, err
	}
	rest = rest[n:]
	if len(rest) < 7*8 {
		return nil, bad("truncated aggregates")
	}
	v := make([]uint64, 7)
	for i := range v {
		v[i] = binary.BigEndian.Uint64(rest[i*8:])
	}
	rest = rest[7*8:]
	w := &WriterCheckpoint{Checkpoint: *cp, FullSeq: v[0], ReceiptBytes: v[1], Cost: archive.FileSetCost{PayloadBytes: v[3], EntryBytes: v[4], TarBytes: v[5]}}
	requests, seq := v[6], cp.Seq.Uint64()
	if v[2] > math.MaxInt || requests > uint64(wire.MaxArchiveFiles) || uint64(len(rest)) < requests*sha256.Size+8 {
		return nil, bad("request digest count")
	}
	w.Cost.Files = int(v[2])
	if seq > uint64(wire.MaxArchiveFiles) || v[2] != seq+requests {
		return nil, bad("file count differs from receipts plus requests")
	}
	if w.FullSeq < 1 || w.FullSeq > seq || w.ReceiptBytes > w.Cost.PayloadBytes {
		return nil, bad("aggregate bounds")
	}
	digests, rest := rest[:requests*sha256.Size], rest[requests*sha256.Size:]
	for i := sha256.Size; i < len(digests); i += sha256.Size {
		if bytes.Compare(digests[i-sha256.Size:i], digests[i:i+sha256.Size]) >= 0 {
			return nil, bad("request digests are not strictly ordered")
		}
	}
	w.requests = append([]byte(nil), digests...)
	notes := binary.BigEndian.Uint64(rest)
	rest = rest[8:]
	if notes > uint64(wire.MaxTicketsPerQueue) || uint64(len(rest)) != notes*writerNoteBytes {
		return nil, bad("note count")
	}
	w.notes = make([]writerNote, notes)
	for i := range w.notes {
		n := &w.notes[i]
		n.entry = binary.BigEndian.Uint32(rest[i*writerNoteBytes:])
		copy(n.digest[:], rest[i*writerNoteBytes+4:(i+1)*writerNoteBytes])
		if (i > 0 && n.entry <= w.notes[i-1].entry) || n.entry >= uint32(len(cp.Entries)) {
			return nil, bad("note entries are not strictly ordered checkpoint entries")
		}
		if e := cp.Entries[n.entry]; !strings.HasPrefix(e.Path, "intent/tickets/") || e.Sha256 == nil {
			return nil, bad("note entry is not a live ticket")
		}
	}
	return w, nil
}

// WriterCheckpoint derives the writer checkpoint this result supports from
// the bytes its audit physically read. A complete audit (ModeFull) derives it
// from receipt 1 and sets FullSeq to its head; a writer audit (ModeWriter)
// extends the checkpoint it resumed from by the tail it walked and keeps that
// FullSeq. Every other observation, and any receipt or request afterimage the
// audit did not read, returns an error: nothing is derived from inference.
func (res *Result) WriterCheckpoint(physical map[string]PhysicalFile) (*WriterCheckpoint, error) {
	if res == nil || res.chain == nil || res.Pending || res.StagingPresent || res.Head == nil || res.LastSeq != res.Head.LastSeq || physical == nil {
		return nil, errCheckpoint("writer checkpoint", "observation is not a settled audited head")
	}
	var w *WriterCheckpoint
	from := uint64(1)
	switch {
	case res.Mode == ModeFull && res.StructuralConsistency == "CONSISTENT":
		w = &WriterCheckpoint{FullSeq: res.LastSeq.Uint64()}
	case res.Mode == ModeWriter && res.StructuralConsistency == ModeWriter && res.writerBase != nil:
		base := res.writerBase
		w = &WriterCheckpoint{FullSeq: base.FullSeq, ReceiptBytes: base.ReceiptBytes, Cost: base.Cost}
		from = base.Seq.Uint64() + 1
	default:
		return nil, errCheckpoint("writer checkpoint", "observation is not a complete or writer audit")
	}
	w.Checkpoint = res.chainCheckpoint()
	if err := w.deriveNotes(res); err != nil {
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
		if err := w.charge(physical, p, l.digest); err != nil {
			return nil, err
		}
		digests = append(digests, d)
	}
	if res.Mode == ModeWriter {
		for i := 0; i < len(res.writerBase.requests); i += sha256.Size {
			digests = append(digests, res.writerBase.requests[i:i+sha256.Size])
		}
	}
	sort.Slice(digests, func(i, j int) bool { return bytes.Compare(digests[i], digests[j]) < 0 })
	w.requests = make([]byte, 0, len(digests)*sha256.Size)
	for i, d := range digests {
		if i > 0 && bytes.Equal(digests[i-1], d) {
			return nil, errCheckpoint("requests", "request path repeats")
		}
		w.requests = append(w.requests, d...)
	}
	return w, nil
}

// deriveNotes records the note reference of every live ticket that carries
// one at Seq: the reference the audit walked, or, for a ticket a writer
// audit's tail did not post, the one its base checkpoint carries. A ticket
// the tail posted must have been walked; anything else is not derived.
func (w *WriterCheckpoint) deriveNotes(res *Result) error {
	var refs map[string]*ticket.OperatorNoteReference
	if res.chain.notes != nil {
		refs = res.chain.notes.refs
	}
	base := res.writerBase
	if res.Mode != ModeWriter {
		base = nil
	}
	candidates := make([]string, 0, len(refs))
	for p, ref := range refs {
		if ref != nil {
			candidates = append(candidates, p)
		}
	}
	if base != nil {
		for _, n := range base.notes {
			candidates = append(candidates, base.Entries[n.entry].Path)
		}
	}
	var notes []writerNote
	for _, p := range candidates {
		e := sort.Search(len(w.Entries), func(i int) bool { return w.Entries[i].Path >= p })
		if e == len(w.Entries) || w.Entries[e].Path != p || w.Entries[e].Sha256 == nil || !strings.HasPrefix(p, "intent/tickets/") {
			continue
		}
		var digest [sha256.Size]byte
		if base != nil && w.Entries[e].Seq.Uint64() <= base.Seq.Uint64() {
			d, ok := base.note(p)
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

// chainCheckpoint is the read checkpoint body for this result's chain.
func (res *Result) chainCheckpoint() Checkpoint {
	cp := Checkpoint{QueueID: res.Head.QueueID, PrimaryWorktree: res.Head.PrimaryWorktree, Seq: res.LastSeq, ReceiptSha256: res.LastReceiptSha256, Generation: res.Head.Generation, InitSha256: res.Head.InitSha256, SemanticCoverage: res.SemanticCoverage}
	for p, l := range res.chain.canonical {
		if strings.HasPrefix(p, "requests/") {
			continue
		}
		cp.Entries = append(cp.Entries, CheckpointEntry{Path: p, Seq: l.seq, Sha256: l.digest})
	}
	sort.Slice(cp.Entries, func(i, j int) bool { return cp.Entries[i].Path < cp.Entries[j].Path })
	return cp
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
// does. requestID, when set, must not be retained anywhere: a writer that
// finds its own request replays through the complete audit.
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

// walkWriter is AuditForWriter's body; see there.
func (r Reader) walkWriter(o *observation, wc *WriterCheckpoint, selected map[string]bool, request string, lim limits) (*Result, error) {
	result := &Result{Identity: o.identity, Head: o.head, Mode: ModeWriter, Records: map[string]Record{}, StructuralConsistency: "NOT_OBSERVED", ProjectionAgreement: "NOT_OBSERVED", SemanticCoverage: "NOT_OBSERVED", HistoricalAcceptance: "NOT_OBSERVED", ActorAuthentication: "NOT_OBSERVED", Liveness: "NOT_OBSERVED", RuntimeQualification: "NOT_OBSERVED"}
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
	if from > headSeq || wc.Generation.Uint64() > head.Generation.Uint64() || len(wc.Entries) > maxCheckpointEntries() {
		return result, errCheckpoint("/seq", "checkpoint is not a prefix of head")
	}
	if headSeq-from > MaxWriterTail || headSeq-wc.FullSeq >= WriterFullBound {
		return result, errCheckpoint("/seq", "tail exceeds the writer bound")
	}
	// Names validated by receiptSeq are canonical and unique, so a count of
	// head with first 1 and last head is exactly 1..head. A pending or extra
	// receipt is classified by the complete audit.
	if uint64(len(o.receipts)) != headSeq {
		return result, errCheckpoint("receipts", "receipt names differ from head")
	}
	first, _ := snapshot.ReceiptName(1)
	last, err := snapshot.ReceiptName(headSeq)
	if err != nil {
		return result, err
	}
	if o.receipts[0] != first || o.receipts[len(o.receipts)-1] != last {
		return result, errCheckpoint("receipts", "receipt names differ from head")
	}
	if request != "" {
		if rp, _ := snapshot.RequestPath(request); wc.HasRequest(rp) {
			return result, errCheckpoint(rp, "request is retained before the checkpoint")
		}
	}
	name, err := snapshot.ReceiptName(from)
	if err != nil {
		return result, err
	}
	raw, err := requiredRead(r.Source, "receipts/"+name, wire.MaxReceiptFileBytes)
	if err != nil {
		return result, err
	}
	if wire.Sum(raw) != wc.ReceiptSha256 {
		return result, errCheckpoint(name, "retained receipt differs")
	}
	if rc, err := snapshot.DecodeReceipt(raw); err != nil || rc.Seq.Uint64() != from || rc.HeadGeneration != wc.Generation {
		return result, errCheckpoint(name, "retained receipt sequence/generation differs")
	}
	if from == headSeq && (*head.LastReceiptSha256 != wc.ReceiptSha256 || head.Generation != wc.Generation) {
		return result, errCheckpoint("head.json", "head digest/generation differs")
	}
	st := &chain{canonical: make(map[string]latest, len(wc.Entries)), prev: &wc.ReceiptSha256, generation: wc.Generation.Uint64(), elided: wc}
	for _, e := range wc.Entries {
		st.canonical[e.Path] = latest{seq: e.Seq, digest: e.Sha256}
	}
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
	if result.request != nil {
		return result, errCheckpoint("requests", "request is retained in the tail")
	}
	result.StructuralConsistency = ModeWriter
	for _, p := range sortedPaths(st.canonical) {
		record := st.canonical[p]
		if strings.HasPrefix(p, "evidence/") || strings.HasPrefix(p, "pinned/") {
			// Content-addressed: the listed name is the digest.
			info, present := o.files[p]
			if present != (record.digest != nil) || (present && !info.Mode().IsRegular()) {
				return result, errCheckpoint(p, "blob presence differs from latest canonical afterimage")
			}
			continue
		}
		_, have := result.Records[p]
		need := !have && r.selects(p, selected)
		digest, raw, err := r.projected(o, p, need)
		if err != nil {
			return result, err
		}
		if !equalDigest(digest, record.digest) {
			return result, errCheckpoint(p, "projection differs from latest canonical afterimage")
		}
		if need {
			if len(raw) > lim.selected-st.selectedBytes {
				return result, errCheckpoint(p, "selected canonical bytes exceed the selection budget")
			}
			st.selectedBytes += len(raw)
			result.Records[p] = Record{Seq: record.seq, Sha256: record.digest, Raw: raw}
		}
	}
	if err := r.strays(o, st.canonical, true); err != nil {
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
	return result, nil
}

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
