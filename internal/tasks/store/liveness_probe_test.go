//go:build darwin || linux

package store

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
)

// TestCALV0224_LivenessWritesProbeReceipts covers the store threading of the
// V1-0887 amendment of CAL-V0-224 (decision 0549): with a temporary name
// below head in a quiescent receipts/, which the listing writer audit
// declines as not plainly settled, heartbeat and renew still commit on the
// writer-checkpoint route because they probe the next receipt name instead
// of listing. Lease, generation and attempt results are unchanged. Over a
// broken history, a receipt below the tail removed, an eligible
// non-liveness verb (CLAIM) lists exactly, is refused, and leaves the
// published journal as it was. Mutations: threading false instead of
// transaction.LivenessLease leaves the heartbeat off the fast route;
// threading true for every lease verb commits the claim over the broken
// history.
func TestCALV0224_LivenessWritesProbeReceipts(t *testing.T) {
	repo := writerStore(t, 70)
	holdLeasePolicy(t, repo)
	root := filepath.Join(filepath.Dir(repo.PrimaryWorktree), "worktree")
	holdGit(t, root, "init", "-q", "-b", "main")
	holdGit(t, root, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "--allow-empty", "-m", "base")
	_, ticketFile := mutationBoundaryFiles(t, repo)
	target := fixture.TicketID(strings.TrimSuffix(filepath.Base(ticketFile), ".json"))
	lease := func(id string, l transaction.LeaseRequest) writerRun {
		return stagedWrite(func(ctx context.Context) (*Report, error) {
			return Lease(ctx, repo, historyActor, LeaseChoice{QueueID: fixture.QueueID, RequestID: id, Root: root, Lease: l}, WallClock())
		}, nil)
	}
	claim := lease("probe-claim", transaction.LeaseRequest{Verb: transaction.LeaseClaim, TicketID: target, Holder: "agent-1", LeaseMinutes: "60"})
	if !claim.completed() || !claim.fast() {
		t.Fatalf("claim: %+v %v %v", claim.rep, claim.err, claim.stages)
	}
	name, err := snapshot.ReceiptName(readWriterCheckpoint(repo).Seq.Uint64())
	if err != nil {
		t.Fatal(err)
	}
	stray := filepath.Join(repo.StateDir, "receipts", name+".tmp-1-x")
	fixture.Write(t, stray, []byte("x"))
	attempt, generation := claim.rep.AttemptID, claim.rep.Generation
	for i, l := range []transaction.LeaseRequest{
		{Verb: transaction.LeaseHeartbeat, AttemptID: attempt, Generation: generation},
		{Verb: transaction.LeaseRenew, AttemptID: attempt, Generation: generation, LeaseMinutes: "60"},
	} {
		awaitReceiptsQuiescent()
		run := lease(fmt.Sprintf("probe-%s-%d", l.Verb, i), l)
		if !run.completed() || !run.fast() || run.rep.AttemptID != attempt || run.rep.Generation != generation {
			t.Fatalf("%s with a temporary name below head: %+v %v %v, want a fast commit", l.Verb, run.rep, run.err, run.stages)
		}
	}
	if err := os.Remove(stray); err != nil {
		t.Fatal(err)
	}

	second, err := snapshot.ReceiptName(2)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(repo.StateDir, "receipts", second)); err != nil {
		t.Fatal(err)
	}
	awaitReceiptsQuiescent()
	entries, err := os.ReadDir(filepath.Dir(ticketFile))
	if err != nil || len(entries) < 2 {
		t.Fatalf("tickets: %d %v", len(entries), err)
	}
	other := fixture.TicketID(strings.TrimSuffix(entries[1].Name(), ".json"))
	if other == target {
		other = fixture.TicketID(strings.TrimSuffix(entries[0].Name(), ".json"))
	}
	before := mutationPublished(t, repo)
	broken := lease("probe-broken-claim", transaction.LeaseRequest{Verb: transaction.LeaseClaim, TicketID: other, Holder: "agent-2", LeaseMinutes: "60"})
	if broken.completed() || broken.fast() || broken.err == nil {
		t.Fatalf("claim over a removed receipt below the tail: %+v %v %v, want a refusal", broken.rep, broken.err, broken.stages)
	}
	if after := mutationPublished(t, repo); after != before {
		t.Fatalf("refused claim changed the published journal:\n%s\nwant\n%s", after, before)
	}
}

// awaitReceiptsQuiescent waits past the journal's one-second quiescence, so
// a liveness audit probes receipts/ instead of listing it (CAL-V0-224).
// The directory's change time cannot be set back, so it is reached by
// waiting.
func awaitReceiptsQuiescent() {
	time.Sleep(1500 * time.Millisecond)
}
