package cli_test

import (
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestTicketShowRecordNamesItsTicketID (V1-2826) pins the documented
// contract in docs/TASKS-EXTERNAL-AGENTS.md: `ticket show` pairs the record
// with its ID through record.ticketId, equal to the item's ticketId, and the
// stored taskman-ticket/0 record has no id key. --fields record.ticketId
// selects it without the rest of the record.
func TestTicketShowRecordNamesItsTicketID(t *testing.T) {
	r := fixture.TempRepo(t)
	fixture.Write(t, filepath.Join(r.IntentDir, "queue.json"), fixture.QueueBytes())
	fixture.Write(t, filepath.Join(r.IntentDir, "policy.json"), fixture.PolicyBytes())
	if x := atm(t, r.Root, nil, "init"); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("init: %s", x.stdout)
	}
	id, _ := effectsTicket(t, r.Root, "paired", "P2", `{"coverage":"INCOMPLETE","externalUnbounded":false,"resources":[],"touchPaths":[]}`)
	show := atm(t, r.Root, nil, "ticket", "show", id)
	item := show.res.Items[0]
	record := field(item, "record")
	if got := field(record, "ticketId").Str; got == "" || got != field(item, "ticketId").Str {
		t.Fatalf("record.ticketId %q does not name the item's ticket: %s", got, show.stdout)
	}
	if _, ok := record.Obj.Get("id"); ok {
		t.Fatalf("the taskman-ticket/0 record gained an id key: %s", show.stdout)
	}
	sel := atm(t, r.Root, nil, "ticket", "show", id, "--fields", "record.ticketId")
	if sel.res.Outcome != wire.OutcomeOK || field(field(sel.res.Items[0], "record"), "ticketId").Str != field(item, "ticketId").Str {
		t.Fatalf("--fields record.ticketId: %s", sel.stdout)
	}
}
