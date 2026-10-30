package snapshot_test

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// segmentOf builds a well-formed 512-receipt segment starting at first with
// two sorted request afterimages.
func segmentOf(t *testing.T, first uint64) *snapshot.Segment {
	t.Helper()
	s := &snapshot.Segment{First: first, Last: first + snapshot.SegmentReceipts - 1}
	for n := first; n <= s.Last; n++ {
		name, err := snapshot.ReceiptName(n)
		if err != nil {
			t.Fatal(err)
		}
		s.Receipts = append(s.Receipts, snapshot.SegmentFile{Path: "receipts/" + name, Raw: []byte(fmt.Sprintf("{\"seq\":%d}\n", n))})
	}
	for _, c := range []string{"ab", "cd"} {
		p := "requests/" + c + "/" + c + strings.Repeat("0", 62) + ".json"
		s.Requests = append(s.Requests, snapshot.SegmentFile{Path: p, Raw: []byte("{}\n")})
	}
	return s
}

// TestJSGV0006_SegmentCodecRoundTripsAndRefuses covers the segment, manifest
// and marker codecs: exact round trips, and refusal of a wrong range,
// trailing bytes, unsorted requests and another segment version.
func TestJSGV0006_SegmentCodecRoundTripsAndRefuses(t *testing.T) {
	s := segmentOf(t, 513)
	raw, err := snapshot.EncodeSegment(s)
	if err != nil {
		t.Fatal(err)
	}
	got, err := snapshot.DecodeSegment(raw, 513, 1024)
	if err != nil {
		t.Fatal(err)
	}
	again, err := snapshot.EncodeSegment(got)
	if err != nil || !bytes.Equal(again, raw) {
		t.Fatalf("segment round trip differs: %v", err)
	}
	if _, err := snapshot.DecodeSegment(raw, 1, 512); code(err) != wire.CodeJournalForked {
		t.Fatalf("wrong range: got %v, want JOURNAL_FORKED", err)
	}
	if _, err := snapshot.DecodeSegment(append(append([]byte(nil), raw...), 0), 513, 1024); code(err) != wire.CodeJournalForked {
		t.Fatalf("trailing bytes: got %v, want JOURNAL_FORKED", err)
	}
	if _, err := snapshot.DecodeSegment(raw[:len(raw)-1], 513, 1024); code(err) != wire.CodeJournalForked {
		t.Fatalf("truncated: got %v, want JOURNAL_FORKED", err)
	}
	other := append([]byte("taskman-segment/1\n"), raw[len("taskman-segment/0\n"):]...)
	if _, err := snapshot.DecodeSegment(other, 513, 1024); code(err) != wire.CodeUnsupportedVersion {
		t.Fatalf("another version: got %v, want UNSUPPORTED_VERSION", err)
	}
	unsorted := segmentOf(t, 513)
	unsorted.Requests[0], unsorted.Requests[1] = unsorted.Requests[1], unsorted.Requests[0]
	if _, err := snapshot.EncodeSegment(unsorted); code(err) != wire.CodeJournalForked {
		t.Fatalf("unsorted requests encoded: %v", err)
	}
	short := segmentOf(t, 1)
	short.Receipts = short.Receipts[:511]
	if _, err := snapshot.EncodeSegment(short); code(err) != wire.CodeJournalForked {
		t.Fatalf("short segment encoded: %v", err)
	}

	q, err := wire.ParseQueueID("", fixture.QueueID)
	if err != nil {
		t.Fatal(err)
	}
	m := &snapshot.SegmentManifest{QueueID: q, Through: 1024, Segments: []snapshot.SegmentEntry{
		{First: 1, Last: 512, Sha256: wire.Sum([]byte("a")), Bytes: 10, LastReceiptSha256: wire.Sum([]byte("b")), Requests: 2},
		{First: 513, Last: 1024, Sha256: wire.Sum(raw), Bytes: uint64(len(raw)), LastReceiptSha256: wire.Sum([]byte("c")), Requests: 2},
	}}
	mraw := wire.EncodeFile(m.Value())
	dm, err := snapshot.DecodeSegmentManifest(mraw, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(wire.EncodeFile(dm.Value()), mraw) {
		t.Fatal("manifest round trip differs")
	}
	if _, err := snapshot.DecodeSegmentManifest(mraw, 513); code(err) != wire.CodeJournalForked {
		t.Fatalf("manifest from another first: %v", err)
	}
	gap := *m
	gap.Segments = []snapshot.SegmentEntry{m.Segments[1]}
	if _, err := snapshot.DecodeSegmentManifest(wire.EncodeFile(gap.Value()), 1); code(err) != wire.CodeJournalForked {
		t.Fatalf("manifest with a gap: %v", err)
	}
	wrongS := *m
	wrongS.Through = 512
	if _, err := snapshot.DecodeSegmentManifest(wire.EncodeFile(wrongS.Value()), 1); code(err) != wire.CodeJournalForked {
		t.Fatalf("manifest S differs from its segments: %v", err)
	}

	mk := snapshot.SealMarker{First: 513, Last: 1024, SegmentSha256: wire.Sum(raw), PriorHead: wire.Sum([]byte("p")), NextHead: wire.Sum([]byte("n"))}
	dk, err := snapshot.DecodeSealMarker(snapshot.EncodeSealMarker(mk))
	if err != nil || *dk != mk {
		t.Fatalf("marker round trip: %+v %v", dk, err)
	}
	if _, err := snapshot.DecodeSealMarker(append(snapshot.EncodeSealMarker(mk), 'x')); code(err) != wire.CodeJournalForked {
		t.Fatalf("marker with trailing bytes: %v", err)
	}
	newer := bytes.Replace(snapshot.EncodeSealMarker(mk), []byte(snapshot.ProfileSealMarker), []byte("taskman-seal-marker/1"), 1)
	if _, err := snapshot.DecodeSealMarker(newer); code(err) != wire.CodeUnsupportedVersion {
		t.Fatalf("another marker version: %v, want UNSUPPORTED_VERSION", err)
	}
}

// TestJSGV0006_SegmentedHeadProfile covers taskman-journal-head/1: an exact
// round trip, the whole-range checks on its S, and a /0-only decoder's
// refusal of it as another version.
func TestJSGV0006_SegmentedHeadProfile(t *testing.T) {
	d := wire.Sum([]byte("x"))
	h0, err := snapshot.DecodeHead(wire.EncodeFile(fixture.HeadValue("/tmp/x", 1100, d, 0, d)))
	if err != nil {
		t.Fatal(err)
	}
	if h0.Segments != nil {
		t.Fatal("a /0 head decoded with segments")
	}
	h1 := *h0
	h1.Segments = &snapshot.SegmentBinding{Through: wire.SizeOf(1024), ManifestSha256: d}
	raw := wire.EncodeFile(h1.Value())
	if !bytes.Contains(raw, []byte(snapshot.ProfileHeadSegmented)) {
		t.Fatalf("segmented head renders another profile: %s", raw)
	}
	got, err := snapshot.DecodeHead(raw)
	if err != nil {
		t.Fatal(err)
	}
	if got.Segments == nil || *got.Segments != *h1.Segments || !bytes.Equal(wire.EncodeFile(got.Value()), raw) {
		t.Fatal("segmented head round trip differs")
	}
	for _, through := range []uint64{0, 500, 1100, 1536} {
		bad := h1
		bad.Segments = &snapshot.SegmentBinding{Through: wire.SizeOf(through), ManifestSha256: d}
		if _, err := snapshot.DecodeHead(wire.EncodeFile(bad.Value())); code(err) != wire.CodeMalformed {
			t.Errorf("segmentsThrough %d: got %v, want MALFORMED", through, err)
		}
	}
	v, err := wire.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if err := wire.ProfileVersion("/profile", v, snapshot.ProfileHead); code(err) != wire.CodeUnsupportedVersion {
		t.Fatalf("a /0-only reader accepted a /1 head: %v", err)
	}
}
