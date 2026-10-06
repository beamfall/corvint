package transaction

import (
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// planStates maps each planned ticket's local ID to its state and reason.
func planStates(plan TicketPlan) map[string][2]string {
	out := map[string][2]string{}
	for _, e := range plan.Entries {
		out[e.Ticket.TicketID.Local] = [2]string{e.State, e.Reason}
	}
	return out
}

// CAL-V0-105, issue 624: tickets the program's work state holds occupied the
// selection window, starving actionable tickets with LIMIT_EXCEEDED.
func TestCALV0105_WorkStateHeldTicketsLeaveWindow(t *testing.T) {
	// 14 live reservations leave two of the 16 slots; MX-00 and MX-01 are held.
	in := resourcePlanInput(t, 14, 0, 4, 0, 0)
	starved := planStates(PriorityFirst(in))
	for _, id := range []string{"MX-02", "MX-03"} {
		if starved[id] != [2]string{PlanDeferred, wire.CodeLimitExceeded} {
			t.Fatalf("without a work-state observation %s = %v, want the LIMIT_EXCEEDED starvation", id, starved[id])
		}
	}
	in.WorkStateHeld = map[string]bool{fixture.TicketID("MX-00"): true, fixture.TicketID("MX-01"): true}
	got := planStates(PriorityFirst(in))
	for id, want := range map[string][2]string{
		"MX-00": {PlanDeferred, PlanReasonWorkStateHeld},
		"MX-01": {PlanDeferred, PlanReasonWorkStateHeld},
		"MX-02": {PlanSelected, starved["MX-00"][1]},
		"MX-03": {PlanSelected, starved["MX-00"][1]},
	} {
		if got[id] != want {
			t.Errorf("%s = %v, want %v", id, got[id], want)
		}
	}
	// Deterministic for one snapshot and one held set.
	if again := planStates(PriorityFirst(in)); len(again) != len(got) || again["MX-02"] != got["MX-02"] || again["MX-00"] != got["MX-00"] {
		t.Fatalf("replanning changed the result: %v vs %v", again, got)
	}
}

// CAL-V0-105: a held ticket never outranks a BLOCKED decision, and a held set
// naming no ticket leaves the plan unchanged.
func TestCALV0105_HeldSetDoesNotOverrideBlockedOrUnheld(t *testing.T) {
	in := resourcePlanInput(t, 14, 0, 4, 0, 0)
	base := planStates(PriorityFirst(in))
	in.WorkStateHeld = map[string]bool{fixture.TicketID("ZZ-99"): true}
	if got := planStates(PriorityFirst(in)); len(got) != len(base) || got["MX-00"] != base["MX-00"] || got["MX-03"] != base["MX-03"] {
		t.Fatalf("unrelated held set changed the plan: %v vs %v", got, base)
	}
	rec := mustGet(t, in, "MX-00")
	rec.Status = "HELD"
	in.WorkStateHeld = map[string]bool{rec.TicketID.Raw: true}
	if got := planStates(PriorityFirst(in)); got["MX-00"][0] != PlanBlocked {
		t.Fatalf("held ticket record = %v, want BLOCKED", got["MX-00"])
	}
}
