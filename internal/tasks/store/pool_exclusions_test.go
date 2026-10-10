package store_test

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func exclusionPolicy(t *testing.T, s *leaseStore, config wire.Value) wire.Value {
	t.Helper()
	v := fixture.PolicyValue()
	v.Obj.Set("policyVersion", str("3"))
	v.Obj.Set("capacity", obj("maxActiveAttempts", str("4"), "maxWorkersTotal", str("4"), "classes", wire.Array()))
	b, _ := v.Obj.Get("budgets")
	b.Obj.Set("requireEnforcedFields", wire.Array())
	pool := obj("id", str("db"), "members", wire.Strings([]string{"a", "b", "review"}), "reservedFor", obj("review", str("review")))
	if config.Kind != wire.KindNull {
		pool.Obj.Set("memberConfig", config)
	}
	v.Obj.Set("pools", wire.Array(pool))
	r, err := store.PolicyUpdate(context.Background(), s.repo, operator(), policyRequest("exclusion-policy", "2", wire.EncodeFile(v)), now(t))
	if err != nil || r.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("policy %+v %v", r, err)
	}
	s.t0 = now(t)
	return v
}

// CAL-V0-065: no excluded health command runs, even in a reserved-first
// tier or after a failed unreserved member causes another selection round.
func TestCALV0065_HealthFiltersEveryRound(t *testing.T) {
	t.Parallel()
	s := newLeaseStore(t)
	markers := t.TempDir()
	command := func(argv ...string) wire.Value {
		return obj("argv", wire.Strings(argv), "cwd", str("REPOSITORY"), "env", wire.Array(), "timeoutSeconds", str("3"))
	}
	forbidden := filepath.Join(markers, "excluded")
	allowed := filepath.Join(markers, "allowed")
	exclusionPolicy(t, s, obj("review", obj("health", command("/usr/bin/touch", forbidden)), "a", obj("health", command("/bin/false")), "b", obj("health", command("/usr/bin/touch", allowed))))
	id := s.ticket(t, "health-exclusion")
	c := claimOf(id, "health")
	c.Pool, c.Stage = "db", "review"
	c.ExcludeMembers = []string{"review"}
	before := fixture.TreeSnapshot(t, s.repo.StateDir)
	bad := c
	bad.ExcludeMembers = []string{"foreign"}
	r, err := store.Lease(context.Background(), s.repo, operator(), store.LeaseChoice{QueueID: fixture.QueueID, RequestID: "foreign", Root: s.root, Lease: bad}, s.at(t, 0))
	if err == nil && r.Outcome.Outcome == mutation.OutcomeCompleted {
		t.Fatalf("foreign member admitted %+v", r)
	}
	if !reflect.DeepEqual(before, fixture.TreeSnapshot(t, s.repo.StateDir)) {
		t.Fatal("foreign exclusion prepared/wrote state")
	}
	a := s.lease(t, "health-filter", c, 0, nil)
	if a.PoolAllocation == nil || a.PoolAllocation.MemberID != "b" {
		t.Fatalf("health claim %+v", a)
	}
	if _, err := os.Stat(forbidden); !os.IsNotExist(err) {
		t.Fatalf("excluded health ran: %v", err)
	}
	if _, err := os.Stat(allowed); err != nil {
		t.Fatalf("allowed health did not run: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(s.repo.StateDir, "pools.json"))
	if err != nil {
		t.Fatal(err)
	}
	state, err := snapshot.DecodePools(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Entries) != 2 {
		t.Fatalf("unexpected pool mutations %+v", state)
	}
	for _, en := range state.Entries {
		if en.MemberID == "review" || en.MemberID == "a" && en.State != "QUARANTINED" {
			t.Fatalf("bad health state %+v", en)
		}
	}
}

// CAL-V0-065: authoritative replay binds the original allocation across a
// released generation, a successor, and removal of an excluded FREE member.
func TestCALV0065_ReplayAfterSuccessorAndPolicyChange(t *testing.T) {
	t.Parallel()
	for _, verb := range []string{transaction.LeaseClaim, transaction.LeaseClaimNext} {
		t.Run(verb, func(t *testing.T) {
			s := newLeaseStore(t)
			v := exclusionPolicy(t, s, wire.Null())
			id := s.ticket(t, "replay-exclusion")
			c := claimOf(id, "one")
			c.Pool, c.Stage = "db", "implement"
			c.ExcludeMembers = []string{"a"}
			if verb == transaction.LeaseClaimNext {
				c.Verb = verb
				c.TicketID = ""
			}
			original := s.lease(t, "original", c, 0, nil)
			if original.PoolAllocation == nil || original.PoolAllocation.MemberID != "b" {
				t.Fatalf("claim %+v", original)
			}
			s.lease(t, "release-original", transaction.LeaseRequest{Verb: transaction.LeaseRelease, AttemptID: original.AttemptID, Generation: original.Generation}, 0, nil)
			s.lease(t, "confirm-original", transaction.LeaseRequest{Verb: transaction.LeasePoolSafe, Member: "b", Allocation: string(original.PoolAllocation.AllocationID), Evidence: "local-reset", Reason: "operator confirmed reset"}, 0, nil)
			successor := s.lease(t, "successor", c, 0, nil)
			if successor.Generation == original.Generation || successor.PoolAllocation == nil || successor.PoolAllocation.AllocationID == original.PoolAllocation.AllocationID {
				t.Fatalf("successor %+v", successor)
			}
			v.Obj.Set("policyVersion", str("4"))
			pools, _ := v.Obj.Get("pools")
			pools.Arr[0].Obj.Set("members", wire.Strings([]string{"b", "review"}))
			update, err := store.PolicyUpdate(context.Background(), s.repo, operator(), policyRequest("remove-free-excluded", "3", wire.EncodeFile(v)), now(t))
			if err != nil || update.Outcome.Outcome != mutation.OutcomeCompleted {
				t.Fatalf("policy change %+v %v", update, err)
			}
			s.t0 = now(t)
			replay := s.lease(t, "original", c, 0, nil)
			if replay.Kind != "Replay" || replay.Generation != original.Generation || !reflect.DeepEqual(replay.PoolAllocation, original.PoolAllocation) {
				t.Fatalf("replay %+v original %+v", replay, original)
			}
			changed := c
			changed.ExcludeMembers = []string{"a", "b"}
			conflict := s.lease(t, "original", changed, 0, nil)
			if !conflict.Outcome.HasCode(wire.CodeRequestIDConflict) {
				t.Fatalf("policy eligibility outranked conflict %+v", conflict)
			}
		})
	}
}

// CAL-V0-065: both the outside-lock scope deriver and final CLAIM_NEXT
// selector receive exclusions, including complete capacity exhaustion.
func TestCALV0065_ClaimNextSelectors(t *testing.T) {
	t.Parallel()
	s := newLeaseStore(t)
	exclusionPolicy(t, s, wire.Null())
	want := s.ticket(t, "next-exclusion")
	c := transaction.LeaseRequest{Verb: transaction.LeaseClaimNext, Holder: "builder", LeaseMinutes: "60", Pool: "db", Stage: "review", ExcludeMembers: []string{"a", "review"}}
	derived := 0
	derive := func(_ context.Context, _, _, title, _ string) ([]string, string, bool) {
		derived++
		if title != "next-exclusion" {
			t.Errorf("wrong derived ticket %s", title)
		}
		return []string{"next"}, string(wire.Sum([]byte("derivation"))), true
	}
	a := s.lease(t, "next-filter", c, 0, derive)
	if a.Ticket != want || a.PoolAllocation == nil || a.PoolAllocation.MemberID != "b" || derived == 0 {
		t.Fatalf("next %+v derived %d", a, derived)
	}
	s.ticket(t, "next-exhausted")
	c.ExcludeMembers = []string{"a", "b", "review"}
	blocked := s.lease(t, "next-all-excluded", c, 0, derive)
	if !blocked.Outcome.HasCode(wire.CodeResourceCollision) {
		t.Fatalf("all-excluded next %+v", blocked)
	}
}
