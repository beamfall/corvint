package store

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
)

// TestTRMV0533_LeaseClaimNeedsOwnerOrOperator (V1-3560): a claim by a WORKER,
// REVIEWER, IMPORTER or SYSTEM binding is refused UNAUTHORIZED and commits
// nothing, so the same ticket's first OPERATOR claim is still generation 1;
// lane claims run under the CLI's OPERATOR binding, never a WORKER one.
func TestTRMV0533_LeaseClaimNeedsOwnerOrOperator(t *testing.T) {
	repo := historyStore(t, 70)
	holdLeasePolicy(t, repo)
	root := filepath.Join(filepath.Dir(repo.PrimaryWorktree), "worktree")
	holdGit(t, root, "init", "-q", "-b", "main")
	holdGit(t, root, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "--allow-empty", "-m", "base")
	_, ticketFile := mutationBoundaryFiles(t, repo)
	target := fixture.TicketID(strings.TrimSuffix(filepath.Base(ticketFile), ".json"))
	claim := transaction.LeaseRequest{Verb: transaction.LeaseClaim, TicketID: target, Holder: "agent-1", LeaseMinutes: "60"}
	for _, role := range []string{"WORKER", "REVIEWER", "IMPORTER", "SYSTEM"} {
		rep, err := Lease(context.Background(), repo, mutation.Binding{ID: "agent-1", Role: role}, LeaseChoice{QueueID: fixture.QueueID, RequestID: "claim-" + strings.ToLower(role), Root: root, Lease: claim}, WallClock())
		if err != nil || rep == nil || rep.Outcome.Outcome != mutation.OutcomeUnauthorized {
			t.Fatalf("%s claim: want UNAUTHORIZED, got %+v %v", role, rep, err)
		}
	}
	rep, err := Lease(context.Background(), repo, mutation.Binding{ID: "agent-1", Role: "OPERATOR"}, LeaseChoice{QueueID: fixture.QueueID, RequestID: "claim-operator", Root: root, Lease: claim}, WallClock())
	if err != nil || rep == nil || rep.Outcome.Outcome != mutation.OutcomeCompleted || rep.Generation.Uint64() != 1 {
		t.Fatalf("operator claim after refused roles: %+v %v", rep, err)
	}
}
