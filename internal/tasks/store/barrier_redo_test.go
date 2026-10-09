package store_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// pendingUnpause pauses at scope, then interrupts an UNPAUSE after its receipt
// is published: before the barrier unlink ("predelete") or after it and before
// the head ("prehead"). It returns the pending receipt's name and bytes.
func pendingUnpause(t *testing.T, scope, point string) (*intent.Repository, string, []byte) {
	t.Helper()
	repo, _ := initialized(t)
	mutate(t, repo, envelope("create", mutation.OpCreate, "", "", createPayload("pending unpause")))
	if scope == "ALL" {
		reconciliationBarrier(t, repo, "ALL")
	} else {
		changeBarrier(t, repo, transaction.Pause, "pause")
	}
	fault := errors.New("injected " + point + " fault")
	var remove func() error
	switch point {
	case "predelete":
		remove = func() error { return fault }
	case "prehead":
		restore := store.SetPublishFaultForTest(func(a transaction.Artifact) error {
			if a.Role == "HEAD" {
				return fault
			}
			return nil
		})
		defer restore()
	}
	report, err := store.BarrierWithFaultsForTest(context.Background(), repo, operator(), barrierRequest(transaction.Unpause, "remove"), now(t), nil, remove)
	if !errors.Is(err, fault) || report.Receipt == "" {
		t.Fatalf("fault %s: %+v %v", point, report, err)
	}
	_, statErr := os.Lstat(filepath.Join(repo.StateDir, "barrier.json"))
	if (point == "predelete") != (statErr == nil) {
		t.Fatalf("barrier presence after %s: %v", point, statErr)
	}
	raw, err := os.ReadFile(filepath.Join(repo.StateDir, "receipts", report.Receipt))
	if err != nil {
		t.Fatal(err)
	}
	return repo, report.Receipt, raw
}

// requireSettled checks that the head names exactly the pending receipt, the
// barrier is gone, and the receipt chain and projection audit pass.
func requireSettled(t *testing.T, repo *intent.Repository, name string, pending []byte) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(repo.StateDir, "receipts", name))
	if err != nil || !bytes.Equal(raw, pending) {
		t.Fatalf("pending receipt changed: %v", err)
	}
	rc, err := snapshot.DecodeReceipt(raw)
	if err != nil {
		t.Fatal(err)
	}
	headRaw, err := os.ReadFile(filepath.Join(repo.StateDir, "head.json"))
	if err != nil {
		t.Fatal(err)
	}
	head, err := snapshot.DecodeHead(headRaw)
	if err != nil {
		t.Fatal(err)
	}
	settled := head.LastSeq == rc.Seq && head.LastReceiptSha256 != nil && *head.LastReceiptSha256 == wire.Sum(raw)
	if !settled && head.LastSeq.Uint64() == rc.Seq.Uint64()+1 {
		// A recovering mutation commits its own receipt after the redo.
		name, _ := snapshot.ReceiptName(head.LastSeq.Uint64())
		nextRaw, err := os.ReadFile(filepath.Join(repo.StateDir, "receipts", name))
		if err != nil {
			t.Fatal(err)
		}
		next, err := snapshot.DecodeReceipt(nextRaw)
		settled = err == nil && next.Prev != nil && *next.Prev == wire.Sum(raw)
	}
	if !settled {
		t.Fatalf("head does not settle the pending receipt: %+v", head)
	}
	if _, err := os.Lstat(filepath.Join(repo.StateDir, "barrier.json")); !os.IsNotExist(err) {
		t.Fatalf("barrier left: %v", err)
	}
	q, _ := wire.ParseQueueID("", fixture.QueueID)
	proof, err := (journal.Reader{Source: journal.Native{StateDir: repo.StateDir, PrimaryWorktree: repo.PrimaryWorktree}, QueueID: q, PrimaryWorktree: repo.PrimaryWorktree}).Audit("intent/queue.json", "intent/tickets/AT-0002.json")
	if err != nil || proof.Pending {
		t.Fatalf("audit after recovery: %+v %v", proof, err)
	}
}

// TestV10309_PendingUnpauseRecovers is the V1-0309 STO-01 regression: an
// UNPAUSE interrupted after its receipt is published, before or after the
// barrier unlink, is settled by a supported command, the identical receipt
// becomes the head, and a second recovery changes nothing.
func TestV10309_PendingUnpauseRecovers(t *testing.T) {
	for _, tc := range []struct{ scope, point, via string }{
		{"ADMISSION", "predelete", "barrier"},
		{"ADMISSION", "prehead", "barrier"},
		{"ADMISSION", "predelete", "mutation"},
		{"ADMISSION", "prehead", "mutation"},
		{"ALL", "predelete", "barrier"},
		{"ALL", "prehead", "barrier"},
	} {
		t.Run(tc.scope+"/"+tc.point+"/"+tc.via, func(t *testing.T) {
			repo, name, pending := pendingUnpause(t, tc.scope, tc.point)
			recoverOnce := func(id string) *store.Report {
				t.Helper()
				if tc.via == "mutation" {
					return mutate(t, repo, envelope(id, mutation.OpCreate, "", "", createPayload(id)))
				}
				report, err := store.Barrier(context.Background(), repo, operator(), barrierRequest(transaction.Unpause, "remove"), now(t))
				if err != nil {
					t.Fatalf("barrier recovery: %v", err)
				}
				return report
			}
			first := recoverOnce("after-recovery")
			if !first.Redone {
				t.Fatalf("recovery did not redo: %+v", first)
			}
			if tc.via == "barrier" && (!first.Outcome.Replayed || first.Receipt != "") {
				t.Fatalf("retry did not replay the pending receipt: %+v", first)
			}
			requireSettled(t, repo, name, pending)
			before := storeDigest(t, repo)
			again, err := store.Barrier(context.Background(), repo, operator(), barrierRequest(transaction.Unpause, "remove"), now(t))
			if err != nil || again.Redone || !again.Outcome.Replayed || storeDigest(t, repo) != before {
				t.Fatalf("second recovery changed state: %+v %v", again, err)
			}
		})
	}
}

// TestV10309_PendingUnpauseAfterHeadReplays covers the fault point after the
// head is published: the transaction is settled, so a retry replays and
// changes nothing.
func TestV10309_PendingUnpauseAfterHeadReplays(t *testing.T) {
	repo, _ := initialized(t)
	changeBarrier(t, repo, transaction.Pause, "pause")
	done := changeBarrier(t, repo, transaction.Unpause, "remove")
	raw, err := os.ReadFile(filepath.Join(repo.StateDir, "receipts", done.Receipt))
	if err != nil {
		t.Fatal(err)
	}
	before := storeDigest(t, repo)
	for range 2 {
		got := changeBarrier(t, repo, transaction.Unpause, "remove")
		if got.Redone || !got.Outcome.Replayed || storeDigest(t, repo) != before {
			t.Fatalf("retry after head changed state: %+v", got)
		}
	}
	requireSettled(t, repo, done.Receipt, raw)
}

// TestV10309_PendingUnpauseChangedBarrierRefuses: a barrier holding a third
// value is never deleted and the head never advances.
func TestV10309_PendingUnpauseChangedBarrierRefuses(t *testing.T) {
	repo, _, _ := pendingUnpause(t, "ADMISSION", "predelete")
	path := filepath.Join(repo.StateDir, "barrier.json")
	changed := []byte("third value barrier\n")
	fixture.Write(t, path, changed)
	before := storeDigest(t, repo)
	if _, err := store.Barrier(context.Background(), repo, operator(), barrierRequest(transaction.Unpause, "remove"), now(t)); err == nil {
		t.Fatal("changed barrier accepted")
	}
	if storeDigest(t, repo) != before {
		t.Fatal("refused recovery changed the store")
	}
	if current, err := os.ReadFile(path); err != nil || !bytes.Equal(current, changed) {
		t.Fatalf("third-value barrier changed: %v", err)
	}
}

// TestV10309_PendingUnpauseRecoversOffIntentBranch: a pending receipt that
// writes no intent projection is settled while the primary is off the intent
// branch, as the barrier command itself is branch independent.
func TestV10309_PendingUnpauseRecoversOffIntentBranch(t *testing.T) {
	repo, name, pending := pendingUnpause(t, "ADMISSION", "predelete")
	fixture.Write(t, filepath.Join(repo.CommonDir, "HEAD"), []byte("ref: refs/heads/feature\n"))
	report, err := store.Barrier(context.Background(), repo, operator(), barrierRequest(transaction.Unpause, "remove"), now(t))
	if err != nil || !report.Redone || !report.Outcome.Replayed {
		t.Fatalf("off-branch recovery: %+v %v", report, err)
	}
	requireSettled(t, repo, name, pending)
}

// TestV10309_BarrierSettlesPendingMutation: the barrier command, like every
// other writer, settles a pending receipt before it models its own request.
func TestV10309_BarrierSettlesPendingMutation(t *testing.T) {
	repo := pendingMutation(t)
	report, err := store.Barrier(context.Background(), repo, operator(), barrierRequest(transaction.Pause, "pause"), now(t))
	if err != nil || !report.Redone || report.Receipt == "" {
		t.Fatalf("barrier over pending mutation: %+v %v", report, err)
	}
	q, _ := wire.ParseQueueID("", fixture.QueueID)
	if _, err := (journal.Reader{Source: journal.Native{StateDir: repo.StateDir, PrimaryWorktree: repo.PrimaryWorktree}, QueueID: q, PrimaryWorktree: repo.PrimaryWorktree}).Audit(); err != nil {
		t.Fatal(err)
	}
}
