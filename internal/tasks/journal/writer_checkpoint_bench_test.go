package journal

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/archive"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// benchWriterCheckpoint encodes a writer checkpoint whose complete checkpoint
// carries `tickets` live ticket entries (ticket-proportional) and whose
// retained request index carries `requests` records (history-proportional),
// shaped like a store with one request per receipt.
func benchWriterCheckpoint(b *testing.B, tickets, requests int) (base, raw []byte) {
	b.Helper()
	q, err := wire.ParseQueueID("", fixture.QueueID)
	if err != nil {
		b.Fatal(err)
	}
	seq := uint64(max(requests, 1))
	cp := Checkpoint{QueueID: q, PrimaryWorktree: "/fixture", Seq: wire.SizeOf(seq), ReceiptSha256: wire.Sum([]byte("receipt")), Generation: wire.SizeOf(1), InitSha256: wire.Sum([]byte("init")), SemanticCoverage: "KNOWN_CODECS"}
	for i := 0; i < tickets; i++ {
		sum := wire.Sum([]byte(fmt.Sprint("ticket", i)))
		cp.Entries = append(cp.Entries, CheckpointEntry{Path: fmt.Sprintf("intent/tickets/T-%04d.json", i), Seq: wire.SizeOf(1), Sha256: &sum})
	}
	sort.Slice(cp.Entries, func(i, j int) bool { return cp.Entries[i].Path < cp.Entries[j].Path })
	digests := make([][]byte, requests)
	for i := range digests {
		d, after := sha256.Sum256([]byte(fmt.Sprint("request", i))), sha256.Sum256([]byte(fmt.Sprint("afterimage", i)))
		digests[i] = append(append([]byte(nil), d[:]...), after[:]...)
	}
	sort.Slice(digests, func(i, j int) bool { return string(digests[i][:sha256.Size]) < string(digests[j][:sha256.Size]) })
	wc := &WriterCheckpoint{Checkpoint: cp, FullSeq: 1, ReceiptBytes: 1000, Cost: archive.FileSetCost{Files: int(seq) + requests, PayloadBytes: 2000, EntryBytes: 300, TarBytes: 4096}}
	for _, d := range digests {
		wc.requests = append(wc.requests, d...)
	}
	wc.sizes = make([]uint64, len(cp.Entries))
	for i := range wc.sizes {
		wc.sizes[i] = 100
	}
	wc.Intent = IntentAggregate{Tickets: tickets, Cost: archive.FileSetCost{Files: tickets, PayloadBytes: uint64(100 * tickets)}}
	base, raw, err = wc.Encode()
	if err != nil {
		b.Fatal(err)
	}
	if _, err := DecodeWriterCheckpoint(raw, base); err != nil {
		b.Fatal(err)
	}
	return base, raw
}

// BenchmarkDecodeWriterCheckpoint separates the history-proportional request
// index from the ticket-proportional checkpoint entries (V1-1408): with the
// ticket count fixed, the three request sizes show how decode time and size
// grow with history; the zero-ticket row isolates the request index.
func BenchmarkDecodeWriterCheckpoint(b *testing.B) {
	for _, c := range []struct{ tickets, requests int }{{880, 3000}, {880, 6000}, {880, 13000}, {0, 13000}, {880, 0}} {
		b.Run(fmt.Sprintf("tickets=%d/requests=%d", c.tickets, c.requests), func(b *testing.B) {
			base, raw := benchWriterCheckpoint(b, c.tickets, c.requests)
			b.ReportMetric(float64(len(raw)), "bytes")
			b.ReportMetric(float64(len(base)), "base-bytes")
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := DecodeWriterCheckpoint(raw, base); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
