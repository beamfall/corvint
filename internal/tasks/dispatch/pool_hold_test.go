//go:build darwin || linux

package dispatch

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// holdQueue is a fake store with the ATR-V0-120 PoolHoldQueue capability.
// A claim starts live attempt a1 (generation 3) for the holder with member
// m1 under allocation "sha-a1" unless refuse is set; a pool release clears
// the attempt's allocation. ops records every claim, pool release and
// attempt release in order.
type holdQueue struct {
	*fakeQueue
	refuse   error
	requests []string
	ops      []string
}

func (q *holdQueue) PoolClaim(_ context.Context, a Assignment, holder string, _ int, request string) (PoolHold, error) {
	q.requests = append(q.requests, request)
	if q.refuse != nil {
		return PoolHold{}, q.refuse
	}
	q.ops = append(q.ops, "claim:"+a.Local+":"+a.Pool)
	q.obs.Attempts = append(q.obs.Attempts, Attempt{ID: "a1", Ticket: a.Ticket, Holder: holder, Phase: "RUNNING", Generation: "3", Live: true, LeaseExpires: time.Now().Add(time.Hour), Pool: a.Pool, Member: "m1"})
	return PoolHold{Attempt: "a1", Generation: "3", Member: "m1", Allocation: "sha-a1"}, nil
}

func (q *holdQueue) PoolRelease(_ context.Context, a Attempt, allocation, _ string) error {
	q.ops = append(q.ops, "pool-release:"+a.ID+":"+allocation)
	for i := range q.obs.Attempts {
		if q.obs.Attempts[i].ID == a.ID {
			q.obs.Attempts[i].Pool, q.obs.Attempts[i].Member = "", ""
		}
	}
	return nil
}

func (q *holdQueue) Release(ctx context.Context, a Attempt, evidence, id string) error {
	q.ops = append(q.ops, "release:"+a.ID)
	return q.fakeQueue.Release(ctx, a, evidence, id)
}

// holdDispatcher opens a dispatcher whose holdPool role matches pool db and
// whose only ticket, t1, requires it.
func holdDispatcher(t *testing.T, q Queue, handoff bool) *Dispatcher {
	t.Helper()
	c := testConfig(t, `echo "$CORVINT_DISPATCH_POOL_FILE" > "$CORVINT_DISPATCH_WORKER.pool"; exec sleep 300`)
	c.GlobalCap = 1
	c.Heal.Handoff = handoff
	c.Roles[0].IdleSeconds, c.Roles[0].WallSeconds = 3600, 7200
	c.Roles[0].Match = &Match{Pool: "db"}
	c.Roles[0].HoldPool = true
	t1 := ticket("t1", "P1", 1)
	t1.RequiresPool = "db"
	switch f := q.(type) {
	case *holdQueue:
		f.obs.Tickets = []Ticket{t1}
	case *fakeQueue:
		f.obs.Tickets = []Ticket{t1}
	}
	d, err := Open("hold", c, q, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

// launched ticks d once and returns its one launched worker.
func launched(t *testing.T, d *Dispatcher) *Worker {
	t.Helper()
	tick(t, d)
	if d.Running() != 1 {
		t.Fatalf("running %d, want one launched worker", d.Running())
	}
	w := d.ledger.Workers[0]
	pid := w.PID
	t.Cleanup(func() { syscall.Kill(-pid, syscall.SIGKILL); syscall.Kill(pid, syscall.SIGKILL) })
	return w
}

func sessionPoolFile(t *testing.T, d *Dispatcher, w *Worker) string {
	t.Helper()
	path := filepath.Join(d.Config.WorkRoot, w.ID+".pool")
	for i := 0; i < 100; i++ {
		if _, err := os.Stat(path); err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(got))
}

// ATR-V0-120, ATR-V0-121, ATR-V0-122 (V1-1833, issue 1147): the dispatcher
// claims the ticket with the member it requires before launch, hands the
// hold to the session through CORVINT_DISPATCH_POOL_FILE, and returns the
// member when the session is killed mid-run, before the attempt's hand-off,
// so no allocation is left behind whether or not it hands the attempt off.
func TestATRV0122_KilledSessionLeavesNoOrphanedAllocation(t *testing.T) {
	for _, handoff := range []bool{true, false} {
		q := &holdQueue{fakeQueue: &fakeQueue{}}
		d := holdDispatcher(t, q, handoff)
		w := launched(t, d)
		h := ReadPoolHold(d.dir, w.ID)
		if h == nil || h.State != PoolHoldHeld || h.Attempt != "a1" || h.Generation != "3" || h.Ticket != w.Ticket || h.Pool != "db" || h.Member != "m1" || h.Allocation != "sha-a1" {
			t.Fatalf("hold %+v", h)
		}
		if w.Pool != "db" {
			t.Fatalf("worker pool %q", w.Pool)
		}
		if got := sessionPoolFile(t, d, w); got != PoolHoldPath(d.dir, w.ID) {
			t.Fatalf("session pool file %q", got)
		}
		tick(t, d) // the attempt holds the member: nothing is claimed again
		if len(q.requests) != 1 {
			t.Fatalf("claims %v", q.requests)
		}
		q.obs.Tickets = nil // nothing is launched again
		syscall.Kill(-w.PID, syscall.SIGKILL)
		waitEnded(t, d)
		tick(t, d)
		tick(t, d) // the released hold is not returned again
		h = ReadPoolHold(d.dir, w.ID)
		if h == nil || h.State != PoolHoldReleased || h.Ended.IsZero() {
			t.Fatalf("handoff %v: hold after the kill %+v", handoff, h)
		}
		want := []string{"claim:t1:db", "pool-release:a1:sha-a1"}
		if handoff {
			want = append(want, "release:a1") // the hand-off follows the return
		}
		if strings.Join(q.ops, " ") != strings.Join(want, " ") {
			t.Fatalf("handoff %v: ops %v, want %v", handoff, q.ops, want)
		}
		if a := q.obs.Attempts[0]; a.Pool != "" || a.Member != "" {
			t.Fatalf("handoff %v: attempt still holds %s/%s", handoff, a.Pool, a.Member)
		}
		acts := []string{}
		for _, e := range eventsOf(t, d, "pool-hold") {
			acts = append(acts, e.Detail["action"]+":"+e.Detail["state"])
		}
		if strings.Join(acts, " ") != "claim: release:"+PoolHoldReleased {
			t.Fatalf("pool-hold events %v", acts)
		}
	}
}

// ATR-V0-122: an attempt that ended before its worker left its member
// natively quarantined (CAL-V0-202); the dispatcher records that and writes
// nothing.
func TestATRV0122_EndedAttemptIsNotReleasedAgain(t *testing.T) {
	q := &holdQueue{fakeQueue: &fakeQueue{}}
	d := holdDispatcher(t, q, true)
	w := launched(t, d)
	q.obs.Attempts[0].Live, q.obs.Attempts[0].Phase = false, "COMPLETED"
	q.obs.Tickets = nil
	syscall.Kill(-w.PID, syscall.SIGKILL)
	waitEnded(t, d)
	tick(t, d)
	if h := ReadPoolHold(d.dir, w.ID); h == nil || h.State != PoolHoldEnded {
		t.Fatalf("hold %+v", h)
	}
	if strings.Join(q.ops, " ") != "claim:t1:db" {
		t.Fatalf("ops %v", q.ops)
	}
}

// ATR-V0-122: a launch that fails before spawn returns the member and the
// attempt its claim took.
func TestATRV0122_FailedLaunchReturnsTheClaim(t *testing.T) {
	q := &holdQueue{fakeQueue: &fakeQueue{}}
	d := holdDispatcher(t, q, true)
	host := d.Config.Hosts["sh"]
	host.Argv = []string{filepath.Join(d.Config.WorkRoot, "missing-host"), "{prompt}"}
	d.Config.Hosts["sh"] = host
	tick(t, d)
	if d.Running() != 0 || len(eventsOf(t, d, "launch-failed")) != 1 {
		t.Fatalf("running %d", d.Running())
	}
	if strings.Join(q.ops, " ") != "claim:t1:db pool-release:a1:sha-a1 release:a1" {
		t.Fatalf("ops %v", q.ops)
	}
	if a := q.obs.Attempts[0]; a.Live || a.Pool != "" {
		t.Fatalf("attempt %+v", a)
	}
}

// ATR-V0-120, ATR-V0-124: a refused claim (the native waitlist order,
// PSR-V0-132, put another ticket first) launches nothing, is reported once
// per distinct refusal, and is retried under a fresh request ID on a later
// tick; the dispatcher never names a member itself.
func TestATRV0124_RefusedClaimKeepsNativeOrder(t *testing.T) {
	q := &holdQueue{fakeQueue: &fakeQueue{}}
	d := holdDispatcher(t, q, true)
	q.refuse = errors.New("REFUSED RESOURCE_COLLISION: head waiter t0 (GRACE)")
	tick(t, d)
	tick(t, d)
	if d.Running() != 0 || len(q.ops) != 0 || len(eventsOf(t, d, "launched")) != 0 {
		t.Fatalf("launched after a refusal: ops %v", q.ops)
	}
	if len(q.requests) != 2 || q.requests[0] == q.requests[1] {
		t.Fatalf("requests %v", q.requests)
	}
	if ev := eventsOf(t, d, "pool-hold"); len(ev) != 1 || !strings.Contains(ev[0].Detail["error"], "head waiter t0") {
		t.Fatalf("refusal events %+v", ev)
	}
	q.refuse = nil
	w := launched(t, d)
	if h := ReadPoolHold(d.dir, w.ID); h == nil || h.State != PoolHoldHeld {
		t.Fatalf("hold after admission %+v", h)
	}
}

// ATR-V0-124: a live attempt whose holder is no recorded worker but whose
// hold this dispatcher wrote (a crash between claim and ledger save) is
// returned and ended; one held by a running worker is left alone.
func TestATRV0124_OrphanedClaimIsReturned(t *testing.T) {
	q := &holdQueue{fakeQueue: &fakeQueue{}}
	d := holdDispatcher(t, q, true)
	w := launched(t, d)
	orphan := &PoolHold{Profile: PoolHoldProfile, Worker: "hold.impl.99", State: PoolHoldHeld, Attempt: "a9", Generation: "1", Ticket: "ticket:a:q:t9", Pool: "db", Member: "m2", Allocation: "sha-a9", Acquired: time.Now()}
	if err := writePoolHold(d.dir, orphan); err != nil {
		t.Fatal(err)
	}
	q.obs.Attempts = append(q.obs.Attempts, Attempt{ID: "a9", Ticket: orphan.Ticket, Holder: orphan.Worker, Phase: "RUNNING", Generation: "1", Live: true, LeaseExpires: time.Now().Add(time.Hour), Pool: "db", Member: "m2"})
	tick(t, d)
	if strings.Join(q.ops, " ") != "claim:t1:db pool-release:a9:sha-a9 release:a9" {
		t.Fatalf("ops %v", q.ops)
	}
	if h := ReadPoolHold(d.dir, w.ID); h == nil || h.State != PoolHoldHeld {
		t.Fatalf("running worker's hold %+v", h)
	}
}

// ATR-V0-120: a role without holdPool, or a queue without the capability,
// never claims for the session nor names a hold file; holdPool needs
// match.pool.
func TestATRV0120_HoldPoolIsOptIn(t *testing.T) {
	q := &holdQueue{fakeQueue: &fakeQueue{}}
	d := holdDispatcher(t, q, true)
	d.Config.Roles[0].HoldPool = false
	w := launched(t, d)
	if len(q.requests) != 0 || ReadPoolHold(d.dir, w.ID) != nil || sessionPoolFile(t, d, w) != "" {
		t.Fatalf("claimed without holdPool: %v", q.requests)
	}
	plain := &fakeQueue{}
	d = holdDispatcher(t, plain, true)
	w = launched(t, d)
	if ReadPoolHold(d.dir, w.ID) != nil || sessionPoolFile(t, d, w) != "" {
		t.Fatal("held without the PoolHoldQueue capability")
	}
	c := testConfig(t, "true")
	c.Roles[0].HoldPool = true
	if err := c.validate(); err == nil || !strings.Contains(err.Error(), "holdPool needs match.pool") {
		t.Fatalf("validate %v", err)
	}
}
