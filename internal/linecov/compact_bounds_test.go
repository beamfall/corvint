// SPDX-License-Identifier: Apache-2.0
package linecov

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// V1-5163, GLS-V0-002: a linecov-index/4 block section whose uvarint delta
// would wrap or overflow the decoded value is a damaged index, refused with
// the package's malformed-index error, never read as evidence.

// boundsIndex is the valid two-test, one-file index the hand encoder below
// reproduces byte for byte.
func boundsIndex() *Index {
	return &Index{
		Module: "example.com/m", Commit: "c0ffee", Runner: "go1.27.3 set",
		Tests: []Test{{Package: "p", Name: "TestA"}, {Package: "p", Name: "TestB"}},
		Files: []File{{Path: "p/a.go", OID: "0123", Blocks: []Block{
			{Start: 3, StartCol: 2, End: 5, EndCol: 1, Tests: []int32{0, 1}},
			{Start: 7, StartCol: 1, End: 7, EndCol: 9, Tests: []int32{1}},
		}}},
	}
}

// sectionBlock hand-encodes one block of a /4 section from raw deltas: the
// start delta, start column, end delta, end column, and the test and segment
// id lists, each a count followed by its deltas.
func sectionBlock(dst []byte, start, startCol, end, endCol uint64, tests, segs []uint64) []byte {
	for _, v := range []uint64{start, startCol, end, endCol, uint64(len(tests))} {
		dst = binary.AppendUvarint(dst, v)
	}
	for _, v := range tests {
		dst = binary.AppendUvarint(dst, v)
	}
	dst = binary.AppendUvarint(dst, uint64(len(segs)))
	for _, v := range segs {
		dst = binary.AppendUvarint(dst, v)
	}
	return dst
}

// validSection is boundsIndex's section, hand-encoded.
func validSection() []byte {
	sec := sectionBlock(nil, 3, 2, 2, 1, []uint64{0, 0}, nil)
	return sectionBlock(sec, 4, 1, 0, 9, []uint64{1}, nil)
}

// compactFile hand-encodes boundsIndex's header around sec, which holds
// blocks blocks, so a test can place any section bytes behind a header and
// section checksum that verify.
func compactFile(sec []byte, blocks int) []byte {
	var h []byte
	uv := func(v uint64) { h = binary.AppendUvarint(h, v) }
	str := func(s string) { uv(uint64(len(s))); h = append(h, s...) }
	ix := boundsIndex()
	str(ix.Module)
	str(ix.Commit)
	str(ix.Runner)
	uv(1)
	str("p")
	uv(uint64(len(ix.Tests)))
	for _, t := range ix.Tests {
		uv(0)
		str(t.Name)
		h = binary.AppendVarint(h, 0)
		str("")
		h = append(h, 0)
	}
	uv(0) // unordered
	uv(0) // in-order ran
	uv(0) // in-order skipped
	uv(1)
	str(ix.Files[0].Path)
	str(ix.Files[0].OID)
	uv(uint64(blocks))
	uv(0)
	uv(uint64(len(sec)))
	sum := sha256.Sum256(sec)
	h = append(h, sum[:]...)
	uv(0) // no extension
	out := []byte(magicV4)
	out = binary.LittleEndian.AppendUint64(out, uint64(len(h)))
	out = append(out, h...)
	hs := sha256.Sum256(h)
	out = append(out, hs[:]...)
	return append(out, sec...)
}

// The hand encoder writes what Encode writes, so a hostile section differs
// from a valid index in its section bytes alone.
func TestCompactHandEncoderMatchesEncode(t *testing.T) {
	var want bytes.Buffer
	if err := boundsIndex().Encode(&want); err != nil {
		t.Fatal(err)
	}
	got := compactFile(validSection(), 2)
	if !bytes.Equal(got, want.Bytes()) {
		t.Fatalf("hand encoding differs from Encode:\n got %x\nwant %x", got, want.Bytes())
	}
	ix, err := Decode(bytes.NewReader(got))
	if err != nil {
		t.Fatal(err)
	}
	var again bytes.Buffer
	if err := ix.Encode(&again); err != nil || !bytes.Equal(again.Bytes(), got) {
		t.Fatalf("round trip changed the bytes (%v)", err)
	}
}

type boundsCase struct {
	name   string
	sec    []byte
	blocks int
}

// checkRefused feeds each case to decodeSection, to a whole Decode and to a
// lazy Open, and wants the malformed-index error from the section decoder and
// a refusal from both readers.
func checkRefused(t *testing.T, cases []boundsCase) {
	t.Helper()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ix := boundsIndex()
			row := fileRow{length: int64(len(c.sec)), blocks: c.blocks, sum: sha256.Sum256(c.sec)}
			blocks, err := decodeSection(c.sec, row, len(ix.Tests), 0)
			if err == nil {
				t.Errorf("decodeSection accepted it: %+v", blocks)
			} else if !strings.HasPrefix(err.Error(), "linecov: malformed index: ") {
				t.Errorf("decodeSection error %q, want the malformed-index error", err)
			}
			data := compactFile(c.sec, c.blocks)
			if got, err := Decode(bytes.NewReader(data)); err == nil {
				t.Errorf("Decode accepted it: %+v", got.Files[0].Blocks)
			}
			p := filepath.Join(t.TempDir(), "x.idx")
			if err := os.WriteFile(p, data, 0o644); err != nil {
				t.Fatal(err)
			}
			lazy, err := Open(p)
			if err != nil {
				t.Fatal(err) // the header is valid; only the section is damaged
			}
			defer lazy.Close()
			if err := lazy.ensureFile(0); err == nil {
				t.Errorf("a lazy Open accepted the section: %+v", lazy.Files[0].Blocks)
			}
		})
	}
}

// Site 1, ids(): `last + 1 + int64(delta)`. Observed on base b55635262:
// decodeSection and a lazy Open accept every case, storing test id -1 for a
// delta of 2^64-1 (and a whole Decode then refuses it only through
// checkSegments) and test id 0 (int32 of -2^63) for the wrapped and
// overflowing deltas, which a whole Decode also accepts.
func TestCompactRefusesIDDeltaOverflow(t *testing.T) { checkRefused(t, idDeltaCases()) }

func idDeltaCases() []boundsCase {
	block := func(tests ...uint64) []byte { return sectionBlock(nil, 3, 2, 2, 1, tests, nil) }
	return []boundsCase{
		{"delta 2^64-1 gives -1", block(math.MaxUint64), 1},
		{"delta 2^63 wraps negative", block(1 << 63), 1},
		{"second delta wraps below the first", block(1, math.MaxUint64), 1},
		{"sum overflows int64", block(0, math.MaxInt64), 1},
		{"delta one past the limit", block(2), 1},
	}
}

// Site 2, the block start: `int64(prev) + int64(delta)`. Observed on base
// b55635262: every reader accepts a first block starting at line -1, and a
// start of 2^32+5 whose end delta wraps back reads as Start 5, End 5.
func TestCompactRefusesBlockStartOverflow(t *testing.T) { checkRefused(t, blockStartCases()) }

func blockStartCases() []boundsCase {
	return []boundsCase{
		{"delta 2^64-1 gives start -1", sectionBlock(nil, math.MaxUint64, 0, 0, 0, nil, nil), 1},
		{"delta 2^63 wraps negative", sectionBlock(nil, 1<<63, 0, 0, 0, nil, nil), 1},
		{"start above int32 truncated by a wrapped end", sectionBlock(nil, 1<<32+5, 0, math.MaxUint64-1<<32+1, 0, nil, nil), 1},
		{"second start overflows int32", sectionBlock(sectionBlock(nil, 3, 0, 0, 0, nil, nil), math.MaxInt32-2, 0, 0, 0, nil, nil), 2},
	}
}

// Site 3, the block end: `start + int64(delta)`. Observed on base b55635262:
// every reader accepts End 9 below Start 10 for a delta of 2^64-1, and reads
// End 10 (int32 of 10-2^63) for a delta of 2^63.
func TestCompactRefusesBlockEndOverflow(t *testing.T) { checkRefused(t, blockEndCases()) }

func blockEndCases() []boundsCase {
	return []boundsCase{
		{"delta 2^64-1 ends before the start", sectionBlock(nil, 10, 0, math.MaxUint64, 0, nil, nil), 1},
		{"delta 2^63 wraps negative", sectionBlock(nil, 10, 0, 1<<63, 0, nil, nil), 1},
		{"end above int32", sectionBlock(nil, 10, 0, math.MaxInt32-9, 0, nil, nil), 1},
	}
}

// FuzzCompactDecode feeds arbitrary block sections, under a checksum that
// verifies, to the section decoder with arbitrary test and segment limits,
// and to a whole Decode behind boundsIndex's header. Property: decoding
// fails, or every id is ascending and below its limit and every block has
// 0 <= Start <= End <= 2^31-1 and non-negative columns; it never panics.
func FuzzCompactDecode(f *testing.F) {
	f.Add(validSection(), uint32(2), uint32(2), uint32(0))
	f.Add(sectionBlock(nil, 1, 0, 0, 0, []uint64{0}, []uint64{0}), uint32(1), uint32(1), uint32(1))
	for _, cases := range [][]boundsCase{idDeltaCases(), blockStartCases(), blockEndCases()} {
		for _, c := range cases {
			f.Add(c.sec, uint32(c.blocks), uint32(2), uint32(0))
		}
	}
	f.Fuzz(func(t *testing.T, sec []byte, blocks, nTests, nSegs uint32) {
		// Limits up to 2^31: the most tests a header count admits.
		lim := func(v uint32) int { return int(v % (1<<31 + 1)) }
		tests, segs := lim(nTests), lim(nSegs)
		row := fileRow{length: int64(len(sec)), blocks: int(blocks), sum: sha256.Sum256(sec)}
		if got, err := decodeSection(sec, row, tests, segs); err == nil {
			checkDecodedBlocks(t, got, tests, segs)
		}
		if ix, err := Decode(bytes.NewReader(compactFile(sec, int(blocks)))); err == nil {
			checkDecodedBlocks(t, ix.Files[0].Blocks, len(ix.Tests), len(ix.Segments))
		}
	})
}

func checkDecodedBlocks(t *testing.T, blocks []Block, nTests, nSegs int) {
	t.Helper()
	inRange := func(what string, ids []int32, limit int) {
		for i, id := range ids {
			if id < 0 || int(id) >= limit || i > 0 && id <= ids[i-1] {
				t.Fatalf("%s ids %v outside [0, %d) or not ascending", what, ids, limit)
			}
		}
	}
	for _, b := range blocks {
		if b.Start < 0 || b.End < b.Start || b.StartCol < 0 || b.EndCol < 0 {
			t.Fatalf("decoded block %+v", b)
		}
		inRange("test", b.Tests, nTests)
		inRange("segment", b.Segs, nSegs)
	}
}
