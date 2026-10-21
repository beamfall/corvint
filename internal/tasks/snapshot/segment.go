package snapshot

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// Journal segment profiles and fixed names (JSG-V0-005, JSG-V0-006,
// JSG-V0-008).
const (
	ProfileSegment         = "taskman-segment/0"
	ProfileSegmentManifest = "taskman-segment-manifest/0"
	// ProfileSealMarker is the first line of a staged seal's marker.
	ProfileSealMarker = "taskman-seal-marker/0"
	// ProfileHeadSegmented is the head of a segmented store: the /0 fields
	// plus the bound manifest's S and digest.
	ProfileHeadSegmented = "taskman-journal-head/1"
	// SegmentReceipts is the fixed number of receipts in one segment.
	SegmentReceipts = 512
	// SegmentsDir holds the sealed segments, the bound manifest and, while a
	// seal runs, its marker.
	SegmentsDir = "segments"
	// SealMarkerName is the staged seal's marker under SegmentsDir.
	SealMarkerName = "SEAL_INCOMPLETE"
	// MaxSegmentBytes bounds one segment file: 512 receipts of at most
	// MaxReceiptFileBytes and their request afterimages.
	MaxSegmentBytes = SegmentReceipts*(wire.MaxReceiptFileBytes+wire.MaxAttemptRecordBytes) + 64*wire.KiB
	// MaxSegmentManifestBytes bounds a manifest: a saturated journal has
	// fewer than 2,000 segments.
	MaxSegmentManifestBytes = 1 * wire.MiB
	// MaxSealMarkerBytes bounds the seal marker.
	MaxSealMarkerBytes = 4 * wire.KiB
	// maxSegmentRequests bounds the request afterimages one segment frames.
	maxSegmentRequests = SegmentReceipts * 64
)

var segmentMagic = []byte(ProfileSegment + "\n")

// SegmentFile is one framed file of a segment: its store path and its exact
// bytes.
type SegmentFile struct {
	Path string
	Raw  []byte
}

// Segment is a decoded taskman-segment/0: receipts First..Last in sequence
// order and the request afterimages they posted, sorted by path.
type Segment struct {
	First, Last uint64
	Receipts    []SegmentFile
	Requests    []SegmentFile
}

// SegmentName renders `segments/<first>-<last>.seg` with twelve-digit
// sequence numbers, the receipt name width.
func SegmentName(first, last uint64) string {
	return fmt.Sprintf("%s/%012d-%012d.seg", SegmentsDir, first, last)
}

// SegmentManifestName renders `segments/manifest-<S>.json`.
func SegmentManifestName(through uint64) string {
	return fmt.Sprintf("%s/manifest-%012d.json", SegmentsDir, through)
}

// SealMarkerPath is the store path of the seal marker.
const SealMarkerPath = SegmentsDir + "/" + SealMarkerName

// ParseSegmentName returns the range a canonical segment file name (the
// name under SegmentsDir) encodes.
func ParseSegmentName(name string) (first, last uint64, ok bool) {
	if len(name) != 12+1+12+4 || name[12] != '-' || !strings.HasSuffix(name, ".seg") {
		return 0, 0, false
	}
	first, ok1 := digits12(name[:12])
	last, ok2 := digits12(name[13:25])
	return first, last, ok1 && ok2 && first >= 1 && last >= first
}

// ParseSegmentManifestName returns the S a canonical manifest file name
// (the name under SegmentsDir) encodes.
func ParseSegmentManifestName(name string) (uint64, bool) {
	if len(name) != len("manifest-")+12+len(".json") || !strings.HasPrefix(name, "manifest-") || !strings.HasSuffix(name, ".json") {
		return 0, false
	}
	return digits12(name[len("manifest-") : len("manifest-")+12])
}

func digits12(s string) (uint64, bool) {
	var n uint64
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < '0' || c > '9' {
			return 0, false
		}
		n = n*10 + uint64(c-'0')
	}
	return n, len(s) == 12
}

// EncodeSegment frames s: the magic, First, Last and the request count,
// then each receipt and each request as a length-prefixed path and a
// length-prefixed body. The receipts must be named First..Last in order and
// the requests sorted by path; nothing is inferred.
func EncodeSegment(s *Segment) ([]byte, error) {
	if err := s.check(); err != nil {
		return nil, err
	}
	n := len(segmentMagic) + 24
	for _, f := range append(append([]SegmentFile(nil), s.Receipts...), s.Requests...) {
		n += 2 + len(f.Path) + 4 + len(f.Raw)
	}
	if n > MaxSegmentBytes {
		return nil, wire.Errorf(wire.CodeLimitExceeded, SegmentName(s.First, s.Last), "segment larger than its bound")
	}
	buf := make([]byte, 0, n)
	buf = append(buf, segmentMagic...)
	buf = binary.BigEndian.AppendUint64(buf, s.First)
	buf = binary.BigEndian.AppendUint64(buf, s.Last)
	buf = binary.BigEndian.AppendUint64(buf, uint64(len(s.Requests)))
	for _, f := range s.Receipts {
		buf = appendSegmentFile(buf, f)
	}
	for _, f := range s.Requests {
		buf = appendSegmentFile(buf, f)
	}
	return buf, nil
}

func appendSegmentFile(buf []byte, f SegmentFile) []byte {
	buf = binary.BigEndian.AppendUint16(buf, uint16(len(f.Path)))
	buf = append(buf, f.Path...)
	buf = binary.BigEndian.AppendUint32(buf, uint32(len(f.Raw)))
	return append(buf, f.Raw...)
}

// DecodeSegment parses a taskman-segment/0 that must hold first..last. It
// checks the framing, the names and the bounds; the caller checks the
// file's digest against the manifest and validates every framed file.
func DecodeSegment(raw []byte, first, last uint64) (*Segment, error) {
	where := SegmentName(first, last)
	bad := func(why string) error { return wire.Errorf(wire.CodeJournalForked, where, "segment %s", why) }
	if len(raw) > MaxSegmentBytes {
		return nil, wire.Errorf(wire.CodeLimitExceeded, where, "segment larger than its bound")
	}
	if name := []byte(strings.TrimSuffix(ProfileSegment, "0")); bytes.HasPrefix(raw, name) && !bytes.HasPrefix(raw, segmentMagic) {
		return nil, wire.Errorf(wire.CodeUnsupportedVersion, where, "another taskman-segment version")
	}
	if !bytes.HasPrefix(raw, segmentMagic) || len(raw) < len(segmentMagic)+24 {
		return nil, bad("profile or header differs")
	}
	rest := raw[len(segmentMagic):]
	s := &Segment{First: binary.BigEndian.Uint64(rest), Last: binary.BigEndian.Uint64(rest[8:])}
	requests := binary.BigEndian.Uint64(rest[16:])
	rest = rest[24:]
	if s.First != first || s.Last != last || requests > maxSegmentRequests {
		return nil, bad("header differs from its name")
	}
	next := func() (SegmentFile, error) {
		if len(rest) < 2 {
			return SegmentFile{}, bad("truncated")
		}
		n := int(binary.BigEndian.Uint16(rest))
		if len(rest) < 2+n+4 {
			return SegmentFile{}, bad("truncated")
		}
		f := SegmentFile{Path: string(rest[2 : 2+n])}
		size := uint64(binary.BigEndian.Uint32(rest[2+n:]))
		rest = rest[2+n+4:]
		if size > uint64(len(rest)) {
			return SegmentFile{}, bad("truncated")
		}
		f.Raw, rest = rest[:size:size], rest[size:]
		return f, nil
	}
	s.Receipts = make([]SegmentFile, 0, last-first+1)
	for i := uint64(0); i <= last-first; i++ {
		f, err := next()
		if err != nil {
			return nil, err
		}
		s.Receipts = append(s.Receipts, f)
	}
	s.Requests = make([]SegmentFile, 0, requests)
	for i := uint64(0); i < requests; i++ {
		f, err := next()
		if err != nil {
			return nil, err
		}
		s.Requests = append(s.Requests, f)
	}
	if len(rest) != 0 {
		return nil, bad("trailing bytes")
	}
	if err := s.check(); err != nil {
		return nil, err
	}
	return s, nil
}

// check validates the receipt names, the request paths and their order and
// the per-file bounds.
func (s *Segment) check() error {
	where := SegmentName(s.First, s.Last)
	if s.First < 1 || s.Last < s.First || s.Last-s.First+1 != SegmentReceipts || uint64(len(s.Receipts)) != s.Last-s.First+1 || len(s.Requests) > maxSegmentRequests {
		return wire.Errorf(wire.CodeJournalForked, where, "segment range is not %d receipts", SegmentReceipts)
	}
	for i, f := range s.Receipts {
		name, err := ReceiptName(s.First + uint64(i))
		if err != nil {
			return err
		}
		if f.Path != "receipts/"+name || len(f.Raw) > wire.MaxReceiptFileBytes {
			return wire.Errorf(wire.CodeJournalForked, where, "segment receipt %d is not %s within its bound", i, name)
		}
	}
	for i, f := range s.Requests {
		if !requestFilePath(f.Path) || len(f.Raw) > wire.MaxAttemptRecordBytes || (i > 0 && s.Requests[i-1].Path >= f.Path) {
			return wire.Errorf(wire.CodeJournalForked, where, "segment request %q is not a sorted request path within its bound", f.Path)
		}
	}
	return nil
}

// requestFilePath says p is `requests/<2 hex>/<64 hex>.json` with the shard
// equal to the name's first two digits, the layout RequestPath produces.
func requestFilePath(p string) bool {
	rest, ok := strings.CutPrefix(p, "requests/")
	if !ok || len(rest) != 2+1+64+5 || rest[2] != '/' || !strings.HasSuffix(rest, ".json") || rest[:2] != rest[3:5] {
		return false
	}
	for _, c := range rest[3 : 3+64] {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

// SegmentEntry is one manifest row (JSG-V0-006).
type SegmentEntry struct {
	First, Last       uint64
	Sha256            wire.Digest
	Bytes             uint64
	LastReceiptSha256 wire.Digest
	Requests          uint64
}

// SegmentManifest is a decoded taskman-segment-manifest/0: the segments
// covering F..Through in order.
type SegmentManifest struct {
	QueueID  wire.QueueID
	Through  uint64
	Segments []SegmentEntry
}

// Value renders the manifest.
func (m *SegmentManifest) Value() wire.Value {
	rows := make([]wire.Value, 0, len(m.Segments))
	for _, e := range m.Segments {
		o := wire.NewObject()
		o.Set("first", wire.String(string(wire.SizeOf(e.First))))
		o.Set("last", wire.String(string(wire.SizeOf(e.Last))))
		o.Set("sha256", wire.String(string(e.Sha256)))
		o.Set("bytes", wire.String(string(wire.SizeOf(e.Bytes))))
		o.Set("lastReceiptSha256", wire.String(string(e.LastReceiptSha256)))
		o.Set("requests", wire.String(string(wire.SizeOf(e.Requests))))
		rows = append(rows, wire.ObjectValue(o))
	}
	o := wire.NewObject()
	o.Set("profile", wire.String(ProfileSegmentManifest))
	o.Set("queueId", wire.String(m.QueueID.Raw))
	o.Set("through", wire.String(string(wire.SizeOf(m.Through))))
	o.Set("segments", wire.Array(rows...))
	return wire.ObjectValue(o)
}

// DecodeSegmentManifest parses a manifest and checks that its segments are
// consecutive 512-receipt ranges from first through its S.
func DecodeSegmentManifest(raw []byte, first uint64) (*SegmentManifest, error) {
	if len(raw) > MaxSegmentManifestBytes {
		return nil, wire.Errorf(wire.CodeLimitExceeded, SegmentsDir, "segment manifest larger than its bound")
	}
	v, err := wire.Parse(raw)
	if err != nil {
		return nil, err
	}
	if err := wire.ProfileVersion("/profile", v, ProfileSegmentManifest); err != nil {
		return nil, err
	}
	r := wire.NewReader(v, "/")
	r.Closed("profile", "queueId", "through", "segments")
	if err := r.Err(); err != nil {
		return nil, err
	}
	if err := wire.CheckProfile("/profile", r.Field("profile").String(), ProfileSegmentManifest); err != nil {
		return nil, err
	}
	m := &SegmentManifest{QueueID: r.Field("queueId").QueueID(), Through: r.Field("through").Size().Uint64()}
	for _, row := range r.Field("segments").Array(-1, true) {
		row.Closed("first", "last", "sha256", "bytes", "lastReceiptSha256", "requests")
		m.Segments = append(m.Segments, SegmentEntry{
			First: row.Field("first").Size().Uint64(), Last: row.Field("last").Size().Uint64(),
			Sha256: row.Field("sha256").Digest(), Bytes: row.Field("bytes").Size().Uint64(),
			LastReceiptSha256: row.Field("lastReceiptSha256").Digest(), Requests: row.Field("requests").Size().Uint64(),
		})
	}
	if err := r.Err(); err != nil {
		return nil, err
	}
	at := first
	for _, e := range m.Segments {
		if e.First != at || e.Last != at+SegmentReceipts-1 || e.Bytes > MaxSegmentBytes || e.Requests > maxSegmentRequests {
			return nil, wire.Errorf(wire.CodeJournalForked, SegmentsDir, "manifest segment %d-%d is not the next %d-receipt range", e.First, e.Last, SegmentReceipts)
		}
		at = e.Last + 1
	}
	if m.Through != at-1 {
		return nil, wire.Errorf(wire.CodeJournalForked, SegmentsDir, "manifest S differs from its segments")
	}
	return m, nil
}

// SortSegmentFiles sorts framed files by path.
func SortSegmentFiles(files []SegmentFile) {
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
}

// SealMarker is the staged seal's marker (JSG-V0-008): the range sealed, the
// new segment's digest and the head digests before and after the seal.
type SealMarker struct {
	First, Last   uint64
	SegmentSha256 wire.Digest
	PriorHead     wire.Digest
	NextHead      wire.Digest
}

// EncodeSealMarker renders the marker as five fixed lines.
func EncodeSealMarker(m SealMarker) []byte {
	return []byte(fmt.Sprintf("%s\n%012d-%012d\n%s\n%s\n%s\n", ProfileSealMarker, m.First, m.Last, m.SegmentSha256, m.PriorHead, m.NextHead))
}

// DecodeSealMarker parses EncodeSealMarker's form exactly.
func DecodeSealMarker(raw []byte) (*SealMarker, error) {
	bad := wire.Errorf(wire.CodeJournalForked, SealMarkerPath, "seal marker is malformed")
	lines := strings.Split(string(raw), "\n")
	if name := strings.TrimSuffix(ProfileSealMarker, "0"); len(raw) <= MaxSealMarkerBytes && strings.HasPrefix(lines[0], name) && lines[0] != ProfileSealMarker {
		return nil, wire.Errorf(wire.CodeUnsupportedVersion, SealMarkerPath, "another taskman-seal-marker version")
	}
	if len(raw) > MaxSealMarkerBytes || len(lines) != 6 || lines[0] != ProfileSealMarker || lines[5] != "" {
		return nil, bad
	}
	first, last, ok := ParseSegmentName(lines[1] + ".seg")
	if !ok {
		return nil, bad
	}
	m := &SealMarker{First: first, Last: last}
	for i, d := range []*wire.Digest{&m.SegmentSha256, &m.PriorHead, &m.NextHead} {
		got, err := wire.ParseDigest(SealMarkerPath, lines[2+i])
		if err != nil {
			return nil, bad
		}
		*d = got
	}
	if string(EncodeSealMarker(*m)) != string(raw) {
		return nil, bad
	}
	return m, nil
}
