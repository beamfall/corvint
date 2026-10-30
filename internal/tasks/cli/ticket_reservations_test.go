package cli_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// Reservations are journal facts, including an expired lease until a writer removes it.
func TestTicketReservationReads(t *testing.T) {
	for _, expired := range []bool{false, true} {
		name := "current"
		base := time.Now().UTC().Add(-12 * time.Minute).Truncate(time.Second)
		if expired {
			name = "expired-unreaped"
			base = base.Add(-8 * time.Minute)
		}
		t.Run(name, func(t *testing.T) {
			root, claims := leaseCLIStore(t, 1, base)
			claim := claims[0]
			repo, err := intent.Resolve(root)
			if err != nil {
				t.Fatal(err)
			}
			before := fixture.TreeSnapshot(t, root)
			beforeState := fixture.TreeSnapshot(t, repo.StateDir)
			for _, verb := range []string{"show", "blockers"} {
				res := atm(t, root, nil, "ticket", verb, claim.Ticket).res
				if res.Outcome != wire.OutcomeOK || len(res.Items) != 1 {
					t.Fatalf("%s: %+v", verb, res)
				}
				item := res.Items[0]
				if field(item, "currentAttempt").Str != "SATISFIED" {
					t.Fatalf("%s currentAttempt: %s", verb, wire.Encode(item))
				}
				found := false
				for _, b := range field(item, "blockers").Arr {
					if field(b, "code").Str == wire.CodeAttemptLive {
						found = true
					}
				}
				if !found {
					t.Fatalf("%s missing reservation blocker", verb)
				}
				for _, u := range field(item, "unknowns").Arr {
					if field(u, "code").Str == wire.CodeAttemptLive {
						t.Fatalf("%s unobserved reservation", verb)
					}
				}
				q := atm(t, root, nil, "queue", "status").res
				live := field(q.Items[0], "liveAttempts").Arr
				if len(live) != 1 || field(live[0], "ticketId").Str != claim.Ticket || *res.Snapshot.HeadSeq != *q.Snapshot.HeadSeq {
					t.Fatalf("ticket and queue disagree: ticket=%s queue=%s ticket-head=%+v queue-head=%+v target=%q", wire.Encode(item), wire.Encode(q.Items[0]), res.Snapshot, q.Snapshot, claim.Ticket)
				}
				expiry, err := time.Parse(time.RFC3339, field(live[0], "expiresAt").Str)
				if err != nil {
					t.Fatal(err)
				}
				if expiry.Before(time.Now()) != expired {
					t.Fatalf("expiry %s expired=%v", expiry, expired)
				}
				if field(item, "eligibility").Str == "ELIGIBLE" || field(item, "publication").Str != "NOT_OBSERVED" {
					t.Fatalf("invented authority: %s", wire.Encode(item))
				}
			}
			if !fixture.SameTree(before, fixture.TreeSnapshot(t, root)) || !fixture.SameTree(beforeState, fixture.TreeSnapshot(t, repo.StateDir)) {
				t.Fatal("read changed intent/state")
			}
			verb := "release"
			args := []string{verb, "--attempt", claim.AttemptID, "--generation", string(claim.Generation), "--request-id", "remove-reservation"}
			if expired {
				args[0] = "reap"
			}
			if res := atm(t, root, nil, args...).res; res.Outcome != wire.OutcomeOK {
				t.Fatalf("remove: %+v", res)
			}
			for _, v := range []string{"show", "blockers"} {
				res := atm(t, root, nil, "ticket", v, claim.Ticket).res
				if res.Outcome != wire.OutcomeOK || field(res.Items[0], "currentAttempt").Str != "UNSATISFIED" {
					t.Fatalf("removed reservation: %+v", res)
				}
				for _, b := range field(res.Items[0], "blockers").Arr {
					if field(b, "code").Str == wire.CodeAttemptLive {
						t.Fatal("removed reservation blocks")
					}
				}
			}
			q := atm(t, root, nil, "queue", "status").res
			if q.Outcome != wire.OutcomeOK || len(field(q.Items[0], "liveAttempts").Arr) != 0 {
				t.Fatalf("removed queue reservation: %+v", q)
			}
		})
	}
}

func TestTicketReservationReadRefusal(t *testing.T) {
	for _, kind := range []string{"missing", "malformed"} {
		t.Run(kind, func(t *testing.T) {
			root, claims := expiredCLIStore(t, 1)
			repo, err := intent.Resolve(root)
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(repo.StateDir, "reservations.json")
			if kind == "missing" {
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
			} else {
				fixture.Write(t, path, []byte("{}\n"))
			}
			before := fixture.TreeSnapshot(t, root)
			state := fixture.TreeSnapshot(t, repo.StateDir)
			for _, verb := range []string{"show", "blockers"} {
				res := atm(t, root, nil, "ticket", verb, claims[0].Ticket).res
				if res.Outcome == wire.OutcomeOK || len(res.Items) != 0 || len(res.Codes) != 1 || res.Codes[0] != wire.CodeJournalForked {
					t.Fatalf("unaudited fallback: %+v", res)
				}
			}
			if !fixture.SameTree(before, fixture.TreeSnapshot(t, root)) || !fixture.SameTree(state, fixture.TreeSnapshot(t, repo.StateDir)) {
				t.Fatal("failed read mutated")
			}
		})
	}
}
