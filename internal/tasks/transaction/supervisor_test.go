package transaction

import (
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
	"testing"
)

func TestSupervisorCumulativeUsageUnknownIsSticky(t *testing.T) {
	usage := cumulativeTokens(snapshot.BudgetField{State: "NOT_OBSERVED"}, 0, true, true)
	if usage.State != "OBSERVED" || usage.Value == nil || usage.Value.Uint64() != 0 {
		t.Fatal("first zero observation lost")
	}
	usage = cumulativeTokens(usage, 7, true, false)
	if usage.State != "OBSERVED" || usage.Value.Uint64() != 7 {
		t.Fatal("known cumulative observation lost")
	}
	usage = cumulativeTokens(usage, 0, false, false)
	if usage.State != "NOT_OBSERVED" || usage.Value != nil {
		t.Fatal("missing turn retained false cumulative observation")
	}
	usage = cumulativeTokens(usage, 9, true, false)
	if usage.State != "NOT_OBSERVED" || usage.Value != nil {
		t.Fatal("later known turn erased unknown history")
	}
}

func TestSupervisorIntentRevisionFence(t *testing.T) {
	a := &snapshot.Attempt{TicketRevision: "1", PolicySha256: wire.Sum(nil)}
	rec := &ticket.Record{Status: "OPEN", AcceptanceRevision: "1"}
	if code := supervisedIntentCode(a, rec, nil); code != "" {
		t.Fatal(code)
	}
	rec.AcceptanceRevision = "2"
	if code := supervisedIntentCode(a, rec, nil); code != wire.CodeStaleTicket {
		t.Fatal(code)
	}
	rec.AcceptanceRevision = "1"
	if code := supervisedIntentCode(a, rec, []byte("changed")); code != wire.CodeStalePolicy {
		t.Fatal(code)
	}
}
