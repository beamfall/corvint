//go:build darwin || linux

package dispatch

import (
	"context"
	"io"
	"testing"
)

// TestTOLV0123_MirrorsDecidedIsNotDispatched: a ticket the native plan
// defers as MIRRORS_DECIDED_ELSEWHERE is never rostered, even by a role
// without a planSelected focus, and the dispatcher offers it to the owner
// with one needs-owner event when the reason first appears.
func TestTOLV0123_MirrorsDecidedIsNotDispatched(t *testing.T) {
	t.Run("TOL-V0-123", func(t *testing.T) {
		c := testConfig(t, "exit 0")
		q := &fakeQueue{obs: Observation{Tickets: []Ticket{ledgerTicket(0, 2, "1", 0)}}}
		q.obs.Tickets[0].Plan = "SELECTED"
		d, err := Open("prog", c, q, io.Discard)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = d.Close() })
		ctx := context.Background()
		if err := d.Tick(ctx); err != nil || d.Running() != 1 {
			t.Fatalf("selected ticket not launched: %d %v", d.Running(), err)
		}
		waitEnded(t, d)
		q.obs.Tickets[0].Plan, q.obs.Tickets[0].PlanReason = "DEFERRED", PlanReasonMirrorsDecided
		q.obs.Tickets[0].Revision = "2"
		for i := 0; i < 2; i++ {
			if err := d.Tick(ctx); err != nil || d.Running() != 0 {
				t.Fatalf("tick %d launched a mirrors-decided ticket: %d %v", i, d.Running(), err)
			}
		}
		ev := eventsOf(t, d, "needs-owner")
		if len(ev) != 1 || ev[0].Ticket != q.obs.Tickets[0].ID {
			t.Fatalf("needs-owner events = %+v", ev)
		}
	})
}

// TestTOLV0124_UnblockReleasesParkedTicket: the unblock count adds a
// fingerprint line only once it is positive, so a ledger that never
// unblocked hashes as before; an item leaving BLOCKED releases the parked
// key on the next observation, unless the ledger fold is unknown.
func TestTOLV0124_UnblockReleasesParkedTicket(t *testing.T) {
	t.Run("TOL-V0-124", func(t *testing.T) {
		base := ledgerTicket(0, 2, "1", 0)
		zero := base
		zero.Obligations = &ObligationsView{Total: 2, HighWaterRevision: "1", Unblocks: 0}
		if baseFingerprint(&Observation{Tickets: []Ticket{zero}}, base.ID) != baseFingerprint(&Observation{Tickets: []Ticket{base}}, base.ID) {
			t.Fatal("a zero unblock count changed the fingerprint")
		}
		one := base
		one.Obligations = &ObligationsView{Total: 2, HighWaterRevision: "1", Unblocks: 1}
		if baseFingerprint(&Observation{Tickets: []Ticket{one}}, base.ID) == baseFingerprint(&Observation{Tickets: []Ticket{base}}, base.ID) {
			t.Fatal("an unblock is not a state change")
		}

		for _, unknown := range []bool{false, true} {
			c := testConfig(t, "exit 0")
			c.Backoff.ParkAfter = 2
			q := &fakeQueue{obs: Observation{Tickets: []Ticket{ledgerTicket(0, 2, "1", 0)}}}
			d, err := Open("prog", c, q, io.Discard)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = d.Close() })
			ctx := context.Background()
			for i := 0; i < 2; i++ {
				if err := d.Tick(ctx); err != nil || d.Running() != 1 {
					t.Fatalf("tick %d: %d %v", i, d.Running(), err)
				}
				waitEnded(t, d)
			}
			key := q.obs.Tickets[0].ID
			if err := d.Tick(ctx); err != nil || d.Running() != 0 || d.ledger.Backoff[key] == nil || !d.ledger.Backoff[key].Parked {
				t.Fatalf("not parked: %v %+v", err, d.ledger.Backoff[key])
			}
			q.obs.Tickets[0].Obligations = &ObligationsView{Total: 2, HighWaterRevision: "1", Unblocks: 1, Unknown: unknown}
			if err := d.Tick(ctx); err != nil {
				t.Fatal(err)
			}
			if unknown {
				if d.Running() != 0 || d.ledger.Backoff[key] == nil || !d.ledger.Backoff[key].Parked {
					t.Fatalf("an unknown ledger fold unparked the key: %+v", d.ledger.Backoff[key])
				}
				continue
			}
			if d.Running() != 1 || len(eventsOf(t, d, "unparked")) != 1 {
				t.Fatalf("an unblock did not release the parked key: running %d", d.Running())
			}
			waitEnded(t, d)
		}
	})
}
