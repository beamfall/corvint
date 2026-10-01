package transaction

import (
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
	"testing"
)

func TestCALV0043_RecoveryExaminesEveryAttempt(t *testing.T) {
	t.Run("CAL-V0-043 recovery inventory safety", func(t *testing.T) {
		for _, name := range []string{"cancelled", "failed", "clean-handoff", "clean-return", "not-exhausted", "wrong-revision", "active-older", "pending-older", "unsafe-older", "ambiguous-generation", "reservation", "supervised-fenced", "missing"} {
			t.Run(name, func(t *testing.T) {
				rec := fixture.Ticket("AT-01")
				qid, _ := wire.ParseQueueID("", rec.TicketID.QueueID())
				inv, err := ticket.NewInventory(qid, []*ticket.Record{rec})
				if err != nil {
					t.Fatal(err)
				}
				a := &snapshot.Attempt{TicketID: rec.TicketID, TicketRevision: rec.AcceptanceRevision, Generation: "2", Phase: "CANCELLED", RetryCount: "3", RuntimeID: snapshot.RuntimeExternalAgent, Quiescence: "FENCED"}
				st := inputState{tickets: inv, attempts: map[string]*snapshot.Attempt{"latest": a}, reservations: &snapshot.ReservationSet{}}
				older := *a
				older.Generation = "1"
				switch name {
				case "clean-handoff", "clean-return":
					disposition := wire.CodeHandoff
					if name == "clean-return" {
						disposition = wire.CodeReviewReturned
					}
					a.RetryAccounting = &snapshot.RetryAccounting{Disposition: disposition}
				case "failed":
					a.Phase = "FAILED"
				case "not-exhausted":
					a.RetryCount = "2"
				case "wrong-revision":
					a.TicketRevision = "2"
				case "active-older":
					older.Phase = "RUNNING"
					st.attempts["older"] = &older
				case "pending-older":
					older.PendingEffects = []string{"pending"}
					st.attempts["older"] = &older
				case "unsafe-older":
					older.Quiescence = "UNPROVED"
					st.attempts["older"] = &older
				case "ambiguous-generation":
					older.Generation = a.Generation
					st.attempts["older"] = &older
				case "reservation":
					st.reservations.Entries = []snapshot.ReservationEntry{{TicketID: rec.TicketID}}
				case "supervised-fenced":
					a.RuntimeID = snapshot.SupervisedProfile
				case "missing":
					st.attempts = nil
				}
				env := &mutation.Envelope{Operation: mutation.OpReopen, TargetID: &rec.TicketID}
				got := retryRecovery(st, env)
				eligible := name == "cancelled" || name == "failed"
				if (got.State == ticket.Satisfied) != eligible {
					t.Fatalf("%s: %+v", name, got)
				}
			})
		}
	})
}

// CAL-V0-044: a legacy cause or review label never supplies prospective evidence.
func TestCALV0044_LegacyReasonCannotExempt(t *testing.T) {
	t.Run("CAL-V0-044 LegacyReasonCannotExempt", func(t *testing.T) {
		rec := fixture.Ticket("AT-01")
		qid, _ := wire.ParseQueueID("", rec.TicketID.QueueID())
		inv, err := ticket.NewInventory(qid, []*ticket.Record{rec})
		if err != nil {
			t.Fatal(err)
		}
		policy, err := intent.DecodePolicy(fixture.PolicyBytes())
		if err != nil {
			t.Fatal(err)
		}
		reason := wire.CodeHandoff
		tree := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		a := &snapshot.Attempt{TicketID: rec.TicketID, TicketRevision: rec.AcceptanceRevision, Generation: "2", Phase: "CANCELLED", RetryCount: "3", RuntimeID: snapshot.RuntimeExternalAgent, Quiescence: "FENCED", Stage: "review", Cause: &reason, CandidateTreeOid: &tree, ScopeCheck: "WITHIN", PolicySha256: wire.Sum(policy.Raw), ConfigSha256: wire.Sum(policy.Raw)}
		st := inputState{tickets: inv, policy: policy, attempts: map[string]*snapshot.Attempt{"last": a}, reservations: &snapshot.ReservationSet{}}
		if !retryExhausted(st.attempts, rec) {
			t.Fatal("legacy cause refunded retries")
		}
		env := &mutation.Envelope{Operation: mutation.OpReopen, TargetID: &rec.TicketID}
		if retryRecovery(st, env).State != ticket.Satisfied {
			t.Fatal("legacy owner recovery changed")
		}
		a.Phase = "BUILT"
		c := leaseContext{st: st, l: &LeaseRequest{Reason: wire.CodeHandoff}}
		result := c.verifyHandoff(a)
		if result == nil || result.result == nil || !result.result.Outcome.HasCode(wire.CodeMissingEvidence) {
			t.Fatalf("legacy handoff accepted: %+v", result)
		}
	})
}
