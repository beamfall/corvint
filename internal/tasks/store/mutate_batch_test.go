package store_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestCALV0106_ClaimCommitsBetweenChunks is the issue 625 hand-off: a batch
// longer than one chunk releases the writer lock between chunks, and a claim
// made at that point commits between the two chunks' receipts instead of
// waiting for the whole batch. Each chunk is bounded by BatchChunkEntries.
func TestCALV0106_ClaimCommitsBetweenChunks(t *testing.T) {
	s := newLeaseStore(t)
	n := store.BatchChunkEntries + 2
	ids := make([]string, n)
	for i := range ids {
		ids[i] = s.ticket(t, fmt.Sprintf("Batch ticket %d", i))
	}
	other := s.ticket(t, "Claimed between chunks")
	envelopes := make([][]byte, n)
	for i, id := range ids {
		envelopes[i] = envelope(fmt.Sprintf("batch/%d", i), mutation.OpRefine, id, "1", obj("title", str(fmt.Sprintf("Refined %d", i))))
	}
	// A live clock that the hook moves past the claim, as the wall clock does.
	minute := 1
	ctx := store.WithClock(context.Background(), func() wire.Timestamp { return s.at(t, minute) })
	var claim *store.Report
	yields := []int{}
	ctx = store.WithBatchYieldHook(ctx, func(next int) {
		yields = append(yields, next)
		if claim == nil {
			claim = s.claim(t, "claim-between", other, 2, "src/")
			minute = 3
		}
	})
	report, err := store.MutateBatch(ctx, s.repo, operator(), envelopes, s.at(t, 0))
	if err != nil {
		t.Fatalf("batch: %v", err)
	}
	if claim == nil || len(yields) == 0 || report.Chunks != len(yields)+1 {
		t.Fatalf("chunks %d, yields %v: the batch never released the lock between chunks", report.Chunks, yields)
	}
	sizes := map[int]int{}
	for i, e := range report.Entries {
		if e.Err != nil || e.Report == nil || e.Report.Outcome.Outcome != mutation.OutcomeCompleted || e.Report.Receipt == "" {
			t.Fatalf("entry %d: %+v", i, e)
		}
		sizes[e.Chunk]++
	}
	for chunk, size := range sizes {
		if size > store.BatchChunkEntries {
			t.Errorf("chunk %d ran %d entries, more than %d", chunk, size, store.BatchChunkEntries)
		}
	}
	first := yields[0]
	before, after := report.Entries[first-1].Report.Receipt, report.Entries[first].Report.Receipt
	if !(before < claim.Receipt && claim.Receipt < after) {
		t.Errorf("claim receipt %s is not between chunk receipts %s and %s", claim.Receipt, before, after)
	}
}

// TestCALV0106_UnadmittedOrMalformedBatchWritesNothing checks that the store
// boundary decodes every envelope before taking the lock.
func TestCALV0106_UnadmittedOrMalformedBatchWritesNothing(t *testing.T) {
	repo, _ := initialized(t)
	created := mutate(t, repo, envelope("req-create", mutation.OpCreate, "", "", createPayload("First ticket")))
	good := envelope("b/0", mutation.OpRefine, created.Ticket, "1", obj("title", str("Refined")))
	before := storeDigest(t, repo)
	_, err := store.MutateBatch(context.Background(), repo, operator(), [][]byte{good, []byte("{}\n")}, now(t))
	if err == nil {
		t.Fatal("a malformed second envelope was accepted")
	}
	report, err := store.MutateBatch(context.Background(), repo, mutation.Binding{ID: "someone", Role: "OWNER"}, [][]byte{good}, now(t))
	if err != nil || report.Entries[0].Report == nil || report.Entries[0].Report.Outcome.Outcome == mutation.OutcomeCompleted {
		t.Fatalf("an envelope for another actor was not refused: %+v %v", report, err)
	}
	if after := storeDigest(t, repo); after != before {
		t.Error("a refused batch changed the store")
	}
}
