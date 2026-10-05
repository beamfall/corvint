//go:build darwin || linux

package cli

import (
	"context"
	"io"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/dispatch"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// issue502DispatchRef is one escalation reference entry; a closed question
// carries its second event.
func issue502DispatchRef(id, kind, state, acceptance string) ticket.EscalationRef {
	r := ticket.EscalationRef{RequestID: id, OriginSha256: wire.Sum([]byte(id + "-origin")), HeadSha256: wire.Sum([]byte(id + "-" + state)), Revision: "1", AcceptanceRevision: wire.Count(acceptance), Kind: kind, State: state}
	if state != "OPEN" {
		r.Revision = "2"
	}
	return r
}

// issue502DispatchRecord is a ticket at acceptance revision 2 carrying
// entries, edited by edit before the codec admits it.
func issue502DispatchRecord(t *testing.T, local string, order int64, edit func(*ticket.Record), entries ...ticket.EscalationRef) *ticket.Record {
	t.Helper()
	rec := fixture.Ticket(local)
	rec.Order = wire.CountOf(order)
	rec.Effects.TouchPaths = []string{"src/" + local + "/"}
	rec.AcceptanceRevision, rec.Revision = "2", "2"
	d := wire.Sum([]byte("revision 1"))
	rec.PreviousRecordSha256 = &d
	if len(entries) > 0 {
		var events int64
		for _, e := range entries {
			events += e.Revision.Int()
		}
		rec.Revision = wire.CountOf(2 + events)
		rec.Escalations = &ticket.EscalationRefs{Revision: wire.CountOf(events), LastControlTicketRevision: rec.Revision, WorkRevision: "2", Entries: entries}
	}
	if edit != nil {
		edit(rec)
	}
	back, err := ticket.Decode(rec.Encode())
	if err != nil {
		t.Fatalf("%s: %v", local, err)
	}
	return back
}

// issue502DispatchInput is the plan input dispatchQueue.Observe builds, over
// the fixture queue and policy and no attempts.
func issue502DispatchInput(t *testing.T, recs ...*ticket.Record) transaction.PlanInput {
	t.Helper()
	q, err := intent.DecodeQueue(fixture.QueueBytes())
	if err != nil {
		t.Fatal(err)
	}
	p, err := intent.DecodePolicy(fixture.PolicyBytes())
	if err != nil {
		t.Fatal(err)
	}
	p.RequireEnforcedFields = nil
	tickets, err := ticket.NewInventory(q.QueueID, recs)
	if err != nil {
		t.Fatal(err)
	}
	return transaction.PlanInput{Queue: q, Policy: p, Tickets: tickets, Reservations: &snapshot.ReservationSet{QueueID: q.QueueID}, Attempts: map[string]*snapshot.Attempt{}}
}

// issue502Queue serves one fixed native observation to a dispatcher.
type issue502Queue struct{ obs dispatch.Observation }

func (q *issue502Queue) Observe(context.Context) (*dispatch.Observation, error) {
	o := q.obs
	o.Tickets = append([]dispatch.Ticket(nil), q.obs.Tickets...)
	return &o, nil
}
func (q *issue502Queue) Release(context.Context, dispatch.Attempt, string, string) error { return nil }
func (q *issue502Queue) Reap(context.Context, dispatch.Attempt, string) error            { return nil }

func issue502DispatchConfig(t *testing.T) *dispatch.Config {
	root := t.TempDir()
	return &dispatch.Config{Profile: dispatch.ConfigProfile, StateDir: filepath.Join(root, "state"), WorkRoot: root, TickSeconds: 1, GlobalCap: 4, KillGraceSeconds: 1,
		Hosts: map[string]dispatch.Host{"sh": {Argv: []string{"/bin/sh", "-c", "exit 0"}}},
		Roles: []dispatch.Role{
			{Name: "any", Host: "sh", Cap: 1, Match: &dispatch.Match{}, Prompt: "work", IdleSeconds: 30, WallSeconds: 60},
			{Name: "selected", Host: "sh", Cap: 1, Match: &dispatch.Match{PlanSelected: true}, Prompt: "work", IdleSeconds: 30, WallSeconds: 60},
		},
		Backoff: dispatch.Backoff{CooldownSeconds: 3600, ParkAfter: 2}}
}

// TestIssue502_DispatchNeverLaunchesForEscalationHold proves ESC-V0-006 on
// the dispatcher's launch path: the native observation carries the derived
// hold of a current OPEN decision, scope or blocked question, and the roster
// skips the ticket for every role, whether or not the role requires a
// selected plan, so no session launches. Stale, answered, superseded and
// infrastructure questions leave the ticket launchable.
func TestIssue502_DispatchNeverLaunchesForEscalationHold(t *testing.T) {
	ref := issue502DispatchRef
	cases := []struct {
		name  string
		entry ticket.EscalationRef
		held  bool
	}{
		{"decision", ref("q-a", "decision", "OPEN", "2"), true},
		{"scope", ref("q-a", "scope", "OPEN", "2"), true},
		{"blocked", ref("q-a", "blocked", "OPEN", "2"), true},
		{"stale", ref("q-a", "decision", "OPEN", "1"), false},
		{"answered", ref("q-a", "decision", "ANSWERED", "2"), false},
		{"superseded", ref("q-a", "scope", "SUPERSEDED", "2"), false},
		{"infrastructure", ref("q-a", "infrastructure", "OPEN", "2"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := issue502DispatchRecord(t, "AT-01", 0, nil, tc.entry)
			obs := dispatch.Observation{Tickets: dispatchTickets(issue502DispatchInput(t, rec))}
			if got := obs.Tickets[0].EscalationPending; tc.held != (len(got) == 1 && got[0] == "q-a") || (!tc.held && got != nil) {
				t.Fatalf("observed hold %v", got)
			}
			c := issue502DispatchConfig(t)
			roster := dispatch.Roster(c, &obs, nil, nil)
			if tc.held != (len(roster) == 0) {
				t.Fatalf("roster %+v", roster)
			}
			if !tc.held && !slices.ContainsFunc(roster, func(a dispatch.Assignment) bool { return a.Role == "any" }) {
				t.Fatalf("roster %+v lacks the role that does not require a selected plan", roster)
			}
			d, err := dispatch.Open("prog", c, &issue502Queue{obs: obs}, io.Discard)
			if err != nil {
				t.Fatal(err)
			}
			defer d.Close()
			if err := d.Tick(context.Background()); err != nil {
				t.Fatal(err)
			}
			if tc.held != (d.Running() == 0) {
				t.Fatalf("running %d", d.Running())
			}
		})
	}
}

// TestIssue502_DispatchStatusKeepsHoldBehindOtherBlockers: a ticket whose
// primary plan reason is a pool collision, a queue pause or an ordinary hold
// still carries its escalation hold through the native observation, the
// dispatcher's ledger and status, and is never rostered.
func TestIssue502_DispatchStatusKeepsHoldBehindOtherBlockers(t *testing.T) {
	ref := issue502DispatchRef
	pool := issue502DispatchRecord(t, "AT-01", 0, func(r *ticket.Record) { r.RequiresPool = "demo" }, ref("q-a", "decision", "OPEN", "2"))
	held := issue502DispatchRecord(t, "AT-02", 1, func(r *ticket.Record) {
		r.Status = ticket.StatusHeld
		r.Holds = []ticket.Hold{{HoldID: "hold", Actor: "owner", Reason: "hold", PlacedAt: "2026-10-04T00:00:00Z"}}
	}, ref("q-b", "scope", "OPEN", "2"), ref("q-c", "blocked", "OPEN", "2"))
	plain := issue502DispatchRecord(t, "AT-03", 2, nil, ref("q-d", "decision", "OPEN", "1"))
	in := issue502DispatchInput(t, pool, held, plain)
	paused := in
	paused.Barrier = true
	for _, run := range []struct {
		name    string
		in      transaction.PlanInput
		reasons map[string]string
	}{
		{"pool and ordinary hold", in, map[string]string{"AT-01": wire.CodeResourceCollision, "AT-02": wire.CodeTicketHeld}},
		{"paused", paused, map[string]string{"AT-01": wire.CodeResourceCollision, "AT-02": wire.CodePaused, "AT-03": wire.CodePaused}},
	} {
		t.Run(run.name, func(t *testing.T) {
			obs := dispatch.Observation{Tickets: dispatchTickets(run.in)}
			for _, x := range obs.Tickets {
				if want, ok := run.reasons[x.Local]; ok && x.PlanReason != want {
					t.Fatalf("%s plan reason %s, want %s", x.Local, x.PlanReason, want)
				}
			}
			c := issue502DispatchConfig(t)
			c.Roles[0].Match.Statuses = []string{"OPEN", "HELD"}
			for _, a := range dispatch.Roster(c, &obs, nil, nil) {
				if a.Local != "AT-03" {
					t.Fatalf("held ticket rostered: %+v", a)
				}
			}
			d, err := dispatch.Open("prog", c, &issue502Queue{obs: obs}, io.Discard)
			if err != nil {
				t.Fatal(err)
			}
			defer d.Close()
			if err := d.Tick(context.Background()); err != nil {
				t.Fatal(err)
			}
			l, err := dispatch.LoadLedger(dispatch.ProgramDir(c, "prog"), "prog")
			if err != nil {
				t.Fatal(err)
			}
			v, ok := statusField(t, dispatchStatusValue(c, dispatch.ProgramDir(c, "prog"), l, nil, time.Now()), "escalationPending")
			want := `[{"requests":["q-a"],"ticket":"` + pool.TicketID.Raw + `"},{"requests":["q-b","q-c"],"ticket":"` + held.TicketID.Raw + `"}]`
			if !ok || string(wire.Encode(v)) != want {
				t.Fatalf("escalationPending = %s", wire.Encode(v))
			}
		})
	}
}
