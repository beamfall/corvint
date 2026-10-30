package main

import (
	"bytes"
	"testing"
	"unsafe"
)

// TestCEMV1690RetainedSpansShareOneBlob is the portable CEM-V1-690 case: 64
// overlapping near-full spans of one large blob, each read afresh as the
// stable session does, are retained as capacity-clipped windows into the
// first copy only, matching the native verifier's bound of one blob.
func TestCEMV1690RetainedSpansShareOneBlob(t *testing.T) {
	const size, records = 4 << 20, 64
	content := make([]byte, size)
	for i := range content {
		content[i] = byte('a' + i%23)
	}
	retained := map[string][]byte{}
	spans := make([][]byte, records)
	for i := range spans {
		read := append([]byte(nil), content...)
		spans[i] = retainedSpan(retained, "oid", read, uint64(i), uint64(size-records+i))
	}
	origin := uintptr(unsafe.Pointer(unsafe.SliceData(spans[0])))
	for i, got := range spans {
		if !bytes.Equal(got, content[i:size-records+i]) {
			t.Fatalf("span %d bytes differ from the blob", i)
		}
		if cap(got) != len(got) {
			t.Fatalf("span %d cap %d, want clipped to len %d", i, cap(got), len(got))
		}
		if offset := uintptr(unsafe.Pointer(unsafe.SliceData(got))) - origin; offset != uintptr(i) {
			t.Fatalf("span %d is not a window into the first retained copy (offset %d)", i, offset)
		}
	}
	if len(retained) != 1 || len(retained["oid"]) != size {
		t.Fatalf("retained %d blobs, want one of %d bytes", len(retained), size)
	}
}
