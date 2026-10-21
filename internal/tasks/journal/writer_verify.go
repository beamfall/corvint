package journal

import (
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/snapshot"
)

// VerifyWriterCheckpoint compares every value writer checkpoint old carries
// for the capacity model with what this complete audit measured from the
// bytes it read (JSG-V0-003): each entry's sequence, digest and afterimage
// size at or before old.Seq, the request afterimages receipts 1..old.Seq
// posted, the receipt bytes and the archive cost. An entry posted again after
// old.Seq is not compared; the next writer walks it. The intent aggregate is
// the decoder's sum of the entry sizes compared here.
//
// ok is false when this result cannot verify: it is not a settled complete
// audit from receipt 1 with walked sizes. Otherwise a non-nil error names the
// first value that differs, and the caller discards the checkpoint.
func (res *Result) VerifyWriterCheckpoint(old *WriterCheckpoint, physical map[string]PhysicalFile) (ok bool, err error) {
	if res == nil || old == nil || res.chain == nil || res.chain.view || res.chain.seal != nil || res.chain.resumed != 0 || res.Mode != ModeFull || res.StructuralConsistency != "CONSISTENT" || res.Pending || res.StagingPresent || res.Head == nil || physical == nil {
		return false, nil
	}
	differs := func(p, why string) error { return errCheckpoint(p, "carried "+why+" differs from the complete audit") }
	seq := old.Seq.Uint64()
	if seq > res.LastSeq.Uint64() || old.QueueID != res.Head.QueueID || old.InitSha256 != res.Head.InitSha256 {
		return true, differs("writer checkpoint", "head")
	}
	// Entries: every path the audit last saw at or before seq is carried
	// with that sequence, digest and size, and nothing else is.
	compared, unwalked := 0, false
	err = old.each([]string{"requests/"}, func(p string, l latest) error {
		got, found := res.chain.canonical[p]
		if !found {
			return differs(p, "entry")
		}
		if got.seq.Uint64() > seq {
			return nil
		}
		if got.bytes < 0 {
			unwalked = true
			return errCheckpoint(p, "afterimage size was not walked")
		}
		sameDigest := (got.digest == nil) == (l.digest == nil) && (got.digest == nil || *got.digest == *l.digest)
		if got.seq != l.seq || !sameDigest || got.bytes != l.bytes {
			return differs(p, "entry")
		}
		compared++
		return nil
	})
	if unwalked {
		return false, nil
	}
	if err != nil {
		return true, err
	}
	// Requests and the cost they and the receipts carry.
	measured := &WriterCheckpoint{}
	requests, entries := uint64(0), 0
	for p, l := range res.chain.canonical {
		if l.seq.Uint64() > seq {
			continue
		}
		if !strings.HasPrefix(p, "requests/") {
			entries++
			continue
		}
		after, carried := old.RequestAfterimage(p)
		if !carried || l.digest == nil || after != *l.digest {
			return true, differs(p, "request afterimage")
		}
		if err := measured.charge(physical, p, l.digest); err != nil {
			return false, nil
		}
		requests++
	}
	if entries != compared {
		return true, differs("writer checkpoint", "entry count")
	}
	if requests != old.Requests() {
		return true, differs("requests", "request count")
	}
	for n := uint64(1); n <= seq; n++ {
		name, err := snapshot.ReceiptName(n)
		if err != nil {
			return true, err
		}
		if err := measured.charge(physical, "receipts/"+name, nil); err != nil {
			return false, nil
		}
	}
	if measured.ReceiptBytes != old.ReceiptBytes {
		return true, differs("receipts", "receipt bytes")
	}
	measured.Cost.Files = old.Cost.Files
	if measured.Cost != old.Cost || uint64(old.Cost.Files) != seq+requests {
		return true, differs("writer checkpoint", "archive cost")
	}
	return true, nil
}
