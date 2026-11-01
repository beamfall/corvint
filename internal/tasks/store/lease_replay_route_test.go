package store

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
)

// TestCALV0236_SupersededReplayAdvisoryOnEachRoute (V1-1426): a replay of a
// claim whose attempt has since moved to a later generation carries the one
// advisory warning on both lease routes, and the test observes which route
// served the replay from its recorded stages, so a route that stopped
// emitting the warning, or a store that silently served both cases from one
// route, fails here.
func TestCALV0236_SupersededReplayAdvisoryOnEachRoute(t *testing.T) {
	for _, c := range []struct {
		name       string
		checkpoint bool
	}{{"complete", false}, {"writer-checkpoint", true}} {
		t.Run(c.name, func(t *testing.T) {
			repo := historyStore(t, 70)
			if c.checkpoint {
				lowerWriterCheckpoint(t, repo)
			} else {
				writerCheckpointFloors.Store(repo.StateDir, uint64(1)<<40)
				t.Cleanup(func() { writerCheckpointFloors.Delete(repo.StateDir) })
			}
			holdLeasePolicy(t, repo)
			root := filepath.Join(filepath.Dir(repo.PrimaryWorktree), "worktree")
			holdGit(t, root, "init", "-q", "-b", "main")
			holdGit(t, root, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "--allow-empty", "-m", "base")
			_, ticketFile := mutationBoundaryFiles(t, repo)
			target := fixture.TicketID(strings.TrimSuffix(filepath.Base(ticketFile), ".json"))
			lease := func(id string, l transaction.LeaseRequest) writerRun {
				t.Helper()
				run := stagedWrite(func(ctx context.Context) (*Report, error) {
					return Lease(ctx, repo, historyActor, LeaseChoice{QueueID: fixture.QueueID, RequestID: id, Root: root, Lease: l}, WallClock())
				}, nil)
				if run.err != nil || run.rep == nil {
					t.Fatalf("%s: %v", id, run.err)
				}
				return run
			}
			claim := transaction.LeaseRequest{Verb: transaction.LeaseClaim, TicketID: target, Holder: "agent-1", LeaseMinutes: "60"}
			first := lease("claim-1", claim)
			if !first.completed() || len(first.rep.ReplayWarnings) != 0 {
				t.Fatalf("first claim: %+v warnings %q", first.rep.Outcome, first.rep.ReplayWarnings)
			}
			if r := lease("release-1", transaction.LeaseRequest{Verb: transaction.LeaseRelease, AttemptID: first.rep.AttemptID, Generation: first.rep.Generation}); !r.completed() {
				t.Fatalf("release: %+v", r.rep.Outcome)
			}
			if again := lease("claim-2", claim); !again.completed() || again.rep.Generation == first.rep.Generation {
				t.Fatalf("second claim: %+v generation %v", again.rep.Outcome, again.rep.Generation)
			}
			replay := lease("claim-1", claim)
			if !replay.rep.Outcome.Replayed {
				t.Fatalf("claim-1 again was not a replay: %+v", replay.rep.Outcome)
			}
			if w := replay.rep.ReplayWarnings; len(w) != 1 || !strings.HasPrefix(w[0], "advisory, not authoritative:") || !strings.Contains(w[0], "generation 1;") {
				t.Fatalf("replay warnings %q, want the one advisory naming generation 1", w)
			}
			served := map[string]bool{}
			for _, s := range replay.stages {
				name, _, _ := strings.Cut(s, "=")
				served[name] = true
			}
			if c.checkpoint {
				if !served["fast.lease.replay"] {
					t.Fatalf("writer-checkpoint store did not serve the replay from its route: %v", replay.stages)
				}
			} else if served["fast.lease.replay"] || !served["fast.complete.guards"] || served["fast.complete.apply"] {
				t.Fatalf("complete-route store did not replay on the complete route without applying: %v", replay.stages)
			}
		})
	}
}
