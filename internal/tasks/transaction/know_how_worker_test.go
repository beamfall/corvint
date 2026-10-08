package transaction

import (
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestKHNV0022_WorkerAttemptLedger projects real attempt records onto the
// provenance ledger a WORKER KNOWHOW_ADD is checked against: Holder is set
// only while the lease is unexpired (strictly before its expiry), a
// supervised attempt's lease does not expire by time (as for every lease
// command), an unleased attempt has no holder, a terminal phase is never
// live, and an unloaded inventory stays nil so the provenance check refuses.
func TestKHNV0022_WorkerAttemptLedger(t *testing.T) {
	const now = wire.Timestamp("2026-10-07T12:00:00Z")
	id := "att-1"
	attempt := func(phase string, expires wire.Timestamp, supervised bool) *snapshot.Attempt {
		a := &snapshot.Attempt{AttemptID: id, TicketID: fixture.Ticket("AT-01").TicketID, Generation: "3", Phase: phase,
			Lease: &snapshot.Lease{Holder: "agent", ExpiresAt: expires}}
		if supervised {
			a.Supervision = &snapshot.Supervision{WorkerHolder: "agent"}
		}
		return a
	}
	unleased := attempt("RUNNING", now, false)
	unleased.Lease = nil
	for _, c := range []struct {
		name   string
		a      *snapshot.Attempt
		live   bool
		holder string
	}{
		{"running before expiry", attempt("RUNNING", "2026-10-07T12:00:01Z", false), true, "agent"},
		{"expiry at the recorded instant", attempt("RUNNING", now, false), true, ""},
		{"expired", attempt("RUNNING", "2026-10-07T11:00:00Z", false), true, ""},
		{"completed", attempt("COMPLETED", "2026-10-07T13:00:00Z", false), false, "agent"},
		{"cancelled", attempt("CANCELLED", "2026-10-07T13:00:00Z", false), false, "agent"},
		{"failed", attempt("FAILED", "2026-10-07T13:00:00Z", false), false, "agent"},
		{"supervised past its lease time", attempt("RUNNING", "2026-10-07T11:00:00Z", true), true, "agent"},
		{"unleased", unleased, true, ""},
	} {
		p, ok := knowHowLedger(map[string]*snapshot.Attempt{id: c.a}, now)[id]
		if !ok || p.Live != c.live || p.Holder != c.holder || p.TicketID.Raw != c.a.TicketID.Raw || p.Generation != "3" {
			t.Fatalf("%s: %+v", c.name, p)
		}
	}
	if l := knowHowLedger(nil, now); l != nil {
		t.Fatalf("unloaded inventory: %+v", l)
	}
}
