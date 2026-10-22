package store

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
)

// TestCALV0232_LivenessWritesNeverReadHistory covers CAL-V0-232: at two
// history sizes, heartbeat and renew through Lease, each in a fresh lease
// cache as a CLI process starts, commit on the writer-checkpoint route while
// every receipt before the checkpoint is unreadable, so a read of a history
// receipt whose error reaches the write fails it. A read that discards its
// error is not detected, and the CAL-V0-117 scheduled refresh and a declined
// write's complete-route fallback, which read history by contract, are out of
// scope. It never asserts a duration.
func TestCALV0232_LivenessWritesNeverReadHistory(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("a cleared mode does not deny reads on windows")
	}
	for _, history := range []int{70, 280} {
		t.Run(fmt.Sprint(history), func(t *testing.T) {
			repo := writerStore(t, history)
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
			claim := lease("liveness-claim", transaction.LeaseRequest{Verb: transaction.LeaseClaim, TicketID: target, Holder: "agent-1", LeaseMinutes: "60"})
			if !claim.completed() || !claim.fast() {
				t.Fatalf("claim: %+v %v %v", claim.rep, claim.err, claim.stages)
			}
			sealed := sealHistoryReceipts(t, repo.StateDir, readWriterCheckpoint(repo).Seq.Uint64())
			if sealed < uint64(history)-1 {
				t.Fatalf("sealed %d history receipts, want at least %d", sealed, history-1)
			}
			attempt, generation := claim.rep.AttemptID, claim.rep.Generation
			for i := 0; i < 2; i++ {
				for _, l := range []transaction.LeaseRequest{
					{Verb: transaction.LeaseHeartbeat, AttemptID: attempt, Generation: generation},
					{Verb: transaction.LeaseRenew, AttemptID: attempt, Generation: generation, LeaseMinutes: "60"},
				} {
					run := lease(fmt.Sprintf("liveness-%s-%d", l.Verb, i), l)
					if !run.completed() || !run.fast() || run.rep.AttemptID != attempt || run.rep.Generation != generation {
						t.Fatalf("%s %d with %d sealed history receipts: %+v %v %v", l.Verb, i, sealed, run.rep, run.err, run.stages)
					}
				}
			}
		})
	}
}

// sealHistoryReceipts clears the mode of every receipt before seq, restoring
// it when t ends, and returns how many it sealed. It skips t when a cleared
// mode still reads (a privileged process).
func sealHistoryReceipts(t *testing.T, stateDir string, seq uint64) uint64 {
	t.Helper()
	var sealed uint64
	for n := uint64(1); n < seq; n++ {
		name, err := snapshot.ReceiptName(n)
		if err != nil {
			t.Fatal(err)
		}
		p := filepath.Join(stateDir, "receipts", name)
		info, err := os.Stat(p)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(p, 0); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { os.Chmod(p, info.Mode().Perm()) })
		sealed++
		if n == 1 {
			if _, err := os.ReadFile(p); err == nil {
				t.Skip("a cleared mode still reads in this process")
			}
		}
	}
	return sealed
}
