//go:build unix

package store_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// acquireStore is a gate store whose policy adds pool db with members a, b
// and a review-reserved member; members maps a member to its health exit
// code, when it has a health command.
func acquireStore(t *testing.T, health map[string]string) *leaseStore {
	t.Helper()
	gate := commandGate("verify", "printf ok", "30", true)
	s := newLeaseStore(t, gate)
	v := fixture.PolicyValue()
	v.Obj.Set("gates", wire.Array(gate))
	v.Obj.Set("policyVersion", str("3"))
	v.Obj.Set("capacity", obj("maxActiveAttempts", str("4"), "maxWorkersTotal", str("4"), "classes", wire.Array()))
	budgets, _ := v.Obj.Get("budgets")
	budgets.Obj.Set("requireEnforcedFields", wire.Strings(nil))
	pool := obj("id", str("db"), "members", wire.Strings([]string{"a", "b", "review"}), "reservedFor", obj("review", str("review")))
	if len(health) > 0 {
		config := wire.NewObject()
		for _, m := range []string{"a", "b", "review"} {
			if code, ok := health[m]; ok {
				config.Set(m, obj("health", obj("argv", wire.Strings([]string{"/bin/sh", "-c", "exit " + code}), "cwd", str("REPOSITORY"), "env", wire.Array(), "timeoutSeconds", str("3"))))
			}
		}
		pool.Obj.Set("memberConfig", wire.ObjectValue(config))
	}
	v.Obj.Set("pools", wire.Array(pool))
	rep, e := store.PolicyUpdate(context.Background(), s.repo, operator(), policyRequest("pools", "2", wire.EncodeFile(v)), now(t))
	if e != nil || rep.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("policy %+v %v", rep, e)
	}
	return s
}

func acquireOf(a *store.Report, pool string) transaction.LeaseRequest {
	return transaction.LeaseRequest{Verb: transaction.LeasePoolAcquire, AttemptID: a.AttemptID, Generation: a.Generation, Pool: pool}
}

func poolReleaseOf(a *store.Report, allocation wire.Digest) transaction.LeaseRequest {
	return transaction.LeaseRequest{Verb: transaction.LeasePoolRelease, AttemptID: a.AttemptID, Generation: a.Generation, Allocation: string(allocation)}
}

// poolEntries reads pools.json keyed by member.
func (s *leaseStore) poolEntries(t *testing.T) map[string]snapshot.PoolEntry {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(s.repo.StateDir, "pools.json"))
	if err != nil {
		t.Fatal(err)
	}
	state, err := snapshot.DecodePools(raw)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]snapshot.PoolEntry{}
	for _, en := range state.Entries {
		out[en.MemberID] = en
	}
	return out
}

// CAL-V0-197, CAL-V0-200, CAL-V0-201, CAL-V0-203: a live attempt claimed
// without a pool acquires one member, replays it, is fenced and refused a
// second member, returns the exact allocation into quarantine while staying
// live, cannot acquire again in that generation, and completes holding none.
func TestCALV0197_AcquireReleaseAndCompleteWithoutAllocation(t *testing.T) {
	s := acquireStore(t, nil)
	id := s.ticket(t, "one")
	claim, commit := s.submitted(t, id, "src", 0)
	if claim.PoolAllocation != nil {
		t.Fatalf("plain claim allocated %+v", claim.PoolAllocation)
	}
	got := s.lease(t, "acquire-1", acquireOf(claim, "db"), 2, nil)
	if got.Outcome.Outcome != mutation.OutcomeCompleted || got.PoolAllocation == nil || got.PoolAllocation.MemberID != "a" || got.Ticket != id {
		t.Fatalf("acquire %+v", got)
	}
	a := s.attempt(t, claim.AttemptID)
	if a.Phase != "BUILT" || a.PoolAllocation == nil || a.PoolAllocation.AllocationID != got.PoolAllocation.AllocationID {
		t.Fatalf("attempt %+v", a)
	}
	if en := s.poolEntries(t)["a"]; en.State != "ALLOCATED" || en.AttemptID != claim.AttemptID || en.Generation != claim.Generation || en.Holder != "agent-1" {
		t.Fatalf("entry %+v", en)
	}
	// CAL-V0-203: a replay returns the receipt-bound allocation.
	if again := s.lease(t, "acquire-1", acquireOf(claim, "db"), 3, nil); again.Kind != "Replay" || again.PoolAllocation == nil || *again.PoolAllocation != *got.PoolAllocation {
		t.Fatalf("replay %+v", again)
	}
	before := storeDigest(t, s.repo)
	held := s.lease(t, "acquire-2", acquireOf(claim, "db"), 3, nil)
	refusedWith(t, held, mutation.OutcomeBlocked, wire.CodeResourceCollision)
	if held.Ticket != id || held.PoolAllocation != nil {
		t.Fatalf("refused acquire must name the ticket and no allocation: %+v", held)
	}
	if storeDigest(t, s.repo) != before {
		t.Fatal("refused acquire wrote")
	}
	stale := acquireOf(claim, "db")
	stale.Generation = wire.SizeOf(claim.Generation.Uint64() + 1)
	refusedWith(t, s.lease(t, "acquire-stale", stale, 3, nil), mutation.OutcomeRevisionConflict, wire.CodeFenced)

	before = storeDigest(t, s.repo)
	refusedWith(t, s.lease(t, "release-wrong", poolReleaseOf(claim, wire.Sum([]byte("other"))), 3, nil), mutation.OutcomeRevisionConflict, wire.CodeFenced)
	if storeDigest(t, s.repo) != before {
		t.Fatal("mismatched pool release wrote")
	}
	ret := s.lease(t, "release-1", poolReleaseOf(claim, got.PoolAllocation.AllocationID), 3, nil)
	if ret.Outcome.Outcome != mutation.OutcomeCompleted || ret.ReleasedPoolAllocation == nil || ret.ReleasedPoolAllocation.Allocation != *got.PoolAllocation || ret.PoolAllocation != nil {
		t.Fatalf("pool release %+v", ret)
	}
	a = s.attempt(t, claim.AttemptID)
	if a.Phase != "BUILT" || a.Lease == nil || a.PoolAllocation != nil || a.ReleasedPoolAllocation == nil || a.ReleasedPoolAllocation.Allocation.AllocationID != got.PoolAllocation.AllocationID {
		t.Fatalf("attempt after pool release %+v", a)
	}
	if en := s.poolEntries(t)["a"]; en.State != "QUARANTINED" || en.AttemptID != claim.AttemptID {
		t.Fatalf("quarantine %+v", en)
	}
	if again := s.lease(t, "release-1", poolReleaseOf(claim, got.PoolAllocation.AllocationID), 4, nil); again.Kind != "Replay" || again.ReleasedPoolAllocation == nil || *again.ReleasedPoolAllocation != *ret.ReleasedPoolAllocation {
		t.Fatalf("release replay %+v", again)
	}
	// One allocation per generation.
	refusedWith(t, s.lease(t, "acquire-3", acquireOf(claim, "db"), 4, nil), mutation.OutcomeBlocked, wire.CodeResourceCollision)
	if r := s.lease(t, "renew-1", renewOf(claim), 4, nil); r.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("renew %+v", r)
	}
	s.passes(t, "gate-1", gateOf(claim, "verify"), 5)
	if r := s.lease(t, "complete-1", completeOf(claim, commit), 6, nil); r.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("complete %+v", r)
	}
	if a = s.attempt(t, claim.AttemptID); a.Phase != "COMPLETED" || a.ReleasedPoolAllocation == nil {
		t.Fatalf("completed %+v", a)
	}
	if en := s.poolEntries(t); len(en) != 1 || en["a"].State != "QUARANTINED" {
		t.Fatalf("entries after complete %+v", en)
	}
	auditOK(t, s.repo)
}

// CAL-V0-201: completing, releasing or reaping an attempt that holds an
// acquired allocation quarantines it exactly as a claimed one.
func TestCALV0201_EndingQuarantinesAcquiredAllocation(t *testing.T) {
	for _, end := range []string{"complete", "release", "reap"} {
		t.Run(end, func(t *testing.T) {
			s := acquireStore(t, nil)
			claim, commit := s.submitted(t, s.ticket(t, "one"), "src", 0)
			got := s.lease(t, "acquire-1", acquireOf(claim, "db"), 2, nil)
			if got.PoolAllocation == nil {
				t.Fatalf("acquire %+v", got)
			}
			var r *store.Report
			if end == "complete" {
				s.passes(t, "gate-1", gateOf(claim, "verify"), 3)
				r = s.lease(t, "end", completeOf(claim, commit), 4, nil)
			} else if end == "release" {
				r = s.lease(t, "end", releaseOf(claim), 4, nil)
			} else {
				r = s.lease(t, "end", transaction.LeaseRequest{Verb: transaction.LeaseReap, AttemptID: claim.AttemptID, Generation: claim.Generation}, 90, store.NoScopeDeriver)
			}
			if r.Outcome.Outcome != mutation.OutcomeCompleted {
				t.Fatalf("%s %+v", end, r)
			}
			en := s.poolEntries(t)[got.PoolAllocation.MemberID]
			if en.State != "QUARANTINED" || en.AllocationID != got.PoolAllocation.AllocationID || en.AttemptID != claim.AttemptID {
				t.Fatalf("entry %+v", en)
			}
			auditOK(t, s.repo)
		})
	}
}

// CAL-V0-198: acquire runs the pooled-claim health preparation for the
// attempt's holder and stage, skipping a failing member.
func TestCALV0198_AcquirePreparesHealth(t *testing.T) {
	s := acquireStore(t, map[string]string{"a": "1", "b": "0"})
	claim := s.claim(t, "claim-1", s.ticket(t, "one"), 0, "src/")
	got := s.lease(t, "acquire-1", acquireOf(claim, "db"), 1, nil)
	if got.Outcome.Outcome != mutation.OutcomeCompleted || got.PoolAllocation == nil || got.PoolAllocation.MemberID != "b" {
		t.Fatalf("acquire %+v", got)
	}
	entries := s.poolEntries(t)
	if entries["a"].State != "QUARANTINED" || entries["b"].State != "ALLOCATED" || entries["b"].AttemptID != claim.AttemptID || entries["b"].Stage != "" || entries["b"].Holder != "agent-1" {
		t.Fatalf("entries %+v", entries)
	}
	if again := s.lease(t, "acquire-1", acquireOf(claim, "db"), 2, nil); again.Kind != "Replay" || again.PoolAllocation == nil || *again.PoolAllocation != *got.PoolAllocation {
		t.Fatalf("replay %+v", again)
	}
	auditOK(t, s.repo)
}

// CAL-V0-199, CAL-V0-202: a review attempt acquiring with author exclusion
// skips the member that implemented the ticket, including one an implement
// attempt acquired and returned early; an implement attempt cannot ask.
func TestCALV0199_AcquireExcludesAuthors(t *testing.T) {
	s := acquireStore(t, nil)
	id := s.ticket(t, "one")
	implement := claimOf(id, "src/")
	implement.Stage = "implement"
	impl := s.lease(t, "claim-impl", implement, 0, nil)
	bad := acquireOf(impl, "db")
	bad.ExcludeAuthors = transaction.ExcludeAuthorsLatest
	if r := s.lease(t, "acquire-bad", bad, 1, nil); r.Outcome.HasCode(wire.CodeResourceCollision) || r.Outcome.Outcome == mutation.OutcomeCompleted {
		t.Fatalf("implement author exclusion %+v", r)
	}
	got := s.lease(t, "acquire-impl", acquireOf(impl, "db"), 1, nil)
	if got.PoolAllocation == nil || got.PoolAllocation.MemberID != "a" {
		t.Fatalf("implement acquire %+v", got)
	}
	if r := s.lease(t, "return-impl", poolReleaseOf(impl, got.PoolAllocation.AllocationID), 2, nil); r.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("pool release %+v", r)
	}
	if r := s.lease(t, "release-impl", releaseOf(impl), 3, nil); r.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("release %+v", r)
	}
	safe := s.lease(t, "safe-a", transaction.LeaseRequest{Verb: transaction.LeasePoolSafe, Member: "a", Allocation: string(got.PoolAllocation.AllocationID), Evidence: "operator-reset-record", Reason: "fixture reset"}, 4, nil)
	if safe.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("confirm-safe %+v", safe)
	}
	review := claimOf(id, "src/")
	review.Holder, review.Stage = "reviewer", "review"
	rclaim := s.lease(t, "claim-review", review, 5, nil)
	if rclaim.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("review claim %+v", rclaim)
	}
	l := acquireOf(rclaim, "db")
	l.ExcludeMembers = []string{"review"}
	l.ExcludeAuthors = transaction.ExcludeAuthorsLatest
	r := s.lease(t, "acquire-review", l, 6, nil)
	if r.Outcome.Outcome != mutation.OutcomeCompleted || r.PoolAllocation == nil || r.PoolAllocation.MemberID != "b" {
		t.Fatalf("review acquire %+v", r)
	}
	if en := s.poolEntries(t)["b"]; en.Holder != "reviewer" || en.Stage != "review" {
		t.Fatalf("review entry %+v", en)
	}
	auditOK(t, s.repo)
}

// CAL-V0-198, CAL-V0-200: an admission barrier pauses acquire but not the
// early return of an allocation.
func TestCALV0198_BarrierPausesAcquireNotRelease(t *testing.T) {
	s := acquireStore(t, nil)
	one, two := s.ticket(t, "one"), s.ticket(t, "two")
	first := s.claim(t, "claim-1", one, 0, "src/")
	second := s.claim(t, "claim-2", two, 0, "lib/")
	got := s.lease(t, "acquire-1", acquireOf(first, "db"), 1, nil)
	if got.PoolAllocation == nil {
		t.Fatalf("acquire %+v", got)
	}
	if r, err := store.Barrier(context.Background(), s.repo, operator(), barrierRequest(transaction.Pause, "pause"), s.at(t, 1)); err != nil || r.Receipt == "" {
		t.Fatalf("pause %+v %v", r, err)
	}
	before := storeDigest(t, s.repo)
	refusedWith(t, s.lease(t, "acquire-2", acquireOf(second, "db"), 2, nil), mutation.OutcomeBlocked, wire.CodePaused)
	if storeDigest(t, s.repo) != before {
		t.Fatal("paused acquire wrote")
	}
	if r := s.lease(t, "release-1", poolReleaseOf(first, got.PoolAllocation.AllocationID), 2, nil); r.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("pool release under barrier %+v", r)
	}
	auditOK(t, s.repo)
}
