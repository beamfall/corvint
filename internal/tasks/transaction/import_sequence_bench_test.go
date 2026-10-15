package transaction

import (
	"fmt"
	"testing"
)

// importBenchBatch is the mean records per IMPORT_APPLY batch of the 2,894-item
// Beamfall first import (414 batches).
const importBenchBatch = 7

// importBenchStore returns an ordinary Input holding n imported tickets, built
// through an ImportSequence, plus the next unused ticket number.
func importBenchStore(b testing.TB, n int) Input {
	b.Helper()
	in := roadmapInitialized(b)
	r := importRequest()
	seq, err := NewImportSequence(r.QueueID, r.Actor, in)
	if err != nil {
		b.Fatal(err)
	}
	defer seq.Close()
	for next := 0; next < n; {
		r = importRequest()
		r.RequestID = fmt.Sprintf("seed-%d", next)
		for k := 0; k < importBenchBatch && next < n; k++ {
			r.Records = append(r.Records, imported(fmt.Sprintf("BF-%05d", next), "seed").Encode())
			next++
		}
		result, batch := seq.Prepare(r, in)
		if batch == nil {
			b.Fatalf("seed %d: %+v", next, result)
		}
		if err = seq.Advance(batch); err != nil {
			b.Fatal(err)
		}
		in.Head, in.HeadReceipt = batch.Head(), batch.Receipt()
	}
	plain, err := NewInventory(seq.input.Inventory.Files(), seq.input.Inventory.Directories())
	if err != nil {
		b.Fatal(err)
	}
	in.Inventory = plain
	for _, rec := range seq.records {
		in.CanonicalTickets = append(in.CanonicalTickets, rec.Encode())
	}
	return in
}

func importBenchRequest(n, first int) Request {
	r := importRequest()
	r.RequestID = fmt.Sprintf("bench-%d", n)
	for k := 0; k < importBenchBatch; k++ {
		r.Records = append(r.Records, imported(fmt.Sprintf("NEW-%07d", first+k), "bench").Encode())
	}
	return r
}

// BenchmarkCALV0018_ImportBatchModel measures one IMPORT_APPLY batch of
// importBenchBatch new records against stores of growing size. "full" is the
// public Model over the whole store (the per-batch cost before CAL-V0-018's
// sequence); "sequence" is ImportSequence Prepare plus Advance, whose cost
// should not grow with the store.
func BenchmarkCALV0018_ImportBatchModel(b *testing.B) {
	for _, size := range []int{500, 1500, 3000} {
		in := importBenchStore(b, size)
		b.Run(fmt.Sprintf("full/tickets=%d", size), func(b *testing.B) {
			for n := 0; n < b.N; n++ {
				if result := Model(importBenchRequest(n, 0), in); result.Plan == nil {
					b.Fatalf("full: %+v", result)
				}
			}
		})
		b.Run(fmt.Sprintf("sequence/tickets=%d", size), func(b *testing.B) {
			r := importRequest()
			seq, err := NewImportSequence(r.QueueID, r.Actor, in)
			if err != nil {
				b.Fatal(err)
			}
			defer seq.Close()
			current := in
			b.ResetTimer()
			for n := 0; n < b.N; n++ {
				result, batch := seq.Prepare(importBenchRequest(n, n*importBenchBatch), current)
				if batch == nil {
					b.Fatalf("sequence: %+v", result)
				}
				if err = seq.Advance(batch); err != nil {
					b.Fatal(err)
				}
				current.Head, current.HeadReceipt = batch.Head(), batch.Receipt()
			}
		})
	}
}
