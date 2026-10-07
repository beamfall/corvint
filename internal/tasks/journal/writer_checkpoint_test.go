package journal

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/archive"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// sealWriter appends the trailer a valid writer checkpoint carries, so a
// case reaches the field check it names instead of the digest check.
func sealWriter(body []byte) []byte {
	sum := sha256.Sum256(body)
	return append(append([]byte(nil), body...), sum[:]...)
}

func requestPathOf(d []byte) string {
	h := hex.EncodeToString(d)
	return "requests/" + h[:2] + "/" + h + ".json"
}

// TestCALV0115_WriterCheckpointCodecIsClosed covers the closed, bounded
// writer checkpoint codec (CAL-V0-115, proposed): a round trip is exact, and
// every truncation, flipped byte, foreign profile, inconsistent aggregate,
// unordered or repeated request digest, and note that is not one strictly
// ordered live ticket entry is refused, so the writer falls back to the
// complete audit.
func TestCALV0115_WriterCheckpointCodecIsClosed(t *testing.T) {
	repo, r := setup(t)
	appendReceipt(t, repo, "MUTATION", map[string][]byte{ticketPath("A"): fixture.Ticket("A").Encode()}, "", true, true, false)
	cp := checkpointed(t, repo, r)
	a, b := sha256.Sum256([]byte("a")), sha256.Sum256([]byte("b"))
	digests := [][]byte{a[:], b[:]}
	sort.Slice(digests, func(i, j int) bool { return bytes.Compare(digests[i], digests[j]) < 0 })
	wc := &WriterCheckpoint{Checkpoint: *cp, FullSeq: 2, ReceiptBytes: 900, Cost: archive.FileSetCost{Files: 4, PayloadBytes: 1200, EntryBytes: 300, TarBytes: 4096}}
	wc.requests = append(append([]byte(nil), digests[0]...), digests[1]...)
	ref := &ticket.OperatorNoteReference{Revision: "1", Head: wire.Sum([]byte("note"))}
	noted, other := -1, -1
	for i, e := range cp.Entries {
		switch {
		case e.Path == ticketPath("A"):
			noted = i
		case other < 0 && !strings.HasPrefix(e.Path, "intent/tickets/"):
			other = i
		}
	}
	if noted < 0 || other < 0 {
		t.Fatalf("checkpoint entries %+v", cp.Entries)
	}
	wc.notes = []writerNote{{entry: uint32(noted), digest: noteDigest(ref)}}
	raw := wc.Encode()
	got, err := DecodeWriterCheckpoint(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got.Encode(), raw) || got.Requests() != 2 || got.FullSeq != 2 || got.Cost != wc.Cost || got.ReceiptBytes != 900 {
		t.Fatalf("writer checkpoint codec is not a round trip: %+v", got)
	}
	if !got.HasRequest(requestPathOf(a[:])) || !got.HasRequest(requestPathOf(b[:])) {
		t.Fatal("retained request path not found")
	}
	changed := *ref
	changed.Revision = "2"
	if !got.sameNote(ticketPath("A"), ref) || got.sameNote(ticketPath("A"), nil) || got.sameNote(ticketPath("A"), &changed) || !got.sameNote(ticketPath("B"), nil) || got.sameNote(ticketPath("B"), ref) {
		t.Fatalf("note references %+v", got.notes)
	}
	c := sha256.Sum256([]byte("c"))
	for _, p := range []string{requestPathOf(c[:]), strings.ToUpper(requestPathOf(a[:])), "requests/" + hex.EncodeToString(a[:]) + ".json", "intent/queue.json"} {
		if got.HasRequest(p) {
			t.Errorf("%s: reported as retained", p)
		}
	}
	if rel, err := filepath.Rel(repo.StateDir, WriterCheckpointPath(repo.StateDir)); err != nil || !strings.HasPrefix(rel, "..") {
		t.Fatalf("writer checkpoint path is inside the state directory: %s", rel)
	}

	for n := 0; n < len(raw); n++ {
		if _, err := DecodeWriterCheckpoint(raw[:n]); err == nil {
			t.Fatalf("truncation to %d bytes accepted", n)
		}
	}
	for i := range raw {
		flipped := append([]byte(nil), raw...)
		flipped[i] ^= 0x01
		if _, err := DecodeWriterCheckpoint(flipped); err == nil {
			t.Fatalf("flipped byte %d accepted", i)
		}
	}

	body := raw[:len(raw)-sha256.Size]
	embedded := len(writerMagic) + 8
	cpLen := int(binary.BigEndian.Uint64(body[len(writerMagic):]))
	aggregates := embedded + cpLen
	field := func(i int, v uint64) []byte {
		out := append([]byte(nil), body...)
		binary.BigEndian.PutUint64(out[aggregates+8*i:], v)
		return sealWriter(out)
	}
	requestsAt := aggregates + 7*8
	swapped := append([]byte(nil), body...)
	copy(swapped[requestsAt:], digests[1])
	copy(swapped[requestsAt+sha256.Size:], digests[0])
	repeated := append([]byte(nil), body...)
	copy(repeated[requestsAt+sha256.Size:], digests[0])
	foreign := append([]byte(nil), body...)
	copy(foreign, strings.Replace(writerMagic, "/0", "/1", 1))
	longCP := append([]byte(nil), body...)
	binary.BigEndian.PutUint64(longCP[len(writerMagic):], uint64(len(body)))
	notesAt := requestsAt + 2*sha256.Size
	note := func(edit func(out []byte) []byte) []byte {
		return sealWriter(edit(append([]byte(nil), body...)))
	}
	entry := func(i uint32) []byte {
		return note(func(out []byte) []byte { binary.BigEndian.PutUint32(out[notesAt+8:], i); return out })
	}
	for name, bad := range map[string][]byte{
		"foreign profile":         sealWriter(foreign),
		"embedded length":         sealWriter(longCP),
		"truncated aggregates":    sealWriter(body[:aggregates+8]),
		"request count":           field(6, 3),
		"file count":              field(2, 5),
		"full seq zero":           field(0, 0),
		"full seq after seq":      field(0, 3),
		"receipt bytes > payload": field(1, 1201),
		"unordered requests":      sealWriter(swapped),
		"repeated request":        sealWriter(repeated),
		"trailing digest bytes":   sealWriter(append(append([]byte(nil), body...), 1)),
		"truncated notes":         sealWriter(body[:notesAt+4]),
		"note count":              note(func(out []byte) []byte { binary.BigEndian.PutUint64(out[notesAt:], 2); return out }),
		"note entry out of range": entry(uint32(len(cp.Entries))),
		"note entry not a ticket": entry(uint32(other)),
		"repeated note entry": note(func(out []byte) []byte {
			binary.BigEndian.PutUint64(out[notesAt:], 2)
			return append(out, out[notesAt+8:]...)
		}),
		"over the bound": make([]byte, MaxWriterCheckpointBytes+1),
	} {
		if _, err := DecodeWriterCheckpoint(bad); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}
