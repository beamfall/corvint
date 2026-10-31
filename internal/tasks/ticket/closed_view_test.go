// SPDX-License-Identifier: Apache-2.0

package ticket_test

import (
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

type allGates struct{ ticket.NoEvidence }

func (allGates) GatePassed(wire.TicketID, string, wire.Count) ticket.Observation {
	return ticket.Satisfied
}

func (allGates) LiveAttempt(string) ticket.Observation { return ticket.Satisfied }

// TestTCXV0_ClosedRoadmapRowsMatchView proves a closed ticket's summary
// carries its title and required gates and that ClosedView equals View's
// eligibility, gate results and next action whatever else the record holds
// (TCX-V0-986).
func TestTCXV0_ClosedRoadmapRowsMatchView(t *testing.T) {
	completed := ticket.StatusCompleted
	variants := []func(*ticket.Record){
		func(r *ticket.Record) {},
		func(r *ticket.Record) { r.RequiredGates = []string{"review", "verify"} },
		func(r *ticket.Record) {
			r.Dependencies = []ticket.Dependency{fixture.Dep("O"), fixture.GateDep("O", "g")}
		},
		func(r *ticket.Record) { r.Dependencies = []ticket.Dependency{fixture.Dep("MISSING")} },
		func(r *ticket.Record) { r.ExecutionClass = "APPROVAL_REQUIRED" },
		func(r *ticket.Record) { r.Effects.Coverage = "UNKNOWN"; r.Effects.ExternalUnbounded = true },
	}
	ctxs := []ticket.Context{{}, {Gates: allGates{}, Attempts: allGates{}, CanonicalWriter: "NATIVE", SerialFallback: "WHOLE_REPOSITORY"}}
	for _, status := range []string{ticket.StatusCompleted, ticket.StatusArchived} {
		for i, vary := range variants {
			rec := fixture.Ticket("C")
			rec.Status = status
			if status == ticket.StatusArchived {
				rec.ArchivedFrom = &completed
			}
			vary(rec)
			inv := inventory(t, rec, fixture.Ticket("O"))
			s, ok := inv.Summary(rec.TicketID.Raw)
			if !ok || s.Title != rec.Title || len(s.RequiredGates) != len(rec.RequiredGates) {
				t.Fatalf("%s/%d: summary %+v", status, i, s)
			}
			for _, ctx := range ctxs {
				v, _ := inv.View(rec.TicketID.Raw, ctx)
				el, next := ticket.ClosedView(s)
				if el != v.Eligibility || next != v.NextAction || v.GateResults != ticket.NotObserved {
					t.Fatalf("%s/%d: ClosedView %s %s, View %s %s %s", status, i, el, next, v.Eligibility, v.NextAction, v.GateResults)
				}
			}
		}
	}
}
