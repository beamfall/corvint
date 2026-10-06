//go:build darwin || linux

package dispatch

import (
	"context"
	"errors"
	"io"
	"reflect"
	"testing"
	"time"
)

// refusingQueue refuses the first releaseFails releases and reapFails reaps
// (negative: every one) and records each request ID.
type refusingQueue struct {
	*fakeQueue
	releaseFails, reapFails int
	requests                []string
}

func (q *refusingQueue) Release(ctx context.Context, a Attempt, evidence, request string) error {
	q.requests = append(q.requests, request)
	if q.releaseFails != 0 {
		q.releaseFails--
		q.released = append(q.released, "refused:"+a.ID)
		return errors.New("FENCED")
	}
	return q.fakeQueue.Release(ctx, a, evidence, request)
}

func (q *refusingQueue) Reap(ctx context.Context, a Attempt, request string) error {
	q.requests = append(q.requests, request)
	if q.reapFails != 0 {
		q.reapFails--
		q.reaped = append(q.reaped, "refused:"+a.ID)
		return errors.New("LOCK_TIMEOUT")
	}
	return q.fakeQueue.Reap(ctx, a, request)
}

// exitRig opens a dispatcher on a fake clock whose launched worker "w1" has
// ended while holding live attempt a1 (lease expiring at lease). The role
// matches nothing, so no worker launches.
func exitRig(t *testing.T, q *refusingQueue, lease time.Time, now *time.Time, configure func(*Config)) *Dispatcher {
	t.Helper()
	c := testConfig(t, "exit 0")
	c.Roles[0].Match.Labels = []string{"do-not-launch"}
	if configure != nil {
		configure(c)
	}
	q.obs.Tickets = []Ticket{ticket("t1", "P1", 1)}
	d, err := Open("exit", c, q, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	d.Now = func() time.Time { return *now }
	tk := q.obs.Tickets[0].ID
	d.ledger.Workers = append(d.ledger.Workers, &Worker{ID: "w1", Role: "impl", Host: "sh", Key: tk, Ticket: tk, PID: -1, Started: *now, LastActive: *now})
	q.obs.Attempts = []Attempt{{ID: "a1", Ticket: tk, Holder: "w1", Phase: "RUNNING", Generation: "7", Live: true, LeaseExpires: lease}}
	return d
}

func tick(t *testing.T, d *Dispatcher) {
	t.Helper()
	if err := d.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func count(list []string, s string) int {
	n := 0
	for _, x := range list {
		if x == s {
			n++
		}
	}
	return n
}

func recoveryOf(t *testing.T, d *Dispatcher, kind string) []string {
	t.Helper()
	events, err := ReadEvents(d.dir, 1000)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range events {
		if e.Kind == kind {
			out = append(out, e.Detail["recovery"])
		}
	}
	return out
}

func TestCALV0104_ExitRecoveryRetriesHandoffOnLiveLease(t *testing.T) {
	now := time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)
	q := &refusingQueue{fakeQueue: &fakeQueue{}, releaseFails: 1}
	d := exitRig(t, q, now.Add(time.Hour), &now, nil)
	tick(t, d)
	if got := kinds(t, d); !has(got, "handoff-refused") || has(got, "needs-owner") {
		t.Fatalf("first refusal events %v", got)
	}
	tick(t, d) // inside the backoff: no write
	if !reflect.DeepEqual(q.released, []string{"refused:a1"}) {
		t.Fatalf("released %v before the backoff elapsed", q.released)
	}
	now = now.Add(time.Second)
	tick(t, d)
	if !reflect.DeepEqual(q.released, []string{"refused:a1", "a1"}) || q.evidence[0] != "dispatch:w1" {
		t.Fatalf("released %v evidence %v", q.released, q.evidence)
	}
	if q.requests[0] == q.requests[1] {
		t.Fatal("a retried release reused the refused request ID")
	}
	if got := recoveryOf(t, d, "handoff"); !reflect.DeepEqual(got, []string{"RELEASED"}) || has(kinds(t, d), "needs-owner") {
		t.Fatalf("handoff recoveries %v events %v", got, kinds(t, d))
	}
	if len(d.recoveries) != 0 {
		t.Fatalf("recovery kept after release: %v", d.recoveries)
	}
}

func TestCALV0104_ExitRecoveryReapsExpiredLease(t *testing.T) {
	t.Run("expired-at-exit", func(t *testing.T) {
		now := time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)
		q := &refusingQueue{fakeQueue: &fakeQueue{}, releaseFails: -1}
		d := exitRig(t, q, now.Add(-9*time.Minute), &now, nil)
		tick(t, d)
		if !reflect.DeepEqual(q.reaped, []string{"a1"}) || has(kinds(t, d), "needs-owner") {
			t.Fatalf("reaped %v events %v", q.reaped, kinds(t, d))
		}
		if got := recoveryOf(t, d, "reaped"); !reflect.DeepEqual(got, []string{"REAPED"}) {
			t.Fatalf("reaped recoveries %v", got)
		}
	})
	t.Run("expires-after-bounded-releases", func(t *testing.T) {
		now := time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)
		q := &refusingQueue{fakeQueue: &fakeQueue{}, releaseFails: -1}
		d := exitRig(t, q, now.Add(time.Hour), &now, nil)
		for _, step := range []time.Duration{0, time.Second, 2 * time.Second, time.Minute, 10 * time.Minute} {
			now = now.Add(step)
			tick(t, d)
		}
		if n := count(q.released, "refused:a1"); n != exitRecoveryTries || len(q.reaped) != 0 {
			t.Fatalf("releases %v reaps %v before expiry", q.released, q.reaped)
		}
		now = now.Add(time.Hour)
		tick(t, d)
		if !reflect.DeepEqual(q.reaped, []string{"a1"}) || has(kinds(t, d), "needs-owner") {
			t.Fatalf("reaped %v events %v", q.reaped, kinds(t, d))
		}
	})
}

func TestCALV0104_ExitRecoveryResolvesFencedAttempt(t *testing.T) {
	t.Run("already-ended", func(t *testing.T) {
		now := time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)
		q := &refusingQueue{fakeQueue: &fakeQueue{}, releaseFails: -1}
		d := exitRig(t, q, now.Add(time.Hour), &now, nil)
		tick(t, d)
		q.obs.Attempts[0].Live, q.obs.Attempts[0].Phase = false, "FAILED" // fenced elsewhere
		now = now.Add(time.Hour + time.Minute)
		tick(t, d)
		tick(t, d)
		if len(q.reaped) != 0 || len(q.released) != 1 || has(kinds(t, d), "needs-owner") {
			t.Fatalf("released %v reaped %v events %v", q.released, q.reaped, kinds(t, d))
		}
		if got := recoveryOf(t, d, "handoff"); !reflect.DeepEqual(got, []string{"ALREADY_ENDED"}) {
			t.Fatalf("handoff recoveries %v", got)
		}
	})
	t.Run("generation-changed", func(t *testing.T) {
		now := time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)
		q := &refusingQueue{fakeQueue: &fakeQueue{}, releaseFails: -1}
		d := exitRig(t, q, now.Add(time.Hour), &now, func(c *Config) { c.Heal.Reap = false })
		tick(t, d)
		q.obs.Attempts[0].Generation, q.obs.Attempts[0].Holder = "8", "someone-else"
		q.obs.Attempts[0].LeaseExpires = now.Add(-time.Minute)
		now = now.Add(time.Minute)
		tick(t, d)
		if len(q.reaped) != 0 || len(q.released) != 1 || has(kinds(t, d), "needs-owner") {
			t.Fatalf("touched a superseded attempt: released %v reaped %v events %v", q.released, q.reaped, kinds(t, d))
		}
		if got := recoveryOf(t, d, "handoff"); !reflect.DeepEqual(got, []string{"SUPERSEDED"}) {
			t.Fatalf("handoff recoveries %v", got)
		}
	})
}

func TestCALV0104_ExitRecoveryNeedsOwnerOnlyWhenBothFail(t *testing.T) {
	now := time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)
	q := &refusingQueue{fakeQueue: &fakeQueue{}, releaseFails: -1, reapFails: -1}
	// heal.reap off isolates the recovery from the generic per-tick reap.
	d := exitRig(t, q, now.Add(-time.Minute), &now, func(c *Config) { c.Heal.Reap = false })
	for i := 0; i < 8; i++ {
		tick(t, d)
		if i < 2 && has(kinds(t, d), "needs-owner") {
			t.Fatalf("needs-owner before the reaps were exhausted: %v", q.reaped)
		}
		now = now.Add(time.Minute)
	}
	if n := count(q.reaped, "refused:a1"); n != exitRecoveryTries {
		t.Fatalf("reaps %v", q.reaped)
	}
	if n := count(kinds(t, d), "needs-owner"); n != 1 {
		t.Fatalf("needs-owner %d times: %v", n, kinds(t, d))
	}
}

func TestCALV0104_ExitRecoverySwitchOff(t *testing.T) {
	now := time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)
	q := &refusingQueue{fakeQueue: &fakeQueue{}, releaseFails: -1}
	off := false
	d := exitRig(t, q, now.Add(time.Hour), &now, func(c *Config) { c.Heal.ExitRecovery = &off })
	tick(t, d)
	if got := kinds(t, d); !has(got, "handoff-refused") || !has(got, "needs-owner") {
		t.Fatalf("switch off events %v", got)
	}
	now = now.Add(2 * time.Hour)
	tick(t, d)
	if len(q.released) != 1 || len(d.recoveries) != 0 {
		t.Fatalf("switch off retried: released %v", q.released)
	}
	// heal.reap still reaps the expired lease of a holder that is not running.
	if !reflect.DeepEqual(q.reaped, []string{"a1"}) {
		t.Fatalf("reaped %v", q.reaped)
	}
}

func TestCALV0104_ExitRecoveryConfig(t *testing.T) {
	off, on := false, true
	for _, c := range []struct {
		h    Heal
		want bool
	}{{Heal{Handoff: true}, true}, {Heal{Handoff: true, ExitRecovery: &on}, true}, {Heal{Handoff: true, ExitRecovery: &off}, false}, {Heal{ExitRecovery: &on}, false}} {
		if c.h.ExitRecoveryOn() != c.want {
			t.Errorf("%+v ExitRecoveryOn=%v", c.h, !c.want)
		}
	}
	if d := recoveryBackoffFor(30, 1); d != 30*time.Second {
		t.Errorf("first backoff %v", d)
	}
	if d := recoveryBackoffFor(3600, 3); d != exitRecoveryMaxBackoff {
		t.Errorf("capped backoff %v", d)
	}
}

func recoveryBackoffFor(tickSeconds, n int) time.Duration {
	d := &Dispatcher{Config: &Config{TickSeconds: tickSeconds}}
	return d.recoveryBackoff(n)
}
