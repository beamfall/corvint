package store

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// poolLeasePolicy is holdLeasePolicy with one pool "db" of two members.
func poolLeasePolicy(t *testing.T, repo *intent.Repository) {
	t.Helper()
	v := fixture.PolicyValue()
	v.Obj.Set("policyVersion", wire.String("2"))
	v.Obj.Set("capacity", historyObject("maxActiveAttempts", wire.String("4"), "maxWorkersTotal", wire.String("4"), "classes", wire.Array()))
	budgets, _ := v.Obj.Get("budgets")
	budgets.Obj.Set("requireEnforcedFields", wire.Strings(nil))
	v.Obj.Set("pools", wire.Array(historyObject("id", wire.String("db"), "members", wire.Strings([]string{"a", "b"}))))
	report, err := PolicyUpdate(context.Background(), repo, historyActor, PolicyRequest{QueueID: fixture.QueueID, RequestID: "pool-policy", ExpectedPolicyVersion: "1", Policy: wire.EncodeFile(v)}, WallClock())
	if err != nil || report.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("policy: %+v %v", report, err)
	}
}

// CAL-V0-205 (V1-1045, decision 0471): `pool confirm-safe` is served by the
// writer-checkpoint route, so it never runs the complete-history audit while
// the store holds a bound writer checkpoint. A committed confirmation writes
// the same bytes and report as the complete route; a confirmation the model
// refuses is answered as the complete route answers it; and with the
// checkpoint removed the complete route still decides it.
func TestCALV0205_PoolConfirmSafeTakesWriterRoute(t *testing.T) {
	repo := writerStore(t, 70)
	poolLeasePolicy(t, repo)
	if run := writerMutate(t, repo, "pool-reseed", nil); !run.completed() {
		t.Fatalf("reseed: %+v %v", run.rep, run.err)
	}
	root := filepath.Join(filepath.Dir(repo.PrimaryWorktree), "worktree")
	holdGit(t, root, "init", "-q", "-b", "main")
	holdGit(t, root, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "--allow-empty", "-m", "base")
	now := WallClock()
	created := stagedWrite(writerCreate(repo, "pool-ticket", now), nil)
	if !created.completed() {
		t.Fatalf("create: %+v %v", created.rep, created.err)
	}
	lease := func(id string, l transaction.LeaseRequest) func(context.Context) (*Report, error) {
		return func(ctx context.Context) (*Report, error) {
			return Lease(ctx, repo, historyActor, LeaseChoice{QueueID: fixture.QueueID, RequestID: id, Root: root, Lease: l}, now)
		}
	}
	claim := stagedWrite(lease("pool-claim", transaction.LeaseRequest{Verb: transaction.LeaseClaim, TicketID: created.rep.Ticket, Holder: "agent-1", LeaseMinutes: "60", Pool: "db", Stage: "implement"}), nil)
	if !claim.completed() || claim.rep.PoolAllocation == nil {
		t.Fatalf("claim: %+v %v %v", claim.rep, claim.err, claim.stages)
	}
	release := stagedWrite(lease("pool-release", transaction.LeaseRequest{Verb: transaction.LeaseRelease, AttemptID: claim.rep.AttemptID, Generation: claim.rep.Generation}), nil)
	if !release.completed() {
		t.Fatalf("release: %+v %v", release.rep, release.err)
	}
	alloc := claim.rep.PoolAllocation
	safe := func(id, allocation string) func(context.Context) (*Report, error) {
		return lease(id, transaction.LeaseRequest{Verb: transaction.LeasePoolSafe, Member: alloc.MemberID, Allocation: allocation, Evidence: "operator-reset-record", Reason: "confirmed external reset"})
	}
	// writerParity requires the writer route for the write and the
	// complete route for its oracle, with identical bytes and reports.
	writerParity(t, repo, safe("pool-safe", string(alloc.AllocationID)))
	// The member is now free: a second confirmation is refused, answered by
	// the writer route as the complete route answers it.
	refused := safe("pool-safe-again", string(alloc.AllocationID))
	run := stagedWrite(refused, nil)
	if run.declined() != "" || (run.servedWithoutWrite() == "" && !run.fast()) {
		t.Fatalf("writer route did not serve the refusal: %+v %v %v", run.rep, run.err, run.stages)
	}
	if run.err == nil && run.rep.Outcome.Outcome == mutation.OutcomeCompleted {
		t.Fatalf("second confirmation admitted: %+v", run.rep)
	}
	sameDecision(t, run, writerOracle(t, repo, safe("pool-safe-oracle", string(alloc.AllocationID))))
}
