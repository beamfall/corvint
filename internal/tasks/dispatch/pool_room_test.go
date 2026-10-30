//go:build darwin || linux

package dispatch

import (
	"context"
	"io"
	"reflect"
	"strings"
	"syscall"
	"testing"
)

func poolRoomConfig(t *testing.T, script string) *Config {
	c := testConfig(t, script)
	c.GlobalCap = 6
	c.Roles = []Role{
		{Name: "laned", Host: "sh", Cap: 4, Match: &Match{PlanSelected: true, Pool: "lanes"}, Prompt: "p {pool}", IdleSeconds: 30, WallSeconds: 60},
		{Name: "impl", Host: "sh", Cap: 2, Match: &Match{PlanSelected: true}, Prompt: "p", IdleSeconds: 30, WallSeconds: 60},
	}
	return c
}

func poolRoomTickets() []Ticket {
	a, b, x, f := ticket("a", "P2", 1), ticket("b", "P0", 2), ticket("c", "P1", 3), ticket("f", "P3", 4)
	a.RequiresPool, b.RequiresPool, x.RequiresPool = "lanes", "lanes", "lanes"
	out := []Ticket{a, b, x, f}
	for i := range out {
		out[i].Plan = "SELECTED"
	}
	return out
}

func roomRoster(c *Config, obs *Observation, workers []*Worker) (out, held []Assignment, room *poolRoom) {
	room = newPoolRoom(obs, workers)
	out, held = roster(c, obs, nil, nil, nil, nil, nil, nil, room, nil)
	return out, held, room
}

func pendingWorker(ticketLocal string) *Worker {
	return &Worker{Role: "laned", Key: "ticket:a:q:" + ticketLocal, Ticket: "ticket:a:q:" + ticketLocal, Pool: "lanes", State: "RUNNING"}
}

// PSR-V0-140: with one free member and three matching tickets one session
// launches, the highest-priority ticket first; the other two are held, a
// role without a pool requirement is unaffected, and an observation that
// reports no pool has no gate.
func TestPSRV0140_OneFreeMemberLaunchesOne(t *testing.T) {
	c := poolRoomConfig(t, "exit 0")
	obs := &Observation{Tickets: poolRoomTickets(), PoolFree: map[string]int{"lanes": 1}}
	out, held, room := roomRoster(c, obs, nil)
	if got, want := keysOf(out), []string{"laned/b/1", "impl/f/1"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("launches %v want %v", got, want)
	}
	if len(held) != 0 {
		t.Fatalf("pressure held %v", held)
	}
	if got, want := keysOf(room.held), []string{"laned/c/0", "laned/a/0"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("room held %v want %v", got, want)
	}
	if room.room["lanes"] != 0 {
		t.Fatalf("room after launch %d", room.room["lanes"])
	}
	// Two free members launch the two highest-priority tickets.
	obs.PoolFree["lanes"] = 2
	if out, _, _ = roomRoster(c, obs, nil); !reflect.DeepEqual(keysOf(out), []string{"laned/b/1", "laned/c/2", "impl/f/1"}) {
		t.Fatalf("two free launches %v", keysOf(out))
	}
	// No free member: only the poolless role launches.
	obs.PoolFree["lanes"] = 0
	if out, _, _ = roomRoster(c, obs, nil); !reflect.DeepEqual(keysOf(out), []string{"impl/f/1"}) {
		t.Fatalf("no-room launches %v", keysOf(out))
	}
	// A pool the observation does not report, or no report at all, is not gated.
	obs.PoolFree = map[string]int{"other": 0}
	if out, _, _ = roomRoster(c, obs, nil); len(out) != 4 {
		t.Fatalf("unreported pool launches %v", keysOf(out))
	}
	obs.PoolFree = nil
	if out, _, room = roomRoster(c, obs, nil); len(out) != 4 || room != nil {
		t.Fatalf("ungated launches %v room %v", keysOf(out), room)
	}
}

// PSR-V0-141: a launched worker whose ticket holds no member yet counts
// against room until its attempt holds the pool or the worker ends.
func TestPSRV0141_PendingWorkerCountsAgainstRoom(t *testing.T) {
	c := poolRoomConfig(t, "exit 0")
	tickets := poolRoomTickets()
	// "z" is a fourth lanes ticket already being worked.
	z := ticket("z", "P3", 9)
	z.RequiresPool, z.Plan = "lanes", "SELECTED"
	obs := &Observation{Tickets: append(tickets, z), PoolFree: map[string]int{"lanes": 1}}
	workers := []*Worker{pendingWorker("z")}
	out, _, room := roomRoster(c, obs, workers)
	if room.pending["lanes"] != 1 || !reflect.DeepEqual(keysOf(out), []string{"impl/f/1"}) {
		t.Fatalf("pending %d launches %v", room.pending["lanes"], keysOf(out))
	}
	// Once z's live attempt holds a lanes member it no longer counts; the
	// free count already excludes that member.
	obs.Attempts = []Attempt{{ID: "attempt:a:q:1", Ticket: z.ID, Live: true, Pool: "lanes", Member: "m1"}}
	out, _, room = roomRoster(c, obs, workers)
	if room.pending["lanes"] != 0 || !reflect.DeepEqual(keysOf(out), []string{"laned/b/1", "impl/f/1"}) {
		t.Fatalf("held-member pending %d launches %v", room.pending["lanes"], keysOf(out))
	}
	// A live attempt without a pool allocation leaves the worker pending.
	obs.Attempts[0].Pool, obs.Attempts[0].Member = "", ""
	if _, _, room = roomRoster(c, obs, workers); room.pending["lanes"] != 1 {
		t.Fatalf("unallocated attempt pending %d", room.pending["lanes"])
	}
}

// PSR-V0-142: a stale worker record (KILLING, poolless, or for a ticket
// that is not observed OPEN) is ignored.
func TestPSRV0142_StaleWorkerIgnored(t *testing.T) {
	c := poolRoomConfig(t, "exit 0")
	tickets := poolRoomTickets()
	done := ticket("d", "P3", 9)
	done.Status = "COMPLETED"
	obs := &Observation{Tickets: append(tickets, done), PoolFree: map[string]int{"lanes": 1}}
	killing := pendingWorker("a")
	killing.State = "KILLING"
	poolless := pendingWorker("c")
	poolless.Pool = ""
	workers := []*Worker{killing, poolless, pendingWorker("d"), pendingWorker("gone"), nil}
	out, _, room := roomRoster(c, obs, workers)
	if room.pending["lanes"] != 0 || !reflect.DeepEqual(keysOf(out), []string{"laned/b/1", "impl/f/1"}) {
		t.Fatalf("stale pending %d launches %v", room.pending["lanes"], keysOf(out))
	}
}

// PSR-V0-143: each held ticket gets one `pool-wait` event naming the
// reason and the computed room; an unchanged hold is not re-reported, and
// a change in the pending count is.
func TestPSRV0143_PoolWaitEventsReportRoom(t *testing.T) {
	c := poolRoomConfig(t, "exec sleep 60")
	q := &fakeQueue{obs: Observation{Tickets: poolRoomTickets()[:3], PoolFree: map[string]int{"lanes": 1}}}
	d, err := Open("prog", c, q, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	t.Cleanup(func() {
		for _, w := range d.ledger.Workers {
			syscall.Kill(w.PID, syscall.SIGKILL)
		}
	})
	waits := func() []Event {
		t.Helper()
		events, err := ReadEvents(d.dir, 1000)
		if err != nil {
			t.Fatal(err)
		}
		var out []Event
		for _, e := range events {
			if e.Kind == "pool-wait" {
				out = append(out, e)
			}
		}
		return out
	}
	tick := func() {
		t.Helper()
		if err := d.Tick(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	tick()
	if d.Running() != 1 || d.ledger.Workers[0].Ticket != "ticket:a:q:b" || d.ledger.Workers[0].Pool != "lanes" {
		t.Fatalf("first tick running %d workers %+v", d.Running(), d.ledger.Workers)
	}
	ev := waits()
	if len(ev) != 2 || ev[0].Ticket != "ticket:a:q:c" || ev[1].Ticket != "ticket:a:q:a" {
		t.Fatalf("first-tick pool-wait events %+v", ev)
	}
	want := map[string]string{"pool": "lanes", "free": "1", "pending": "0", "room": "0", "reason": "POOL_ROOM"}
	if !reflect.DeepEqual(ev[0].Detail, want) {
		t.Fatalf("detail %v want %v", ev[0].Detail, want)
	}
	// The launched worker has not claimed: it is now pending, so the hold is
	// re-reported once with the new numbers and nothing else launches.
	tick()
	ev = waits()
	if d.Running() != 1 || len(ev) != 4 || ev[2].Detail["pending"] != "1" || ev[2].Detail["room"] != "0" {
		t.Fatalf("second tick running %d events %+v", d.Running(), ev)
	}
	tick()
	if ev = waits(); len(ev) != 4 {
		t.Fatalf("unchanged hold re-reported: %+v", ev)
	}
}

// PSR-V0-267: a ticket the plan defers as POOL_MEMBER_RECOVERY_STALLED
// reaches the planSelected pool role's room as a hold and never launches;
// a pool ticket deferred for any other reason stays out of the room.
func TestPSRV0267_PlanStalledDeferralReachesPoolRoom(t *testing.T) {
	c := poolRoomConfig(t, "exit 0")
	tickets := poolRoomTickets()
	for i := range tickets[:3] {
		tickets[i].Plan, tickets[i].PlanReason = "DEFERRED", PlanReasonPoolStalled
	}
	tickets[2].PlanReason = "RESOURCE_COLLISION"
	obs := &Observation{Tickets: tickets, PoolFree: map[string]int{"lanes": 0}, PoolStalled: map[string]bool{"lanes": true}}
	out, _, room := roomRoster(c, obs, nil)
	if !reflect.DeepEqual(keysOf(out), []string{"impl/f/1"}) || len(room.held) != 2 {
		t.Fatalf("stalled deferral launches %v held %v", keysOf(out), keysOf(room.held))
	}
}

// PSR-V0-265: a pool whose every member is QUARANTINED with a stalled
// recovery holds every pool-needing launch, even with room reported, and
// each held ticket's `pool-wait` event names POOL_MEMBER_RECOVERY_STALLED
// and how to read the condition; a role without a pool still launches.
func TestPSRV0265_StalledPoolHoldsLaunches(t *testing.T) {
	c := poolRoomConfig(t, "exec sleep 60")
	obs := &Observation{Tickets: poolRoomTickets(), PoolFree: map[string]int{"lanes": 1}, PoolStalled: map[string]bool{"lanes": true}}
	if out, _, room := roomRoster(c, obs, nil); !reflect.DeepEqual(keysOf(out), []string{"impl/f/1"}) || len(room.held) != 3 {
		t.Fatalf("stalled launches %v held %v", keysOf(out), keysOf(room.held))
	}
	q := &fakeQueue{obs: Observation{Tickets: poolRoomTickets()[:1], PoolFree: map[string]int{"lanes": 0}, PoolStalled: map[string]bool{"lanes": true}}}
	d, err := Open("prog", c, q, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	t.Cleanup(func() {
		for _, w := range d.ledger.Workers {
			syscall.Kill(w.PID, syscall.SIGKILL)
		}
	})
	if err := d.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	events, err := ReadEvents(d.dir, 1000)
	if err != nil {
		t.Fatal(err)
	}
	var waits []Event
	for _, e := range events {
		if e.Kind == "pool-wait" {
			waits = append(waits, e)
		}
	}
	if d.Running() != 0 || len(waits) != 1 || waits[0].Detail["reason"] != "POOL_MEMBER_RECOVERY_STALLED" || !strings.Contains(waits[0].Message, "pool status --pool lanes") {
		t.Fatalf("stalled pool running %d events %+v", d.Running(), waits)
	}
}
