package transaction

import (
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestCALV0099_ClaimAndPlanAreStageScoped: plan, claim and claim-next for a
// listed stage report PREREQUISITE_UNSATISFIED naming the prerequisite; an
// unlisted stage is unaffected; a stageless read fails closed.
func TestCALV0099_ClaimAndPlanAreStageScoped(t *testing.T) {
	q, err := intent.DecodeQueue(fixture.QueueBytes())
	if err != nil {
		t.Fatal(err)
	}
	policy, err := intent.DecodePolicy(fixture.PolicyBytes())
	if err != nil {
		t.Fatal(err)
	}
	policy.RequireEnforcedFields = nil
	rec, pre := fixture.Ticket("AT-01"), fixture.Ticket("AT-02")
	pre.Status = ticket.StatusHeld
	pre.Holds = []ticket.Hold{{HoldID: "hold", Actor: "owner", Reason: "hold", PlacedAt: timestamp}}
	rec.ExecutionPrerequisites = []ticket.Prerequisite{{TicketID: pre.TicketID, Obligation: "COMPLETED", Stages: []string{"integrate", "review"}}}
	inv, err := ticket.NewInventory(q.QueueID, []*ticket.Record{rec, pre})
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		stage   string
		blocked bool
	}{{"review", true}, {"integrate", true}, {"", true}, {"implement", false}} {
		in := PlanInput{Queue: q, Policy: policy, Tickets: inv, Stage: c.stage, Attempts: map[string]*snapshot.Attempt{}, Reservations: &snapshot.ReservationSet{QueueID: q.QueueID}}
		e := planEntry(in, rec)
		got, reason := RecordedClaimability(in, rec)
		if c.blocked {
			if e.State != PlanBlocked || e.Reason != wire.CodePrerequisiteUnsatisfied || !contains(e.Blockers, pre.TicketID.Raw) {
				t.Fatalf("plan stage %q: %+v", c.stage, e)
			}
			if got.Kind != wire.KindBool || got.Bool || reason != wire.CodePrerequisiteUnsatisfied {
				t.Fatalf("claimability stage %q: %s %s", c.stage, wire.Encode(got), reason)
			}
		} else if e.State == PlanBlocked || reason == wire.CodePrerequisiteUnsatisfied {
			t.Fatalf("unlisted stage %q blocked: %+v %s", c.stage, e, reason)
		}
		// claim and claim-next share leaseContext.eligibility.
		lc := leaseContext{r: admin(Lease, "claim"), l: &LeaseRequest{Verb: LeaseClaim, TicketID: rec.TicketID.Raw, Holder: "builder", LeaseMinutes: "60", Stage: c.stage}, st: inputState{queue: q, policy: policy, tickets: inv, reservations: &snapshot.ReservationSet{QueueID: q.QueueID}}}
		out := lc.eligibility(rec.TicketID.Raw)
		if c.blocked {
			if out == nil || out.result.Outcome.Codes[0] != wire.CodePrerequisiteUnsatisfied || !strings.Contains(out.result.Detail, "AT-02") {
				t.Fatalf("claim stage %q: %+v", c.stage, out)
			}
		} else if out != nil {
			t.Fatalf("claim of unlisted stage %q refused: %+v", c.stage, out.result)
		}
	}
}

func contains(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

// TestCALV0099_ClaimNextRefusalNamesPrerequisite: when nothing is
// selectable, the claim-next refusal names the unsatisfied prerequisite and
// its stages, not only the candidate and reason.
func TestCALV0099_ClaimNextRefusalNamesPrerequisite(t *testing.T) {
	rec, pre := fixture.Ticket("AT-01"), fixture.Ticket("AT-02")
	pre.Status = ticket.StatusHeld
	pre.Holds = []ticket.Hold{{HoldID: "hold", Actor: "owner", Reason: "hold", PlacedAt: timestamp}}
	rec.ExecutionPrerequisites = []ticket.Prerequisite{{TicketID: pre.TicketID, Obligation: "COMPLETED", Stages: []string{"integrate"}}}
	c := issue502ClaimContext(t, LeaseClaimNext, "", rec, pre)
	c.l.Stage = "integrate"
	code, detail := issue502Refused(planClaimNext(c))
	if code != wire.CodePrerequisiteUnsatisfied || !strings.Contains(detail, rec.TicketID.Raw+", is BLOCKED "+wire.CodePrerequisiteUnsatisfied) || !strings.Contains(detail, "prerequisite "+pre.TicketID.Raw+" is HELD") || !strings.Contains(detail, "stages integrate") {
		t.Fatalf("claim-next %s %q", code, detail)
	}
	// A GATE_PASSED prerequisite with no gate oracle is refused as an
	// unknown, and the refusal still names the prerequisite and its gate.
	gated := fixture.Ticket("AT-01")
	gated.ExecutionPrerequisites = []ticket.Prerequisite{{TicketID: pre.TicketID, Obligation: "GATE_PASSED", GateID: strPtr("verify"), Stages: []string{"integrate"}}}
	c = issue502ClaimContext(t, LeaseClaimNext, "", gated, pre)
	c.l.Stage = "integrate"
	if code, detail := issue502Refused(planClaimNext(c)); code != wire.CodePrerequisiteUnsatisfied || !strings.Contains(detail, "prerequisite gate verify of "+pre.TicketID.Raw) {
		t.Fatalf("claim-next gate %s %q", code, detail)
	}
	// The unlisted stage admits the same ticket.
	c = issue502ClaimContext(t, LeaseClaimNext, "", rec, pre)
	c.l.Stage = "implement"
	if out := planClaimNext(c); out.result != nil || out.effect == nil {
		t.Fatalf("claim-next of unlisted stage refused %+v", out.result)
	}
}

func strPtr(s string) *string { return &s }
