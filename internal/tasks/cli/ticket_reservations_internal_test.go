package cli

import (
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func TestTicketReservationJournalBinding(t *testing.T) {
	for _, verb := range []string{"show", "blockers"} {
		t.Run(verb, func(t *testing.T) {
			r := fixture.TempRepo(t)
			fixture.WriteIntent(t, r, fixture.Ticket("A"))
			fixture.WriteState(t, r)
			fixture.CommitPosts(t, r, "MUTATION", "", map[string][]byte{"intent/tickets/A.json": fixture.Ticket("A").Encode()})
			// Even generation zero must audit the journal's empty reservation set.
			before := fixture.TreeSnapshot(t, r.Root)
			state := fixture.TreeSnapshot(t, r.StateDir)
			res := ticketShow(Env{Cwd: r.Root}, []string{"A"}, verb == "show")
			if res.Outcome != wire.OutcomeOK || len(res.Items) != 1 {
				t.Fatalf("empty reservations: %+v", res)
			}
			current, _ := res.Items[0].Obj.Get("currentAttempt")
			if current.Str != "UNSATISFIED" {
				t.Fatalf("empty reservations: %s", wire.Encode(res.Items[0]))
			}
			if !fixture.SameTree(before, fixture.TreeSnapshot(t, r.Root)) || !fixture.SameTree(state, fixture.TreeSnapshot(t, r.StateDir)) {
				t.Fatal("read mutated")
			}
			// A schema-valid wrong queue must not acquire the outer queue's authority.
			fixture.CommitPosts(t, r, "MUTATION", "", map[string][]byte{"reservations.json": []byte(`{"entries":[],"profile":"taskman-reservation-set/0","queueId":"queue:other:main"}` + "\n")})
			before = fixture.TreeSnapshot(t, r.Root)
			state = fixture.TreeSnapshot(t, r.StateDir)
			res = ticketShow(Env{Cwd: r.Root}, []string{"A"}, verb == "show")
			if len(res.Items) != 0 || len(res.Codes) != 1 || res.Codes[0] != wire.CodeJournalForked {
				t.Fatalf("wrong queue: %+v", res)
			}
			if !fixture.SameTree(before, fixture.TreeSnapshot(t, r.Root)) || !fixture.SameTree(state, fixture.TreeSnapshot(t, r.StateDir)) {
				t.Fatal("failed read mutated")
			}
		})
	}
}

func TestTicketReservationMovingHead(t *testing.T) {
	for _, verb := range []string{"show", "blockers"} {
		t.Run(verb, func(t *testing.T) {
			r := fixture.TempRepo(t)
			fixture.WriteIntent(t, r, fixture.Ticket("A"))
			fixture.WriteState(t, r)
			fixture.CommitPosts(t, r, "MUTATION", "", map[string][]byte{"intent/tickets/A.json": fixture.Ticket("A").Encode()})
			calls := 0
			res := ticketShow(Env{Cwd: r.Root, afterRead: func() {
				calls++
				// Advance a valid journal after reservation audit, before outer re-probe.
				fixture.CommitPosts(t, r, "MUTATION", "", map[string][]byte{"reservations.json": []byte(`{"entries":[],"profile":"taskman-reservation-set/0","queueId":"queue:acme:main"}` + "\n")})
			}}, []string{"A"}, verb == "show")
			if calls != 4 || res.Outcome != wire.OutcomeNotRun || len(res.Items) != 0 || len(res.Codes) != 1 || res.Codes[0] != wire.CodeSnapshotMoved {
				t.Fatalf("stale reservation read: calls=%d %+v", calls, res)
			}
			// The racing fixture is the only writer; reads create no lock.
			before := fixture.TreeSnapshot(t, r.Root)
			state := fixture.TreeSnapshot(t, r.StateDir)
			res = ticketShow(Env{Cwd: r.Root}, []string{"A"}, verb == "show")
			if res.Outcome != wire.OutcomeOK || !fixture.SameTree(before, fixture.TreeSnapshot(t, r.Root)) || !fixture.SameTree(state, fixture.TreeSnapshot(t, r.StateDir)) {
				t.Fatalf("stable reread: %+v", res)
			}
		})
	}
}
