package store_test

import (
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func historyText(s *string) string {
	if s == nil {
		return "<null>"
	}
	return *s
}

// CAL-V0-079: a retry records the stage and pool member the ended
// external-agent generation held, and null when it held none.
func TestCALV0079_PriorGenerationRecordsStageAndMember(t *testing.T) {
	t.Run("CAL-V0-079 PriorGenerationRecordsStageAndMember", func(t *testing.T) {
		s := newLeaseStore(t)
		exclusionPolicy(t, s, wire.Null())
		pooled := s.ticket(t, "pooled")
		c := claimOf(pooled, "one")
		c.Pool, c.Stage = "db", "implement"
		c.ExcludeMembers = []string{"a"}
		first := s.lease(t, "pooled-1", c, 0, nil)
		if first.PoolAllocation == nil || first.PoolAllocation.MemberID != "b" {
			t.Fatalf("claim %+v", first)
		}
		s.lease(t, "pooled-release", transaction.LeaseRequest{Verb: transaction.LeaseRelease, AttemptID: first.AttemptID, Generation: first.Generation}, 0, nil)
		s.lease(t, "pooled-safe", transaction.LeaseRequest{Verb: transaction.LeasePoolSafe, Member: "b", Allocation: string(first.PoolAllocation.AllocationID), Evidence: "local-reset", Reason: "operator confirmed reset"}, 0, nil)
		c.ExcludeMembers = []string{"b"}
		second := s.lease(t, "pooled-2", c, 0, nil)
		if second.AttemptID != first.AttemptID || second.PoolAllocation == nil || second.PoolAllocation.MemberID != "a" {
			t.Fatalf("retry %+v", second)
		}
		a := s.attempt(t, second.AttemptID)
		if len(a.PriorGenerations) != 1 || a.PriorGenerations[0].Generation != first.Generation {
			t.Fatalf("prior %+v", a.PriorGenerations)
		}
		h := a.PriorGenerations[0].History
		if h == nil || historyText(h.Stage) != "implement" || historyText(h.PoolID) != "db" || historyText(h.MemberID) != "b" {
			t.Fatalf("pooled history %+v", h)
		}

		plain := s.ticket(t, "plain")
		one := s.claim(t, "plain-1", plain, 1, "src")
		s.lease(t, "plain-release", releaseOf(one), 1, nil)
		two := s.claim(t, "plain-2", plain, 2, "src")
		b := s.attempt(t, two.AttemptID)
		if len(b.PriorGenerations) != 1 {
			t.Fatalf("prior %+v", b.PriorGenerations)
		}
		if h := b.PriorGenerations[0].History; h == nil || h.Stage != nil || h.PoolID != nil || h.MemberID != nil {
			t.Fatalf("unpooled history must be recorded null, got %+v", h)
		}
		auditOK(t, s.repo)
	})
}
