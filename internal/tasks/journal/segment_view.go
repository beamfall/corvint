package journal

import (
	"os"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// segmentView serves receipts 1..S and the request afterimages they posted
// from the segments a taskman-journal-head/1 binds, by their logical
// receipts/ and requests/ paths, so every walk validates sealed files with
// the validators it applies to active ones (JSG-V0-012). Every other path
// is read from inner. Each segment is read through inner at most once per
// audit attempt and its digest and size are checked against the manifest
// before any byte of it is served.
type segmentView struct {
	inner    Source
	manifest *snapshot.SegmentManifest
	// current is the most recently loaded segment's receipts by sequence.
	current      []snapshot.SegmentFile
	currentFirst uint64
	loaded       []bool
	// requests holds every sealed request afterimage of a loaded segment.
	requests map[string][]byte
}

func newSegmentView(inner Source, m *snapshot.SegmentManifest) *segmentView {
	return &segmentView{inner: inner, manifest: m, loaded: make([]bool, len(m.Segments)), requests: map[string][]byte{}}
}

// nativeOf returns the native read session beneath s, if any.
func nativeOf(s Source) (*nativeRead, bool) {
	if v, ok := s.(*segmentView); ok {
		s = v.inner
	}
	n, ok := s.(*nativeRead)
	return n, ok
}

func (v *segmentView) List(p string, max int) (Listing, error) { return v.inner.List(p, max) }

func (v *segmentView) Read(p string, max int) ([]byte, error) {
	if name, ok := strings.CutPrefix(p, "receipts/"); ok {
		if seq, ok := receiptNameSeq(name); ok && seq <= v.manifest.Through {
			raw, err := v.receipt(seq)
			return v.serve(p, raw, max, err)
		}
		return v.inner.Read(p, max)
	}
	if !strings.HasPrefix(p, "requests/") {
		return v.inner.Read(p, max)
	}
	if raw, ok := v.requests[p]; ok {
		return v.serve(p, raw, max, nil)
	}
	raw, err := v.inner.Read(p, max)
	if !os.IsNotExist(err) {
		return raw, err
	}
	// A request not yet served may be sealed in a segment not yet loaded.
	for i := range v.loaded {
		if !v.loaded[i] {
			if err := v.load(i); err != nil {
				return nil, err
			}
			if raw, ok := v.requests[p]; ok {
				return v.serve(p, raw, max, nil)
			}
		}
	}
	return nil, err
}

// sealed reports whether p is a request afterimage of a loaded segment.
func (v *segmentView) sealed(p string) bool {
	_, ok := v.requests[p]
	return ok
}

// serve applies the read bound and records the logical file as consumed, as
// a native read records a physical one, so derived checkpoints charge it.
func (v *segmentView) serve(p string, raw []byte, max int, err error) ([]byte, error) {
	if err != nil {
		return nil, err
	}
	if len(raw) > max {
		return nil, wire.Errorf(wire.CodeLimitExceeded, p, "file exceeds bound")
	}
	if n, ok := v.inner.(*nativeRead); ok && n.physical != nil {
		if err := n.physical.add(p, raw); err != nil {
			return nil, err
		}
	}
	return raw, nil
}

func (v *segmentView) receipt(seq uint64) ([]byte, error) {
	first := v.manifest.Segments[0].First
	i := int((seq - first) / snapshot.SegmentReceipts)
	if v.current == nil || seq < v.currentFirst || seq-v.currentFirst >= uint64(len(v.current)) {
		if err := v.load(i); err != nil {
			return nil, err
		}
	}
	return v.current[seq-v.currentFirst].Raw, nil
}

// load reads segment i, checks its size, digest, last receipt digest and
// request count against the manifest, and makes it current.
func (v *segmentView) load(i int) error {
	e := v.manifest.Segments[i]
	name := snapshot.SegmentName(e.First, e.Last)
	raw, err := v.inner.Read(name, snapshot.MaxSegmentBytes)
	if os.IsNotExist(err) {
		return wire.Errorf(wire.CodeJournalForked, name, "bound segment absent")
	}
	if err != nil {
		return err
	}
	if uint64(len(raw)) != e.Bytes || wire.Sum(raw) != e.Sha256 {
		return wire.Errorf(wire.CodeJournalForked, name, "segment differs from the digest the manifest binds")
	}
	s, err := snapshot.DecodeSegment(raw, e.First, e.Last)
	if err != nil {
		return err
	}
	// An elided receipt's chain digest is not its file digest.
	_, last, _, err := snapshot.DecodeRetainedReceipt(s.Receipts[len(s.Receipts)-1].Raw)
	if err != nil {
		return err
	}
	if last != e.LastReceiptSha256 || uint64(len(s.Requests)) != e.Requests {
		return wire.Errorf(wire.CodeJournalForked, name, "segment differs from its manifest entry")
	}
	if !v.loaded[i] {
		for _, f := range s.Requests {
			if _, dup := v.requests[f.Path]; dup {
				return wire.Errorf(wire.CodeJournalForked, f.Path, "request sealed in more than one segment")
			}
			v.requests[f.Path] = append([]byte(nil), f.Raw...)
		}
		v.loaded[i] = true
	}
	v.current, v.currentFirst = s.Receipts, e.First
	return nil
}

// segments reads the manifest a taskman-journal-head/1 binds and checks
// segments/ and the listed receipts against it (JSG-V0-006, JSG-V0-007). A
// seal marker is a pending seal: only a writer settles it (JSG-V0-008).
// Every other disagreement is returned for the walk to report, so a seal
// that ran between this capture's listing and its head read is a moved
// observation once the second capture differs. A full capture's receipt
// names gain the logical names 1..S the view serves.
func (r Reader) segments(o *observation) (deferred error, err error) {
	if _, ok := o.files[snapshot.SealMarkerPath]; ok {
		return nil, wire.Errorf(wire.CodeRedoPending, snapshot.SealMarkerPath, "a seal is not settled yet; a mutating command will settle it")
	}
	listed := map[string]os.FileInfo{}
	for p, info := range o.files {
		if strings.HasPrefix(p, snapshot.SegmentsDir+"/") {
			listed[p] = info
		}
	}
	forked := func(p, why string) error { return wire.Errorf(wire.CodeJournalForked, p, "%s", why) }
	if o.head == nil || o.head.Segments == nil {
		if len(listed) != 0 {
			return forked(snapshot.SegmentsDir, "segments without a segmented head"), nil
		}
		return nil, nil
	}
	through := o.head.Segments.Through.Uint64()
	if o.seal != nil {
		return wire.Errorf(wire.CodeUnsupported, snapshot.SegmentsDir, "a segmented journal with a retired prefix is unsupported"), nil
	}
	name := snapshot.SegmentManifestName(through)
	info, ok := listed[name]
	if !ok || info.IsDir() {
		return forked(name, "bound segment manifest absent"), nil
	}
	raw, err := requiredRead(r.Source, name, snapshot.MaxSegmentManifestBytes)
	if err != nil {
		if os.IsNotExist(err) || wire.CodeOf(err) == wire.CodeJournalForked {
			return forked(name, "bound segment manifest absent"), nil
		}
		return nil, err
	}
	if wire.Sum(raw) != o.head.Segments.ManifestSha256 || int64(len(raw)) != info.Size() {
		return forked(name, "segment manifest differs from the digest the head binds"), nil
	}
	m, err := snapshot.DecodeSegmentManifest(raw, 1)
	if err != nil {
		return err, nil
	}
	if len(m.Segments) == 0 || m.Through != through || m.QueueID != o.head.QueueID {
		return forked(name, "segment manifest differs from the head"), nil
	}
	want := len(m.Segments) + 1
	for _, e := range m.Segments {
		info, ok := listed[snapshot.SegmentName(e.First, e.Last)]
		if !ok || info.IsDir() || uint64(info.Size()) != e.Bytes {
			return forked(snapshot.SegmentName(e.First, e.Last), "bound segment absent or resized"), nil
		}
	}
	if len(listed) != want {
		return forked(snapshot.SegmentsDir, "segments/ holds a file the manifest does not bind"), nil
	}
	o.manifest = m
	if len(o.receipts) != 0 {
		if seq, _ := receiptNameSeq(o.receipts[0]); seq <= through {
			return forked("receipts/"+o.receipts[0], "a sealed receipt is still linked"), nil
		}
	}
	if !o.lite && !o.writer {
		names := make([]string, 0, int(through)+len(o.receipts))
		for n := uint64(1); n <= through; n++ {
			s, err := snapshot.ReceiptName(n)
			if err != nil {
				return nil, err
			}
			names = append(names, s)
		}
		o.receipts = append(names, o.receipts...)
	}
	return nil, nil
}
