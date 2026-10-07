package ticket_test

import (
	"slices"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestCALV0186_UnmetCompletedDependencies: only COMPLETED obligations count,
// COMPLETED and ARCHIVED-from-COMPLETED satisfy them, a missing ticket is
// unmet with an empty status, and the result is in ticketId byte order
// whatever order the record declares.
func TestCALV0186_UnmetCompletedDependencies(t *testing.T) {
	done := fixture.Ticket("DONE")
	done.Status = ticket.StatusCompleted
	done.Completion = &ticket.Completion{Kind: "MANUAL", Actor: "a", Evidence: []wire.Digest{}, RecordedAt: fixture.Timestamp}
	tomb := fixture.Ticket("TOMB")
	tomb.Status = ticket.StatusArchived
	from := ticket.StatusCompleted
	tomb.ArchivedFrom = &from
	tomb.Completion = done.Completion
	held := fixture.Ticket("HELD")
	held.Status = "HELD"
	open := fixture.Ticket("OPEN")
	gated := fixture.Ticket("GATED")
	target := fixture.Ticket("TARGET")
	target.Dependencies = []ticket.Dependency{fixture.Dep("ZMISSING"), fixture.Dep("OPEN"), fixture.Dep("DONE"), fixture.Dep("TOMB"), fixture.GateDep("GATED", "verify"), fixture.Dep("HELD")}
	inv := inventory(t, done, tomb, held, open, gated, target)
	got := inv.UnmetCompletedDependencies(fixture.TicketID("TARGET"))
	want := []ticket.UnmetDependency{{TicketID: fixture.TicketID("HELD"), Status: "HELD"}, {TicketID: fixture.TicketID("OPEN"), Status: "OPEN"}, {TicketID: fixture.TicketID("ZMISSING")}}
	if !slices.Equal(got, want) {
		t.Fatalf("unmet = %+v, want %+v", got, want)
	}
	if got := inv.UnmetCompletedDependencies(fixture.TicketID("DONE")); len(got) != 0 {
		t.Fatalf("no dependencies, got %+v", got)
	}
	if got := inv.UnmetCompletedDependencies(fixture.TicketID("NOPE")); got != nil {
		t.Fatalf("unknown ticket, got %+v", got)
	}
}
