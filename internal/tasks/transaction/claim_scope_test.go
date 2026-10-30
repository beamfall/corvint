package transaction

import (
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func TestCALV0022_DerivedFactsBindSelectedTicket(t *testing.T) {
	t.Run("CAL-V0-022 reject another ticket's scope", func(t *testing.T) {
		id, err := wire.ParseTicketID("ticketId", "ticket:acme:main:AT-0001")
		if err != nil {
			t.Fatal(err)
		}
		record := &ticket.Record{TicketID: id}
		c := leaseContext{l: &LeaseRequest{}, in: Input{LeaseFacts: LeaseFacts{DerivedTicketID: "ticket:acme:main:AT-0002", DerivedPaths: []string{"private.go"}, DerivationSha256: wire.Digest(strings.Repeat("a", 64))}}}
		if _, err = c.claimScope(record); err == nil {
			t.Fatal("scope for another ticket was accepted")
		}
		c.in.LeaseFacts.DerivedTicketID = id.Raw
		if scope, err := c.claimScope(record); err != nil || scope.Source != "DERIVED" {
			t.Fatalf("matching ticket: %+v %v", scope, err)
		}
	})
}
