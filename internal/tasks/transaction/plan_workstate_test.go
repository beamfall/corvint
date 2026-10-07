package transaction

import (
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
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

// CAL-V0-105 with CAL-V0-101: a held OPEN pooled ticket leaves the window but
// still waits for its pool, so a later eligible ticket keeps yielding the last
// free member to it, exactly as the native explicit claim decides.
func TestCALV0105_HeldPooledTicketStillTakesPriorityYield(t *testing.T) {
	aa, hi, fr, lo := priorityScenario()
	f := newPriorityFixture(t, priorityPolicy(t, []string{"a", "b"}, "true"), []*ticket.Record{aa, hi, fr, lo})
	f.apply(t, planClaim(f.claim(aa.TicketID.Raw, "lanes"))) // one free member left
	held := map[string]bool{hi.TicketID.Raw: true}
	for _, pool := range []string{"lanes", ""} {
		in := f.planInput(pool)
		unheld := PriorityFirst(in)
		in.WorkStateHeld = held
		plan := PriorityFirst(in)
		yielded := false
		for i, e := range plan.Entries {
			id := e.Ticket.TicketID.Raw
			if id == hi.TicketID.Raw {
				if e.State != PlanDeferred || e.Reason != PlanReasonWorkStateHeld {
					t.Fatalf("pool %q: held %s = %s %s", pool, id, e.State, e.Reason)
				}
				continue
			}
			if e.State == PlanBlocked || (pool == "" && e.Ticket.RequiresPool == "") {
				continue
			}
			if u := unheld.Entries[i]; u.State != e.State || u.Reason != e.Reason || u.yieldTo != e.yieldTo {
				t.Fatalf("pool %q: holding %s changed %s from %s/%s/%q to %s/%s/%q", pool, hi.TicketID.Raw, id, u.State, u.Reason, u.yieldTo, e.State, e.Reason, e.yieldTo)
			}
			if got := yieldTarget(planClaim(f.claim(id, "lanes"))); got != e.yieldTo {
				t.Fatalf("pool %q: plan %s yieldTo %q, native explicit claim %q", pool, id, e.yieldTo, got)
			}
			yielded = yielded || (id == lo.TicketID.Raw && e.yieldTo == hi.TicketID.Raw)
		}
		if !yielded {
			t.Fatalf("pool %q: %s did not yield to the held %s: %+v", pool, lo.TicketID.Raw, hi.TicketID.Raw, plan.Entries)
		}
	}
	// The native claim-next has no work-state observation (a recorded
	// non-goal): it admits the held ticket, as the plan without a held set
	// does, while the dispatcher's held plan selects nothing on the pool.
	next := planClaimNext(f.claim("", "lanes"))
	if next.result != nil || attemptOf(t, next).TicketID.Raw != hi.TicketID.Raw {
		t.Fatalf("native claim-next %+v", next.result)
	}
	if c := PriorityFirst(f.planInput("lanes")).ClaimNext("lanes"); c == nil || c.Ticket.TicketID.Raw != hi.TicketID.Raw {
		t.Fatalf("unheld plan chose %v", c)
	}
	in := f.planInput("lanes")
	in.WorkStateHeld = held
	if c := PriorityFirst(in).ClaimNext("lanes"); c != nil {
		t.Fatalf("held plan chose %s on the pool", c.Ticket.TicketID.Raw)
	}
}

// CAL-V0-155: budget-held tickets leave the window the same way, with their
// own reason; a work-state hold names the ticket first.
func TestCALV0155_BudgetHeldTicketsLeaveWindow(t *testing.T) {
	in := resourcePlanInput(t, 14, 0, 4, 0, 0)
	starved := planStates(PriorityFirst(in))
	in.WorkStateHeld = map[string]bool{fixture.TicketID("MX-00"): true}
	in.BudgetHeld = map[string]bool{fixture.TicketID("MX-00"): true, fixture.TicketID("MX-01"): true}
	got := planStates(PriorityFirst(in))
	for id, want := range map[string][2]string{
		"MX-00": {PlanDeferred, PlanReasonWorkStateHeld},
		"MX-01": {PlanDeferred, PlanReasonBudgetHeld},
		"MX-02": {PlanSelected, starved["MX-00"][1]},
		"MX-03": {PlanSelected, starved["MX-00"][1]},
	} {
		if got[id] != want {
			t.Errorf("%s = %v, want %v", id, got[id], want)
		}
	}
}
