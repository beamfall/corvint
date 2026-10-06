package store_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/intent"
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

// TestCALV0106_RetryAfterInterruptedBatchRedoesReplaysAndCompletes is the
// recovery path of an interrupted batch: the process died after entry 2's
// receipt was linked in but before its projection and head were written
// (§5.2 crash point C2). Retrying the original batch redoes entry 2, replays
// entries 0 to 2 under their request IDs, and applies 3 and 4 once each.
func TestCALV0106_RetryAfterInterruptedBatchRedoesReplaysAndCompletes(t *testing.T) {
	repo, _ := initialized(t)
	const n = 5
	ids := make([]string, n)
	envs := make([][]byte, n)
	for i := range ids {
		ids[i] = mutate(t, repo, envelope(fmt.Sprintf("create-%d", i), mutation.OpCreate, "", "", createPayload(fmt.Sprintf("Ticket %d", i)))).Ticket
		envs[i] = envelope(fmt.Sprintf("batch/%d", i), mutation.OpRefine, ids[i], "1", obj("title", str(fmt.Sprintf("Refined %d", i))))
	}
	batch := func(envelopes [][]byte) *store.BatchReport {
		t.Helper()
		report, err := store.MutateBatch(context.Background(), repo, operator(), envelopes, now(t))
		if err != nil {
			t.Fatalf("batch: %v", err)
		}
		return report
	}
	head := filepath.Join(repo.StateDir, "head.json")
	projection := filepath.Join(repo.PrimaryWorktree, intent.Dir, intent.TicketsDir, ids[2][len(ids[2])-7:]+".json")

	first := batch(envs[:2])
	headBefore, err := os.ReadFile(head)
	if err != nil {
		t.Fatal(err)
	}
	projectionBefore, err := os.ReadFile(projection)
	if err != nil {
		t.Fatal(err)
	}
	// Entry 2 commits, then the head and its projection are rewound: its
	// receipt is linked in and pending, exactly as a kill at C2 leaves it.
	interrupted := batch(envs[:3])
	pending := interrupted.Entries[2].Report.Receipt
	if pending == "" {
		t.Fatalf("entry 2 did not commit: %+v", interrupted.Entries[2])
	}
	if err := os.WriteFile(head, headBefore, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(projection, projectionBefore, 0o600); err != nil {
		t.Fatal(err)
	}

	retry := batch(envs)
	if !retry.Entries[0].Report.Redone {
		t.Error("the retry did not redo the pending receipt of entry 2")
	}
	for i, e := range retry.Entries {
		if e.Err != nil || e.Report == nil || e.Report.Outcome.Outcome != mutation.OutcomeCompleted {
			t.Fatalf("retry entry %d: %+v", i, e)
		}
		if replayed := e.Report.Outcome.Replayed; replayed != (i < 3) || (e.Report.Receipt == "") != replayed {
			t.Errorf("retry entry %d replayed %v receipt %q", i, replayed, e.Report.Receipt)
		}
	}
	order := []string{first.Entries[0].Report.Receipt, first.Entries[1].Report.Receipt, pending,
		retry.Entries[3].Report.Receipt, retry.Entries[4].Report.Receipt}
	for i := 1; i < len(order); i++ {
		if !(order[i-1] < order[i]) {
			t.Errorf("receipts out of order: %v", order)
		}
	}
	receipts, err := os.ReadDir(filepath.Join(repo.StateDir, "receipts"))
	if err != nil {
		t.Fatal(err)
	}
	if last := receipts[len(receipts)-1].Name(); last != order[4] {
		t.Errorf("last receipt %s, want %s: an entry was applied twice", last, order[4])
	}
	for i, id := range ids {
		rec := loadRecord(t, repo, id)
		if rec.Revision != "2" || rec.Title != fmt.Sprintf("Refined %d", i) {
			t.Errorf("%s: revision %s title %q, want one refine", id, rec.Revision, rec.Title)
		}
	}
}
