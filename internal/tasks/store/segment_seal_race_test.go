package store

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
)

// sealPauses maps a store's state directory to a function its seal calls
// after each durable step, so a test can hold one store's seal in flight.
var sealPauses sync.Map

func withSealPause(t *testing.T, repo *intent.Repository, pause func(stage string)) {
	t.Helper()
	sealPauses.Store(repo.StateDir, pause)
	t.Cleanup(func() { sealPauses.Delete(repo.StateDir) })
}

// raceLeaseClaim runs one lease claim on repo whose lock waits are bounded
// by wait. Its outcome is not asserted: the tests below only need it to run
// leaseWrite's unlocked pre-round guard read.
func raceLeaseClaim(t *testing.T, repo *intent.Repository, requestID string, wait time.Duration) error {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(repo.PrimaryWorktree, intent.Dir, "tickets"))
	if err != nil || len(entries) == 0 {
		t.Fatalf("tickets: %v", err)
	}
	target := fixture.TicketID(strings.TrimSuffix(entries[0].Name(), ".json"))
	ctx := WithLeaseLockWait(context.Background(), wait)
	_, err = Lease(ctx, repo, historyActor, LeaseChoice{QueueID: fixture.QueueID, RequestID: requestID, Root: repo.PrimaryWorktree, Lease: transaction.LeaseRequest{Verb: transaction.LeaseClaim, TicketID: target, Holder: "agent-b", LeaseMinutes: "60"}}, WallClock())
	return err
}

// TestV13007_UnlockedLeaseGuardLeavesActiveSealIntact holds writer A's seal
// between publishing its segment and switching the head, under the writer
// lock, while writer B's lease runs its unlocked pre-round guard read. B
// must neither settle nor remove A's live seal: it may only take the lock to
// settle, which A holds, so B's bounded wait ends and A's seal completes
// with its segment bound and its sealed receipts readable (JSG-V0-008).
func TestV13007_UnlockedLeaseGuardLeavesActiveSealIntact(t *testing.T) {
	t.Parallel()
	repo := historyStore(t, segmentHistory)
	paused, resume := make(chan struct{}), make(chan struct{})
	withSealPause(t, repo, func(stage string) {
		if stage == "segment" {
			close(paused)
			<-resume
		}
	})
	sealed := make(chan error, 1)
	go func() { sealed <- sealSegmentsLocked(context.Background(), repo) }()
	<-paused
	before := stateTree(t, repo)
	segment := snapshot.SegmentName(1, snapshot.SegmentReceipts)
	if _, ok := before[segment]; !ok {
		close(resume)
		t.Fatalf("paused seal has not published %s", segment)
	}
	_ = raceLeaseClaim(t, repo, "race-claim", 50*time.Millisecond)
	after := stateTree(t, repo)
	close(resume)
	if err := <-sealed; err != nil {
		t.Fatalf("seal A: %v", err)
	}
	for _, p := range []string{segment, snapshot.SealMarkerPath, snapshot.SegmentManifestName(snapshot.SegmentReceipts)} {
		if after[p] != before[p] {
			t.Errorf("writer B's unlocked guard read changed %s of writer A's live seal", p)
		}
	}
	if got := segmentThrough(t, repo); got != snapshot.SegmentReceipts {
		t.Fatalf("sealed through %d, want %d", got, snapshot.SegmentReceipts)
	}
	mustSegmentAudit(t, repo, segmentHistory)
}

// TestV13007_LeaseSettlesAbandonedSealUnderLock shows that a seal a stop
// abandoned is still settled by the next lease, under the writer lock it
// takes for that, and that the lease then proceeds (JSG-V0-008).
func TestV13007_LeaseSettlesAbandonedSealUnderLock(t *testing.T) {
	t.Parallel()
	repo := historyStore(t, segmentHistory)
	withSealStop(t, repo, "segment")
	if err := sealSegmentsLocked(context.Background(), repo); err != errSegmentSealStop {
		t.Fatalf("seal: %v", err)
	}
	clearSealStop(repo)
	_ = raceLeaseClaim(t, repo, "settle-claim", time.Minute)
	if on, err := present(segmentPath(repo, snapshot.SealMarkerPath)); err != nil || on {
		t.Fatalf("the lease left the abandoned seal marker: %v %v", on, err)
	}
	if got := segmentThrough(t, repo); got != 0 {
		t.Fatalf("settled through %d, want the prior head", got)
	}
	if !receiptLinked(t, repo, 1) {
		t.Fatal("rolling back the abandoned seal unlinked receipt 1")
	}
}
