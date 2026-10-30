package journal

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"math"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/archive"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// anchorMagic is the frozen encoding of a retired-prefix anchor (JRT-V0-004):
// the writer checkpoint layout of profile taskman-writer-checkpoint/1. The
// anchor is journal state an operator retains, not a derived file, so it does
// not follow the writer checkpoint's later versions; a seal written by any
// earlier binary still decodes (decision 0549 S2).
const anchorMagic = "taskman-writer-checkpoint/1\n"

// encodeAnchor renders w in the frozen anchor layout: the magic, the read
// checkpoint encoding, the aggregates, the request records, the notes, the
// invalidation digest and a SHA-256 trailer.
func encodeAnchor(w *WriterCheckpoint) ([]byte, error) {
	w, err := w.full()
	if err != nil {
		return nil, err
	}
	cp := w.Checkpoint.Encode()
	buf := make([]byte, 0, len(anchorMagic)+8+len(cp)+7*8+len(w.requests)+8+writerNoteBytes*len(w.notes)+2*sha256.Size)
	buf = append(buf, anchorMagic...)
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
	buf = append(buf, w.Invalidation[:]...)
	sum := sha256.Sum256(buf)
	return append(buf, sum[:]...), nil
}

// decodeAnchor parses the frozen anchor layout. The result carries every
// entry, no sizes and no intent aggregate: an anchor seeds a sealed walk,
// which measures nothing from it.
func decodeAnchor(raw []byte) (*WriterCheckpoint, error) {
	bad := func(why string) error { return errCheckpoint("anchor", why) }
	if len(raw) > MaxWriterCheckpointBytes {
		return nil, bad("larger than its bound")
	}
	if name, _, _ := strings.Cut(anchorMagic, "/"); bytes.HasPrefix(raw, []byte(name+"/")) && !bytes.HasPrefix(raw, []byte(anchorMagic)) {
		return nil, wire.Errorf(wire.CodeUnsupportedVersion, "anchor", "another %s version", name)
	}
	if len(raw) < len(anchorMagic)+8+7*8+8+2*sha256.Size {
		return nil, bad("truncated")
	}
	body, trailer := raw[:len(raw)-sha256.Size], raw[len(raw)-sha256.Size:]
	if sum := sha256.Sum256(body); !bytes.Equal(sum[:], trailer) {
		return nil, bad("trailer digest differs")
	}
	if !bytes.HasPrefix(body, []byte(anchorMagic)) {
		return nil, bad("profile differs")
	}
	rest := body[len(anchorMagic):]
	n := binary.BigEndian.Uint64(rest)
	rest = rest[8:]
	if n > MaxCheckpointBytes || n > uint64(len(rest)) {
		return nil, bad("embedded checkpoint length")
	}
	cp, err := decodeEmbeddedCheckpoint(rest[:n])
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
	if v[2] > math.MaxInt || requests > uint64(wire.MaxArchiveFiles) || uint64(len(rest)) < requests*writerRequestBytes+8+sha256.Size {
		return nil, bad("request digest count")
	}
	w.Cost.Files = int(v[2])
	if seq > uint64(wire.MaxArchiveFiles) || v[2] != seq+requests {
		return nil, bad("file count differs from receipts plus requests")
	}
	if w.FullSeq < 1 || w.FullSeq > seq || w.ReceiptBytes > w.Cost.PayloadBytes {
		return nil, bad("aggregate bounds")
	}
	digests, rest := rest[:requests*writerRequestBytes], rest[requests*writerRequestBytes:]
	if err := orderedRequests(digests); err != nil {
		return nil, err
	}
	w.requests = append([]byte(nil), digests...)
	notes := binary.BigEndian.Uint64(rest)
	rest = rest[8:]
	if notes > uint64(wire.MaxTicketsPerQueue) || uint64(len(rest)) != notes*writerNoteBytes+sha256.Size {
		return nil, bad("note count")
	}
	copy(w.Invalidation[:], rest[notes*writerNoteBytes:])
	if w.notes, err = decodeNotes(rest[:notes*writerNoteBytes], cp.Entries); err != nil {
		return nil, err
	}
	return w, nil
}
