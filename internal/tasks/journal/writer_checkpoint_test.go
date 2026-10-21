package journal

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/archive"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// sealWriter appends the trailer a valid writer checkpoint carries, so a
// case reaches the field check it names instead of the digest check.
func sealWriter(body []byte) []byte {
	sum := sha256.Sum256(body)
	return append(append([]byte(nil), body...), sum[:]...)
}

// encodeWriter is a writer checkpoint's base followed by its delta, so two
// checkpoints compare as the two files a writer retains.
func encodeWriter(t testing.TB, w *WriterCheckpoint) []byte {
	t.Helper()
	base, delta, err := w.Encode()
	if err != nil {
		t.Fatal(err)
	}
	return append(append([]byte(nil), base...), delta...)
}

func requestPathOf(d []byte) string {
	h := hex.EncodeToString(d)
	return "requests/" + h[:2] + "/" + h + ".json"
}

// TestCALV0115_WriterCheckpointCodecIsClosed covers the closed, bounded
// writer checkpoint codec (CAL-V0-115, proposed; JSG-V0-004): the delta and
// the base it binds round-trip exactly, a base-free checkpoint renders a base
// of every entry, and every truncation, flipped byte, foreign profile,
// inconsistent aggregate, unordered or repeated request digest, note that is
// not one strictly ordered live ticket entry, file without the invalidation
// digest, absent or other base, and malformed base is refused, so the writer
// falls back to the complete audit. A base entry whose path or size fails its
// bound decodes, and is refused where a walk first uses it.
func TestCALV0115_WriterCheckpointCodecIsClosed(t *testing.T) {
	repo, r := setup(t)
	seqA := appendReceipt(t, repo, "MUTATION", map[string][]byte{ticketPath("A"): fixture.Ticket("A").Encode()}, "", true, true, false)
	seqB := appendReceipt(t, repo, "MUTATION", map[string][]byte{ticketPath("B"): fixture.Ticket("B").Encode()}, "", true, true, false)
	full, observed, err := r.AuditForWriteObserved()
	if err != nil {
		t.Fatal(err)
	}
	derived, err := full.WriterCheckpoint(observed.Files)
	if err != nil {
		t.Fatal(err)
	}
	if !derived.FreshBase() || derived.Intent.Tickets != 2 || derived.Intent.Cost.PayloadBytes != uint64(len(fixture.Ticket("A").Encode())+len(fixture.Ticket("B").Encode())) {
		t.Fatalf("derived checkpoint %+v", derived.Intent)
	}
	// A base-free checkpoint renders a base of every entry and an empty
	// delta; decoding them and merging the base back is exact.
	base0, delta0, err := derived.Encode()
	if err != nil {
		t.Fatal(err)
	}
	got0, err := DecodeWriterCheckpoint(delta0, base0)
	if err != nil {
		t.Fatal(err)
	}
	if b, d, err := got0.Encode(); err != nil || !bytes.Equal(b, base0) || !bytes.Equal(d, delta0) || len(got0.Entries) != 0 || got0.count() != len(derived.Entries) {
		t.Fatalf("base-free round trip: %v", err)
	}
	if f, err := got0.full(); err != nil || !reflect.DeepEqual(f.Entries, derived.Entries) || !reflect.DeepEqual(f.sizes, derived.sizes) || !bytes.Equal(f.requests, derived.requests) || len(f.notes) != len(derived.notes) {
		t.Fatalf("merged base differs from the derived checkpoint: %v", err)
	}

	// A base at seqA holding ticket A with its note and one request, and a
	// delta at seqB holding ticket B with its note, two requests, and the
	// deletion of one base entry.
	refA := &ticket.OperatorNoteReference{Revision: "1", Head: wire.Sum([]byte("note a"))}
	refB := &ticket.OperatorNoteReference{Revision: "1", Head: wire.Sum([]byte("note b"))}
	pair := func(path, after string) []byte {
		p, a := sha256.Sum256([]byte(path)), sha256.Sum256([]byte(after))
		return append(append([]byte(nil), p[:]...), a[:]...)
	}
	baseReq := pair("base", "request base")
	digests := [][]byte{pair("a", "request a"), pair("b", "request b")}
	sort.Slice(digests, func(i, j int) bool { return bytes.Compare(digests[i], digests[j]) < 0 })
	var baseEntries, deltaEntries []CheckpointEntry
	var baseSizes, deltaSizes []uint64
	other, kept := "", ""
	for i, e := range derived.Entries {
		if e.Seq.Uint64() > seqA {
			deltaEntries, deltaSizes = append(deltaEntries, e), append(deltaSizes, derived.sizes[i])
			continue
		}
		baseEntries, baseSizes = append(baseEntries, e), append(baseSizes, derived.sizes[i])
		if e.Sha256 != nil && !intentRecord(e.Path) {
			if other == "" {
				other = e.Path
			} else if kept == "" {
				kept = e.Path
			}
		}
	}
	if len(deltaEntries) != 1 || deltaEntries[0].Path != ticketPath("B") || other == "" || kept == "" {
		t.Fatalf("checkpoint entries %+v", derived.Entries)
	}
	deltaEntries = append(deltaEntries, CheckpointEntry{Path: other, Seq: wire.SizeOf(seqB)})
	deltaSizes = append(deltaSizes, 0)
	if other < ticketPath("B") {
		deltaEntries[0], deltaEntries[1] = deltaEntries[1], deltaEntries[0]
		deltaSizes[0], deltaSizes[1] = deltaSizes[1], deltaSizes[0]
	}
	baseIndex := func(p string) int {
		for i, e := range baseEntries {
			if e.Path == p {
				return i
			}
		}
		t.Fatalf("%s not in the base", p)
		return -1
	}
	deltaIndex := func(p string) int {
		for i, e := range deltaEntries {
			if e.Path == p {
				return i
			}
		}
		t.Fatalf("%s not in the delta", p)
		return -1
	}
	baseW := &WriterCheckpoint{Checkpoint: derived.Checkpoint, sizes: baseSizes, requests: baseReq}
	baseW.Seq, baseW.Entries = wire.SizeOf(seqA), baseEntries
	baseW.notes = []writerNote{{entry: uint32(baseIndex(ticketPath("A"))), digest: noteDigest(refA)}}
	// JSG-V0-009: the base carries its header and aggregates at seqA, which
	// must agree with its own table.
	baseW.Cost = archive.FileSetCost{Files: int(seqA) + len(baseReq)/writerRequestBytes}
	for i, e := range baseEntries {
		if e.Sha256 != nil && intentRecord(e.Path) {
			if strings.HasPrefix(e.Path, "intent/tickets/") {
				baseW.Intent.Tickets++
			} else {
				baseW.Intent.Releases++
			}
			baseW.Intent.Cost.PayloadBytes += baseSizes[i]
		}
	}
	baseW.Intent.Cost.Files = baseW.Intent.Tickets + baseW.Intent.Releases
	baseRaw, _, err := baseW.Encode()
	if err != nil {
		t.Fatal(err)
	}
	bt, err := decodeBase(baseRaw)
	if err != nil {
		t.Fatal(err)
	}
	wc := &WriterCheckpoint{Checkpoint: derived.Checkpoint, FullSeq: 2, ReceiptBytes: 900, Cost: archive.FileSetCost{Files: int(seqB) + 3, PayloadBytes: 1200, EntryBytes: 300, TarBytes: 4096}}
	wc.Entries, wc.sizes, wc.Intent = deltaEntries, deltaSizes, derived.Intent
	wc.requests = append(append([]byte(nil), digests[0]...), digests[1]...)
	wc.notes = []writerNote{{entry: uint32(deltaIndex(ticketPath("B"))), digest: noteDigest(refB)}}
	wc.Invalidation = sha256.Sum256([]byte("token\n"))
	wc.base, wc.total = bt, bt.n+1
	encode := func(w *WriterCheckpoint) []byte {
		t.Helper()
		b, d, err := w.Encode()
		if err != nil || !bytes.Equal(b, baseRaw) {
			t.Fatalf("encode on the base: %v", err)
		}
		return d
	}
	raw := encode(wc)
	got, err := DecodeWriterCheckpoint(raw, baseRaw)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(encode(got), raw) || got.Requests() != 3 || got.FullSeq != 2 || got.Cost != wc.Cost || got.ReceiptBytes != 900 || got.Invalidation != wc.Invalidation || got.Intent != wc.Intent || got.count() != bt.n+1 {
		t.Fatalf("writer checkpoint codec is not a round trip: %+v", got)
	}
	for _, d := range append(digests, baseReq) {
		p := requestPathOf(d[:sha256.Size])
		// CAL-V0-190 (proposed): each request carries its afterimage digest.
		if after, ok := got.RequestAfterimage(p); !ok || after != wire.Digest(hex.EncodeToString(d[sha256.Size:])) {
			t.Fatalf("%s afterimage %q %v", p, after, ok)
		}
	}
	changed := *refA
	changed.Revision = "2"
	if !got.sameNote(ticketPath("A"), refA) || got.sameNote(ticketPath("A"), nil) || got.sameNote(ticketPath("A"), &changed) || !got.sameNote(ticketPath("B"), refB) || got.sameNote(ticketPath("B"), refA) || !got.sameNote(ticketPath("C"), nil) {
		t.Fatalf("note references %+v", got.notes)
	}
	if l, ok, err := got.lookup(other); err != nil || !ok || l.digest != nil || got.liveIntent(other) || !got.liveIntent(ticketPath("A")) || !got.liveIntent(ticketPath("B")) {
		t.Fatalf("delta deletion of %s: %+v %v %v", other, l, ok, err)
	}
	if n, ok := got.entrySize(ticketPath("A")); !ok || n != uint64(len(fixture.Ticket("A").Encode())) {
		t.Fatalf("base ticket size %d %v", n, ok)
	}
	c := sha256.Sum256([]byte("c"))
	for _, p := range []string{requestPathOf(c[:]), strings.ToUpper(requestPathOf(digests[0][:sha256.Size])), "requests/" + hex.EncodeToString(digests[0][:sha256.Size]) + ".json", "intent/queue.json"} {
		if got.HasRequest(p) {
			t.Errorf("%s: reported as retained", p)
		}
	}
	if rel, err := filepath.Rel(repo.StateDir, WriterCheckpointPath(repo.StateDir)); err != nil || !strings.HasPrefix(rel, "..") {
		t.Fatalf("writer checkpoint path is inside the state directory: %s", rel)
	}
	// Merging the base back gives the base-free checkpoint of the same
	// entries, requests and notes.
	f, err := got.full()
	if err != nil || f.count() != bt.n+1 || f.Requests() != 3 || !f.sameNote(ticketPath("A"), refA) || !f.sameNote(ticketPath("B"), refB) || f.search(other) < 0 || f.Entries[f.search(other)].Sha256 != nil {
		t.Fatalf("merged checkpoint: %v", err)
	}

	for n := 0; n < len(raw); n++ {
		if _, err := DecodeWriterCheckpoint(raw[:n], baseRaw); err == nil {
			t.Fatalf("truncation to %d bytes accepted", n)
		}
	}
	for i := range raw {
		flipped := append([]byte(nil), raw...)
		flipped[i] ^= 0x01
		if _, err := DecodeWriterCheckpoint(flipped, baseRaw); err == nil {
			t.Fatalf("flipped byte %d accepted", i)
		}
	}

	body := raw[:len(raw)-sha256.Size]
	lenAt := len(writerMagic) + sha256.Size
	cpLen := int(binary.BigEndian.Uint64(body[lenAt:]))
	aggregates := lenAt + 8 + cpLen
	field := func(i int, v uint64) []byte {
		out := append([]byte(nil), body...)
		binary.BigEndian.PutUint64(out[aggregates+8*i:], v)
		return sealWriter(out)
	}
	intentAt := aggregates + 7*8
	sizesAt := intentAt + writerIntentFields*8
	requestCountAt := sizesAt + 8*len(deltaEntries)
	requestsAt := requestCountAt + 8
	sized := func(at int, v uint64) []byte {
		out := append([]byte(nil), body...)
		binary.BigEndian.PutUint64(out[at:], v)
		return sealWriter(out)
	}
	swapped := append([]byte(nil), body...)
	copy(swapped[requestsAt:], digests[1])
	copy(swapped[requestsAt+writerRequestBytes:], digests[0])
	repeated := append([]byte(nil), body...)
	copy(repeated[requestsAt+writerRequestBytes:], digests[0])
	// The same path with another afterimage is still a repeated request.
	repeatedPath := append([]byte(nil), body...)
	copy(repeatedPath[requestsAt+writerRequestBytes:], digests[0][:sha256.Size])
	foreign := append([]byte(nil), body...)
	copy(foreign, strings.Replace(writerMagic, "/3", "/4", 1))
	previous := append([]byte(nil), body...)
	copy(previous, strings.Replace(writerMagic, "/3", "/2", 1))
	longCP := append([]byte(nil), body...)
	binary.BigEndian.PutUint64(longCP[lenAt:], uint64(len(body)))
	notesAt := requestsAt + 2*writerRequestBytes
	note := func(edit func(out []byte) []byte) []byte {
		return sealWriter(edit(append([]byte(nil), body...)))
	}
	entry := func(i uint32) []byte {
		return note(func(out []byte) []byte { binary.BigEndian.PutUint32(out[notesAt+8:], i); return out })
	}
	// A request the base holds, repeated in the delta.
	repeatsBase := *wc
	repeatsBase.requests = mergeRequests(baseReq, digests[0])
	// A delta entry at or before the base's sequence.
	early := *wc
	early.Entries = append([]CheckpointEntry(nil), wc.Entries...)
	early.Entries[deltaIndex(ticketPath("B"))].Seq = wire.SizeOf(seqA)
	b := ticketPath("B")
	for name, bad := range map[string][]byte{
		"foreign profile":         sealWriter(foreign),
		"previous profile":        sealWriter(previous),
		"embedded length":         sealWriter(longCP),
		"truncated aggregates":    sealWriter(body[:aggregates+8]),
		"request count":           field(6, 4),
		"file count":              field(2, seqB+4),
		"full seq zero":           field(0, 0),
		"full seq after seq":      field(0, seqB+1),
		"receipt bytes > payload": field(1, 1201),
		"delta request count":     sized(requestCountAt, 1),
		"unordered requests":      sealWriter(swapped),
		"repeated request":        sealWriter(repeated),
		"repeated request path":   sealWriter(repeatedPath),
		"delta repeats the base":  encode(&repeatsBase),
		"delta entry not after":   encode(&early),
		"trailing digest bytes":   sealWriter(append(append([]byte(nil), body...), 1)),
		"truncated notes":         sealWriter(body[:notesAt+4]),
		"note count":              note(func(out []byte) []byte { binary.BigEndian.PutUint64(out[notesAt:], 2); return out }),
		"note entry out of range": entry(uint32(len(deltaEntries))),
		"note entry not a ticket": entry(uint32(deltaIndex(other))),
		"repeated note entry": note(func(out []byte) []byte {
			binary.BigEndian.PutUint64(out[notesAt:], 2)
			end := notesAt + 8 + writerNoteBytes
			return append(append(append([]byte(nil), out[:end]...), out[notesAt+8:end]...), out[end:]...)
		}),
		"layout without invalidation": sealWriter(body[:len(body)-sha256.Size]),
		"intent ticket count":         sized(intentAt, 3),
		"intent release count":        sized(intentAt+8, 1),
		"intent payload":              sized(intentAt+16, wc.Intent.Cost.PayloadBytes+1),
		"intent payload over bound":   sized(intentAt+16, uint64(wire.MaxIntentTreeBytes)+1),
		"ticket size":                 sized(sizesAt+8*deltaIndex(b), uint64(len(fixture.Ticket("B").Encode()))+1),
		"ticket size over bound":      sized(sizesAt+8*deltaIndex(b), uint64(wire.MaxIntentTreeBytes)+1),
		"deletion with a size":        sized(sizesAt+8*deltaIndex(other), 1),
		"truncated sizes":             sealWriter(body[:sizesAt+8]),
		"over the bound":              make([]byte, MaxWriterCheckpointBytes+1),
	} {
		if _, err := DecodeWriterCheckpoint(bad, baseRaw); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}

	// The base: absent, another one, or one the delta binds that is
	// malformed. rebound edits a copy of the base and binds the delta to it.
	rebound := func(edit func(out []byte) []byte) [2][]byte {
		out := edit(append([]byte(nil), baseRaw...))
		delta := append([]byte(nil), body...)
		sum := sha256.Sum256(out)
		copy(delta[len(writerMagic):], sum[:])
		return [2][]byte{sealWriter(delta), out}
	}
	rec := func(i int) int { return baseHeaderBytes + i*baseRecordBytes }
	header := func(k int, v uint64) [2][]byte {
		return rebound(func(out []byte) []byte { binary.BigEndian.PutUint64(out[len(baseMagic)+8*k:], v); return out })
	}
	record := func(i, at int, v byte) [2][]byte {
		return rebound(func(out []byte) []byte { out[rec(i)+at] = v; return out })
	}
	recordU64 := func(i, at int, v uint64) [2][]byte {
		return rebound(func(out []byte) []byte { binary.BigEndian.PutUint64(out[rec(i)+at:], v); return out })
	}
	io, ia := baseIndex(other), baseIndex(ticketPath("A"))
	withEntry := func(p string, live bool) [2][]byte {
		w := *baseW
		w.Entries = append(append([]CheckpointEntry(nil), baseW.Entries...), CheckpointEntry{Path: p, Seq: wire.SizeOf(1)})
		w.sizes = append(append([]uint64(nil), baseW.sizes...), 0)
		if live {
			w.Entries[len(w.Entries)-1].Sha256 = baseW.Entries[io].Sha256
		}
		sort.SliceStable(w.Entries, func(i, j int) bool { return w.Entries[i].Path < w.Entries[j].Path })
		w.notes = []writerNote{{entry: uint32(w.search(ticketPath("A"))), digest: noteDigest(refA)}}
		out, _, err := w.Encode()
		if err != nil {
			t.Fatal(err)
		}
		return rebound(func([]byte) []byte { return out })
	}
	otherBase := append([]byte(nil), baseRaw...)
	otherBase[len(otherBase)-1] ^= 1
	// JSG-V0-009: the base's checkpoint header, aggregates and segment
	// digests follow its notes.
	notesEnd := rec(bt.n) + len(bt.paths) + len(bt.reqs) + len(bt.notes)
	metaCP := int(binary.BigEndian.Uint64(baseRaw[notesEnd:]))
	aggAt := notesEnd + 8 + metaCP
	meta := func(i int, v uint64) [2][]byte {
		return rebound(func(out []byte) []byte { binary.BigEndian.PutUint64(out[aggAt+8*i:], v); return out })
	}
	reheader := func(edit func(w *WriterCheckpoint)) [2][]byte {
		w := *baseW
		edit(&w)
		out, _, err := w.Encode()
		if err != nil {
			t.Fatal(err)
		}
		return rebound(func([]byte) []byte { return out })
	}
	for name, c := range map[string][2][]byte{
		"base absent":    {raw, nil},
		"another base":   {raw, otherBase},
		"base version":   rebound(func(out []byte) []byte { copy(out, strings.Replace(baseMagic, "/1", "/2", 1)); return out }),
		"base version 0": rebound(func(out []byte) []byte { copy(out, strings.Replace(baseMagic, "/1", "/0", 1)); return out }),
		"base meta header length": rebound(func(out []byte) []byte {
			binary.BigEndian.PutUint64(out[notesEnd:], uint64(metaCP+1))
			return out
		}),
		"base meta truncated":        rebound(func(out []byte) []byte { return out[:len(out)-1] }),
		"base meta file count":       meta(1, uint64(baseW.Cost.Files+1)),
		"base meta intent tickets":   meta(5, uint64(baseW.Intent.Tickets+1)),
		"base meta intent payload":   meta(7, baseW.Intent.Cost.PayloadBytes+1),
		"base meta segments":         meta(baseAggregates, 1),
		"base exceeds the delta":     meta(0, 901),
		"base header at another seq": reheader(func(w *WriterCheckpoint) { w.Seq = wire.SizeOf(seqA - 1) }),
		"base worktree differs":      reheader(func(w *WriterCheckpoint) { w.PrimaryWorktree += "x" }),
		"base queue differs":         reheader(func(w *WriterCheckpoint) { w.InitSha256 = wire.Digest(strings.Repeat("0", 64)) }),
		"base profile":               rebound(func(out []byte) []byte { out[0] ^= 0x20; return out }),
		"base truncated":             rebound(func(out []byte) []byte { return out[:baseHeaderBytes-1] }),
		"base length":                rebound(func(out []byte) []byte { return append(out, 0) }),
		"base entry count":           header(1, uint64(bt.n+1)),
		"base request count":         header(2, 0),
		"base note count":            header(3, 2),
		"base path bytes":            header(4, uint64(len(bt.paths)-1)),
		"base seq zero":              header(0, 0),
		"base past the delta":        header(0, seqB+1),
		"base path gap":              rebound(func(out []byte) []byte { binary.BigEndian.PutUint32(out[rec(1):], 1); return out }),
		"base unordered paths": rebound(func(out []byte) []byte {
			out[rec(bt.n)+int(binary.BigEndian.Uint32(out[rec(bt.n-1):]))] = 'A'
			return out
		}),
		"base flags":                record(io, 6, 2),
		"base pad":                  record(io, 7, 1),
		"base deletion with a size": record(io, 6, 0),
		"base entry after its seq":  recordU64(io, 8, seqA+1),
		"base entry seq zero":       recordU64(io, 8, 0),
		"base ticket size":          recordU64(ia, 16, uint64(len(fixture.Ticket("A").Encode()))+1),
		"base request path":         withEntry(requestPathOf(c[:]), true),
		"base note on a non-ticket": rebound(func(out []byte) []byte {
			binary.BigEndian.PutUint32(out[notesEnd-writerNoteBytes:], uint32(io))
			return out
		}),
		"base note out of range": rebound(func(out []byte) []byte {
			binary.BigEndian.PutUint32(out[notesEnd-writerNoteBytes:], uint32(bt.n))
			return out
		}),
	} {
		if _, err := DecodeWriterCheckpoint(c[0], c[1]); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}

	// A base entry whose path fails PostBound, or whose size exceeds its
	// bound, decodes; the walk that first uses it is refused.
	keptBound, err := snapshot.PostBound(kept)
	if err != nil {
		t.Fatal(err)
	}
	for name, c := range map[string]struct {
		pair [2][]byte
		path string
	}{
		"path outside the archive namespace": {withEntry("zz-unknown", false), "zz-unknown"},
		"size over its bound":                {recordU64(baseIndex(kept), 16, uint64(keptBound)+1), kept},
	} {
		lazy, err := DecodeWriterCheckpoint(c.pair[0], c.pair[1])
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if _, _, err := lazy.lookup(c.path); err == nil {
			t.Errorf("%s: lookup accepted", name)
		}
		if err := lazy.each(nil, func(string, latest) error { return nil }); err == nil {
			t.Errorf("%s: walk accepted", name)
		}
		if _, err := lazy.full(); err == nil {
			t.Errorf("%s: merge accepted", name)
		}
	}
}

// TestJSGV0004_RetiredPrefixAnchorIsFrozen covers the anchor a seal carries
// (JRT-V0-004): it keeps the taskman-writer-checkpoint/1 layout whatever the
// writer checkpoint's version, and a checkpoint on a base renders the same
// anchor as its merged form.
func TestJSGV0004_RetiredPrefixAnchorIsFrozen(t *testing.T) {
	repo, r := setup(t)
	appendReceipt(t, repo, "MUTATION", map[string][]byte{ticketPath("A"): fixture.Ticket("A").Encode()}, "", true, true, false)
	full, observed, err := r.AuditForWriteObserved()
	if err != nil {
		t.Fatal(err)
	}
	derived, err := full.WriterCheckpoint(observed.Files)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := encodeAnchor(derived)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(raw, []byte("taskman-writer-checkpoint/1\n")) {
		t.Fatalf("anchor profile %q", raw[:32])
	}
	got, err := decodeAnchor(raw)
	if err != nil {
		t.Fatal(err)
	}
	if again, err := encodeAnchor(got); err != nil || !bytes.Equal(again, raw) || got.sizes != nil || !reflect.DeepEqual(got.Entries, derived.Entries) {
		t.Fatalf("anchor round trip: %v", err)
	}
	base, delta, err := derived.Encode()
	if err != nil {
		t.Fatal(err)
	}
	onBase, err := DecodeWriterCheckpoint(delta, base)
	if err != nil {
		t.Fatal(err)
	}
	if again, err := encodeAnchor(onBase); err != nil || !bytes.Equal(again, raw) {
		t.Fatalf("anchor of a checkpoint on a base differs: %v", err)
	}
	for n := 0; n < len(raw); n++ {
		if _, err := decodeAnchor(raw[:n]); err == nil {
			t.Fatalf("anchor truncation to %d bytes accepted", n)
		}
	}
	current := append([]byte(nil), raw...)
	copy(current, writerMagic)
	if _, err := decodeAnchor(sealWriter(current[:len(current)-sha256.Size])); err == nil {
		t.Fatal("anchor in the current writer checkpoint profile accepted")
	}
}

// TestCALV0189_WriterAuditReadsIntentOnce covers CAL-V0-189 (proposed): a
// writer audit reads each intent file's bytes once, in its first capture.
// The walk takes selected intent records from those bytes, and the second
// capture reuses them for a file that is still the same file with the same
// size and modification time; the bytes it returns are the bytes on disk.
func TestCALV0189_WriterAuditReadsIntentOnce(t *testing.T) {
	repo, r := setup(t)
	for _, id := range []string{"A", "B"} {
		appendReceipt(t, repo, "MUTATION", map[string][]byte{ticketPath(id): fixture.Ticket(id).Encode()}, "", true, true, false)
	}
	full, observed, err := r.AuditForWriteObserved()
	if err != nil || full.Mode != ModeFull {
		t.Fatalf("complete audit: %v", err)
	}
	wc, err := full.WriterCheckpoint(observed.Files)
	if err != nil || wc == nil {
		t.Fatalf("writer checkpoint: %v", err)
	}
	// One tail receipt, so the walk also projects a ticket posted after the
	// checkpoint.
	appendReceipt(t, repo, "MUTATION", map[string][]byte{ticketPath("C"): fixture.Ticket("C").Encode()}, "", true, true, false)
	reads := map[string]int{}
	old := afterNativeRead
	t.Cleanup(func() { afterNativeRead = old })
	afterNativeRead = func(p string) { reads[p]++ }
	for _, forMutation := range []bool{false, true} {
		clear(reads)
		res, _, err := r.AuditForWriter(wc, "", forMutation)
		if err != nil || res.Mode != ModeWriter {
			t.Fatalf("forMutation=%v: writer audit: %v", forMutation, err)
		}
		intents := 0
		for p, n := range reads {
			if strings.HasPrefix(p, "intent/") {
				intents++
				if n != 1 {
					t.Fatalf("forMutation=%v: %s read %d times", forMutation, p, n)
				}
			}
		}
		if intents < 4 {
			t.Fatalf("forMutation=%v: %d intent files read, want queue, policy and three tickets", forMutation, intents)
		}
		tk, ok := res.Records[ticketPath("C")]
		if !ok || tk.Raw == nil || !bytes.Equal(tk.Raw, fixture.Ticket("C").Encode()) || tk.Sha256 == nil || *tk.Sha256 != wire.Sum(tk.Raw) {
			t.Fatalf("forMutation=%v: selected ticket record %+v", forMutation, tk)
		}
	}
}

// TestCALV0224_WriterAuditWorkIsHistoryIndependent covers CAL-V0-224 as
// JSG-V0-007 amends it: at two history sizes, one settled writer audit reads
// only the checkpoint's receipt and the tail and lists receipts/ exactly in
// each of its two captures, whatever the directory's stamps; it never reads
// a prefix receipt. It counts work and never asserts a duration. A
// temporary receipt name that appears during the walk is in the second
// capture's listing, so the attempt moves and the fast route declines.
func TestCALV0224_WriterAuditWorkIsHistoryIndependent(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("a listing is never reused without a change time (CAL-V0-224)")
	}
	const tail = 3
	for _, history := range []int{4, 48} {
		t.Run(fmt.Sprint(history), func(t *testing.T) {
			repo, r := setup(t)
			receipts := filepath.Join(repo.StateDir, "receipts")
			past, future := time.Now().Add(-time.Hour), time.Now().Add(time.Hour)
			for i := 0; i < history; i++ {
				id := fmt.Sprintf("H%d", i)
				appendReceipt(t, repo, "MUTATION", map[string][]byte{ticketPath(id): fixture.Ticket(id).Encode()}, "", true, true, false)
			}
			full, observed, err := r.AuditForWriteObserved()
			if err != nil || full.Mode != ModeFull {
				t.Fatalf("complete audit: %v", err)
			}
			wc, err := full.WriterCheckpoint(observed.Files)
			if err != nil || wc == nil {
				t.Fatalf("writer checkpoint: %v", err)
			}
			for i := 0; i < tail; i++ {
				id := fmt.Sprintf("T%d", i)
				appendReceipt(t, repo, "MUTATION", map[string][]byte{ticketPath(id): fixture.Ticket(id).Encode()}, "", true, true, false)
			}
			listings, reads := 0, 0
			oldNames, oldRead := afterReadNames, afterNativeRead
			t.Cleanup(func() { afterReadNames, afterNativeRead = oldNames, oldRead })
			afterReadNames = func(p string) {
				if p == "receipts" {
					listings++
				}
			}
			afterNativeRead = func(p string) {
				if strings.HasPrefix(p, "receipts/") {
					reads++
				}
			}
			for _, c := range []struct {
				name     string
				stamp    time.Time
				await    bool
				listings int
			}{{"fresh", future, false, 2}, {"restored", past, false, 2}} {
				if err := os.Chtimes(receipts, c.stamp, c.stamp); err != nil {
					t.Fatal(err)
				}
				for _, forMutation := range []bool{false, true} {
					listings, reads = 0, 0
					res, _, err := r.AuditForWriter(wc, "", forMutation)
					if err != nil || res.Mode != ModeWriter {
						t.Fatalf("%s forMutation=%v: writer audit: %v", c.name, forMutation, err)
					}
					if listings != c.listings || reads != tail+1 {
						t.Fatalf("%s forMutation=%v: %d receipts/ listings and %d receipt reads, want %d and %d", c.name, forMutation, listings, reads, c.listings, tail+1)
					}
				}
			}
			// A temporary name written after the first capture is in the
			// second capture's listing, so the attempt moves; the retried
			// attempt lists the directory twice and declines on it.
			stray, _ := snapshot.ReceiptName(uint64(history + tail + 1))
			r.afterCapture = func() {
				if listings == 1 {
					fixture.Write(t, filepath.Join(receipts, stray+".tmp-1-x"), []byte("x"))
				}
			}
			listings = 0
			if _, _, err := r.AuditForWriter(wc, "", true); err == nil || !strings.Contains(err.Error(), "not plainly settled") {
				t.Fatalf("writer audit with a receipt name that appeared during its walk: %v, want a decline", err)
			}
			if listings != 4 {
				t.Fatalf("%d receipts/ listings after a name appeared during the walk, want 4", listings)
			}
		})
	}
}

// TestCALV0190_WriterReplayKeyAndCloseFailure covers CAL-V0-190 (proposed)
// in the journal: a request posted before the checkpoint is found under the
// record keyed by the SHA-256 of its request ID (the name of its request
// path) and replays its original entry; a failed native close while reading
// it is returned as cleanup, with no replay, for each kind of handle.
func TestCALV0190_WriterReplayKeyAndCloseFailure(t *testing.T) {
	repo, r := setup(t)
	rp, err := snapshot.RequestPath("R")
	if err != nil {
		t.Fatal(err)
	}
	raw := requestBytes("R", 2, wire.Sum([]byte("R")), false)
	appendReceipt(t, repo, "MUTATION", map[string][]byte{rp: raw}, "R", true, true, false)
	full, observed, err := r.AuditForWriteObserved()
	if err != nil || full.Mode != ModeFull {
		t.Fatalf("complete audit: %v", err)
	}
	wc, err := full.WriterCheckpoint(observed.Files)
	if err != nil || wc == nil {
		t.Fatalf("writer checkpoint: %v", err)
	}
	key := sha256.Sum256([]byte("R"))
	if got, ok := wc.RequestAfterimage(requestPathOf(key[:])); !ok || got != wire.Sum(raw) {
		t.Fatalf("record keyed by the request ID digest: %v %v", got, ok)
	}
	appendReceipt(t, repo, "MUTATION", map[string][]byte{ticketPath("A"): fixture.Ticket("A").Encode()}, "", true, true, false)
	res, _, err := r.AuditForWriter(wc, "R", true)
	if err != nil || res.Mode != ModeWriter {
		t.Fatalf("writer audit: %v", err)
	}
	want, err := snapshot.DecodeRequest(raw)
	if err != nil {
		t.Fatal(err)
	}
	found, entry, _, cleanup, err := r.WriterReplay(res, "R")
	if err != nil || cleanup != nil || !found || entry.RequestID != "R" || entry.Outcome.Outcome != want.Entry.Outcome.Outcome {
		t.Fatalf("replay: found=%v entry=%+v cleanup=%v err=%v", found, entry, cleanup, err)
	}
	oldFile, oldRoot := closeReadFile, closeReadRoot
	t.Cleanup(func() { closeReadFile, closeReadRoot = oldFile, oldRoot })
	sentinel := errors.New("injected writer replay close")
	for _, kind := range []string{"request-file", "receipt-file", "root"} {
		injected := false
		closeReadFile = func(f *os.File) error {
			if err := f.Close(); err != nil {
				return err
			}
			name := filepath.Base(f.Name())
			hit := (kind == "request-file" && name == filepath.Base(rp)) || (kind == "receipt-file" && strings.HasPrefix(name, "000000"))
			if hit && !injected {
				injected = true
				return sentinel
			}
			return nil
		}
		closeReadRoot = func(root *os.Root) error {
			if err := root.Close(); err != nil {
				return err
			}
			if kind == "root" && !injected {
				injected = true
				return sentinel
			}
			return nil
		}
		found, _, _, cleanup, err := r.WriterReplay(res, "R")
		closeReadFile, closeReadRoot = oldFile, oldRoot
		if !injected {
			t.Fatalf("%s: close not reached", kind)
		}
		if found || !errors.Is(cleanup, sentinel) || err == nil || !strings.Contains(err.Error(), sentinel.Error()) {
			t.Fatalf("%s: found=%v cleanup=%v err=%v", kind, found, cleanup, err)
		}
		requireCode(t, err, wire.CodeUnsupportedFilesystem)
	}
}
