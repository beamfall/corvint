package transaction

import (
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
	"testing"
)

func TestIssue503_RecordedAdmissionProvenance(t *testing.T) {
	t.Run("CAL-V0-049 recorded default admission", func(t *testing.T) {
		for _, name := range []string{"selected", "unknown", "unknown-held", "unknown-collision", "unknown-capacity", "coverage-fallback", "expired-reservation", "completed", "pool-required"} {
			t.Run(name, func(t *testing.T) {
				rec, dep := fixture.Ticket("AT-01"), fixture.Ticket("AT-02")
				q, err := intent.DecodeQueue(fixture.QueueBytes())
				if err != nil {
					t.Fatal(err)
				}
				policy, err := intent.DecodePolicy(fixture.PolicyBytes())
				if err != nil {
					t.Fatal(err)
				}
				policy.RequireEnforcedFields = nil
				rec.Effects.TouchPaths = []string{"src/"}
				gate := "verify"
				if name == "unknown" || name == "unknown-held" || name == "unknown-collision" || name == "unknown-capacity" {
					rec.Dependencies = []ticket.Dependency{{TicketID: dep.TicketID, Obligation: "GATE_PASSED", GateID: &gate}}
				}
				wantKind, wantBool, wantReason := wire.KindBool, true, PlanSelected
				switch name {
				case "unknown":
					wantKind, wantReason = wire.KindNull, wire.CodeDependencyUnsatisfied
				case "unknown-held":
					rec.Status = ticket.StatusHeld
					rec.Holds = []ticket.Hold{{HoldID: "hold", Actor: "owner", Reason: "hold", PlacedAt: timestamp}}
					wantBool, wantReason = false, wire.CodeTicketHeld
				case "unknown-collision", "expired-reservation":
					wantBool, wantReason = false, wire.CodeResourceCollision
				case "unknown-capacity":
					wantBool, wantReason = false, wire.CodeLimitExceeded
				case "coverage-fallback":
					rec.Effects.Coverage = "INCOMPLETE"
				case "completed":
					rec.Status = ticket.StatusCompleted
					wantBool, wantReason = false, wire.CodeTicketState
				case "pool-required":
					rec.RequiresPool = "demo"
					wantBool, wantReason = false, wire.CodeResourceCollision
				}
				inv, err := ticket.NewInventory(q.QueueID, []*ticket.Record{rec, dep})
				if err != nil {
					t.Fatal(err)
				}
				in := PlanInput{Queue: q, Policy: policy, Tickets: inv, Attempts: map[string]*snapshot.Attempt{}, Reservations: &snapshot.ReservationSet{QueueID: q.QueueID}}
				if name == "unknown-collision" || name == "expired-reservation" || name == "unknown-capacity" {
					resources := []ticket.Resource{{Class: "PATH", Key: "src/"}}
					if name == "unknown-capacity" {
						resources = []ticket.Resource{{Class: "PATH", Key: "other/"}}
					}
					in.Reservations.Entries = []snapshot.ReservationEntry{{TicketID: dep.TicketID, Resources: resources}}
				}
				if name == "expired-reservation" {
					in.Attempts["expired"] = &snapshot.Attempt{TicketID: dep.TicketID, Phase: "RUNNING", Lease: &snapshot.Lease{ExpiresAt: "2000-01-01T00:00:00Z"}}
				}
				before := len(in.Reservations.Entries)
				got, reason := RecordedClaimability(in, rec)
				if got.Kind != wantKind || (got.Kind == wire.KindBool && got.Bool != wantBool) || reason != wantReason {
					t.Fatalf("%s: %s reason %s", name, wire.Encode(got), reason)
				}
				if len(in.Reservations.Entries) != before {
					t.Fatal("read changed reservations")
				}
				if name == "unknown" || name == "unknown-held" || name == "unknown-collision" || name == "unknown-capacity" {
					all, known, unknown := claimBlockerObservations(in, rec)
					if len(unknown) != 1 || len(all) != len(known)+len(unknown) {
						t.Fatalf("lost provenance %+v %+v %+v", all, known, unknown)
					}
				}
			})
		}
	})
}
