package transaction

import (
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/snapshot"
)

// CAL-V0-096: an ended external-agent generation copies its stage and pool
// member; a supervised generation spans stages and releases its allocation per
// stage, so no single stage or member is recorded for it.
func TestCALV0096_EndedHistoryCopiesOnlyExact(t *testing.T) {
	t.Run("CAL-V0-096 EndedHistoryCopiesOnlyExact", func(t *testing.T) {
		alloc := &snapshot.PoolAllocation{PoolID: "db", MemberID: "b"}
		if h := endedHistory(&snapshot.Attempt{RuntimeID: "supervisor", Stage: "review", PoolAllocation: alloc}); h != nil {
			t.Fatalf("supervised history guessed: %+v", h)
		}
		h := endedHistory(&snapshot.Attempt{RuntimeID: snapshot.RuntimeExternalAgent, Stage: "review", PoolAllocation: alloc})
		if h == nil || h.Stage == nil || *h.Stage != "review" || h.PoolID == nil || *h.PoolID != "db" || h.MemberID == nil || *h.MemberID != "b" {
			t.Fatalf("external history %+v", h)
		}
		alloc.MemberID = "a"
		if *h.MemberID != "b" {
			t.Fatal("history aliases the live allocation")
		}
		if h := endedHistory(&snapshot.Attempt{RuntimeID: snapshot.RuntimeExternalAgent}); h == nil || h.Stage != nil || h.PoolID != nil || h.MemberID != nil {
			t.Fatalf("unpooled history must be recorded null: %+v", h)
		}
	})
}
