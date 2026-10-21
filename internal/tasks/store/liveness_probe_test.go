//go:build darwin || linux

package store

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
)

// TestCALV0224_LivenessWritesProbeReceipts covers the store threading of the
// V1-0887 amendment of CAL-V0-224 (decision 0547): with a temporary name
// below head in receipts/, which the listing writer audit declines as not
// plainly settled, heartbeat and renew still commit on the writer-checkpoint
// route because they probe the next receipt name instead of listing, while a
// non-liveness lease write declines to the complete route. Lease, generation
// and attempt results are unchanged. Mutation: threading false instead of
// transaction.LivenessLease leaves the heartbeat off the fast route.
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
		run := lease(fmt.Sprintf("probe-%s-%d", l.Verb, i), l)
		if !run.completed() || !run.fast() || run.rep.AttemptID != attempt || run.rep.Generation != generation {
			t.Fatalf("%s with a temporary name below head: %+v %v %v, want a fast commit", l.Verb, run.rep, run.err, run.stages)
		}
	}
	widen := lease("probe-widen", transaction.LeaseRequest{Verb: transaction.LeaseWiden, AttemptID: attempt, Generation: generation, Scope: []string{"internal/"}})
	if widen.fast() {
		t.Fatalf("widen with a temporary name below head stayed on the writer route: %v %v", widen.err, widen.stages)
	}
	if err := os.Remove(stray); err != nil {
		t.Fatal(err)
	}
}
