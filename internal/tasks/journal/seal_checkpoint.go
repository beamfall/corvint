package journal

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"

	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// SealValidates reports whether a seal of receipts first..last can resume
// the complete validators from w's base (JSG-V0-009): w has a base at its
// FullSeq, FullSeq lies in first-1..last-1, w reaches last, and the base's
// segments are the leading segments of the manifest being extended. A seal
// that does not validate still seals; FullSeq then stays where it is and
// the writer bound sends a later writer through the complete audit.
func (w *WriterCheckpoint) SealValidates(first, last uint64, sealed []snapshot.SegmentEntry) bool {
	if w == nil || w.base == nil || w.FullSeq != w.base.seq || w.FullSeq+1 < first || w.FullSeq >= last || w.Seq.Uint64() < last {
		return false
	}
	segs := w.base.meta.segments
	if len(segs) > len(sealed) {
		return false
	}
	for i, d := range segs {
		if hex.EncodeToString(d[:]) != string(sealed[i].Sha256) {
			return false
		}
	}
	return true
}

// segmentSource serves the receipts and request afterimages of the segment
// being sealed from the bytes the seal will write, and every other path
// from the store, so the validators check exactly what is sealed.
type segmentSource struct {
	inner Source
	files map[string][]byte
}

func (s segmentSource) Read(p string, max int) ([]byte, error) {
	if raw, ok := s.files[p]; ok {
		if len(raw) > max {
			return nil, errCheckpoint(p, "larger than its bound")
		}
		return raw, nil
	}
	return s.inner.Read(p, max)
}

func (s segmentSource) List(p string, max int) (Listing, error) { return s.inner.List(p, max) }

// SealCheckpoint runs the complete audit's per-receipt validators over
// receipts FullSeq+1..seg.Last of the segment being sealed, resumed from w's
// base, and returns w rebased on a new base at seg.Last whose FullSeq is
// seg.Last (JSG-V0-009). head is the store's head, which is past seg.Last;
// sealed is the manifest's segments once seg is sealed. Any receipt the
// validators refuse, a chain that does not reach the segment's last digest,
// or a new base that differs from w at seg.Last returns an error: the seal
// declines and the caller invalidates the checkpoint, so the next writer's
// complete audit classifies the fault.
func (r Reader) SealCheckpoint(head *snapshot.Head, w *WriterCheckpoint, seg *snapshot.Segment, sealed []snapshot.SegmentEntry) (*WriterCheckpoint, error) {
	if head == nil || seg == nil || len(sealed) == 0 || sealed[len(sealed)-1].Last != seg.Last || !w.SealValidates(seg.First, seg.Last, sealed[:len(sealed)-1]) || head.LastSeq.Uint64() <= seg.Last {
		return nil, errCheckpoint("writer checkpoint", "seal range is not validated from the base")
	}
	b := w.base
	m := &b.meta
	bc := &WriterCheckpoint{Checkpoint: m.header, FullSeq: b.seq, ReceiptBytes: m.receiptBytes, Cost: m.cost, Intent: m.intent, Invalidation: w.Invalidation, base: b, total: b.n}
	bc.Entries = nil
	files := make(map[string][]byte, len(seg.Receipts)+len(seg.Requests))
	physical := make(map[string]PhysicalFile, len(seg.Receipts)+len(seg.Requests))
	for _, f := range append(append([]snapshot.SegmentFile(nil), seg.Receipts...), seg.Requests...) {
		files[f.Path] = f.Raw
		physical[f.Path] = PhysicalFile{Sha256: wire.Sum(f.Raw), Bytes: len(f.Raw)}
	}
	r.Source = segmentSource{inner: r.Source, files: files}
	o := &observation{head: head}
	result := &Result{Head: head, Mode: ModeWriter, Records: map[string]Record{}, StructuralConsistency: "NOT_OBSERVED", SemanticCoverage: bc.SemanticCoverage}
	from := b.seq
	st := &chain{canonical: map[string]latest{}, prev: &bc.ReceiptSha256, generation: bc.Generation.Uint64(), elided: bc, resumed: from, posted: map[string]bool{}, view: true, primary: bc.PrimaryWorktree}
	result.LastSeq, result.LastReceiptSha256 = bc.Seq, bc.ReceiptSha256
	for seq := from + 1; seq <= seg.Last; seq++ {
		name, err := snapshot.ReceiptName(seq)
		if err != nil {
			return nil, err
		}
		if err := r.step(o, st, result, name, seq, head.LastSeq.Uint64(), nil, "", profileLimits); err != nil {
			return nil, err
		}
	}
	if st.prev == nil || *st.prev != sealed[len(sealed)-1].LastReceiptSha256 || result.LastSeq.Uint64() != seg.Last {
		return nil, errCheckpoint("writer checkpoint", "validated chain does not reach the segment's last receipt")
	}
	at := *head
	at.LastSeq, at.Generation = result.LastSeq, wire.SizeOf(st.generation)
	digest := result.LastReceiptSha256
	at.LastReceiptSha256 = &digest
	result.Head = &at
	result.Mode, result.StructuralConsistency = ModeWriter, ModeWriter
	result.chain, result.writerBase, result.sealRebase = st, bc, true
	nb, err := result.WriterCheckpoint(physical)
	if err != nil {
		return nil, err
	}
	nb.FullSeq = seg.Last
	nb.segments = make([][sha256.Size]byte, len(sealed))
	for i, e := range sealed {
		d, err := hex.DecodeString(string(e.Sha256))
		if err != nil || len(d) != sha256.Size {
			return nil, errCheckpoint("writer checkpoint", "segment digest is not canonical")
		}
		nb.segments[i] = [sha256.Size]byte(d)
	}
	return rebase(w, nb)
}

// rebase returns w on nb's entries as its new base: nb is the base-free
// checkpoint the validators derived at an earlier sequence S'. Every entry,
// request and note w holds at or before S' must be nb's, and nb holds
// nothing w does not; the delta is w's entries after S' and the requests
// and notes nb does not hold. w's header, aggregates and invalidation digest
// are kept, its FullSeq becomes S', and the result is encoded and decoded
// as a self-check.
func rebase(w, nb *WriterCheckpoint) (*WriterCheckpoint, error) {
	bad := func(why string) error { return errCheckpoint("writer checkpoint", "seal rebase: "+why) }
	s := nb.Seq.Uint64()
	f, err := w.full()
	if err != nil {
		return nil, err
	}
	if nb.base != nil || len(nb.sizes) != len(nb.Entries) || len(f.sizes) != len(f.Entries) || nb.QueueID != w.QueueID || nb.PrimaryWorktree != w.PrimaryWorktree || nb.InitSha256 != w.InitSha256 {
		return nil, bad("checkpoints are not comparable")
	}
	baseRaw, err := encodeBase(nb)
	if err != nil {
		return nil, err
	}
	bt, err := decodeBase(baseRaw)
	if err != nil {
		return nil, err
	}
	out := &WriterCheckpoint{Checkpoint: w.Checkpoint, FullSeq: s, ReceiptBytes: w.ReceiptBytes, Cost: w.Cost, Invalidation: w.Invalidation, Intent: w.Intent, base: bt, freshBase: true, total: bt.n}
	out.Entries = nil
	i, j := 0, 0
	for i < len(f.Entries) || j < len(nb.Entries) {
		switch {
		case j == len(nb.Entries) || (i < len(f.Entries) && f.Entries[i].Path < nb.Entries[j].Path):
			// A path first posted after S'.
			if f.Entries[i].Seq.Uint64() <= s {
				return nil, bad(f.Entries[i].Path + " is absent from the validated base")
			}
			out.total++
			out.appendDelta(f, i)
			i++
		case i == len(f.Entries) || nb.Entries[j].Path < f.Entries[i].Path:
			return nil, bad(nb.Entries[j].Path + " is not held by the checkpoint")
		default:
			fe, ne := f.Entries[i], nb.Entries[j]
			if fe.Seq.Uint64() > s {
				out.appendDelta(f, i)
			} else {
				fd, fok := f.note(fe.Path)
				nd, nok := nb.note(ne.Path)
				if fe.Seq != ne.Seq || !equalDigest(fe.Sha256, ne.Sha256) || f.sizes[i] != nb.sizes[j] || fok != nok || fd != nd {
					return nil, bad(fe.Path + " differs from the validated base")
				}
			}
			i++
			j++
		}
	}
	for k := 0; k < len(nb.requests); k += writerRequestBytes {
		after, ok := findRequest(f.requests, nb.requests[k:k+sha256.Size])
		if !ok || !bytes.Equal(after, nb.requests[k+sha256.Size:k+writerRequestBytes]) {
			return nil, bad("a validated request is not held by the checkpoint")
		}
	}
	for k := 0; k < len(f.requests); k += writerRequestBytes {
		if _, ok := bt.request(f.requests[k : k+sha256.Size]); !ok {
			out.requests = append(out.requests, f.requests[k:k+writerRequestBytes]...)
		}
	}
	base, delta, err := out.Encode()
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(base, baseRaw) {
		return nil, bad("base does not encode")
	}
	check, err := DecodeWriterCheckpoint(delta, base)
	if err != nil {
		return nil, err
	}
	if check.FullSeq != s || check.count() != w.count() || check.Requests() != w.Requests() || check.Intent != w.Intent {
		return nil, bad("rebased checkpoint differs from the checkpoint")
	}
	check.freshBase = true
	return check, nil
}

// appendDelta appends f's entry i, its size and its note to w's delta.
func (w *WriterCheckpoint) appendDelta(f *WriterCheckpoint, i int) {
	e := f.Entries[i]
	w.Entries = append(w.Entries, e)
	w.sizes = append(w.sizes, f.sizes[i])
	if d, ok := f.note(e.Path); ok {
		w.notes = append(w.notes, writerNote{entry: uint32(len(w.Entries) - 1), digest: d})
	}
}

// RebaseOnSealedBase is the base-at-S capture (LGL-V0-891, V1-2621). w is the
// checkpoint a complete audit derived from receipt 1 at its head on a
// segmented store whose manifest seals through S, and old is the bound
// checkpoint whose base is at S with FullSeq S, the base a seal rebased. It
// returns w on old's base: FullSeq S, every entry, request and note posted
// after S in the delta, so a later command still finds the sealed base
// (LGL-V0-009). rebase refuses unless everything w holds at or before S is
// exactly old's base, and the base's segment digests must be w's, so old's
// base is checked against the complete audit rather than trusted.
func (w *WriterCheckpoint) RebaseOnSealedBase(old *WriterCheckpoint, through uint64) (*WriterCheckpoint, error) {
	if w == nil || w.base != nil || old == nil || old.base == nil || through == 0 || old.FullSeq != through || old.base.seq != through || w.Seq.Uint64() < through {
		return nil, errCheckpoint("writer checkpoint", "no complete checkpoint and sealed base at the sealed sequence")
	}
	b := old.base
	m := &b.meta
	if len(m.segments) != len(w.segments) {
		return nil, errCheckpoint("writer checkpoint", "sealed base segments differ from the manifest")
	}
	for i := range m.segments {
		if m.segments[i] != w.segments[i] {
			return nil, errCheckpoint("writer checkpoint", "sealed base segments differ from the manifest")
		}
	}
	bc := &WriterCheckpoint{Checkpoint: m.header, FullSeq: b.seq, ReceiptBytes: m.receiptBytes, Cost: m.cost, Intent: m.intent, Invalidation: old.Invalidation, base: b, total: b.n}
	bc.Entries = nil
	nb, err := bc.full()
	if err != nil {
		return nil, err
	}
	nb.segments = m.segments
	return rebase(w, nb)
}
