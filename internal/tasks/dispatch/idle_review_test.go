//go:build darwin || linux

package dispatch

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"
)

// CAL-V0-139: an unchanged save skips only bytes whose whole save (rename
// and directory fsync) is known to have succeeded; after a failed directory
// sync the next identical save syncs again and reports what happened.
func TestCALV0139_UnchangedSaveRetriesAFailedDirectorySync(t *testing.T) {
	d, err := Open("prog", testConfig(t, "exit 0"), &fakeQueue{}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { syncDir = syncDirectory })
	failing := func(string) error { return errors.New("injected directory sync failure") }
	syncs := 0
	counting := func(dir string) error { syncs++; return syncDirectory(dir) }

	d.ledger.EventSeq++
	syncDir = failing
	if err := d.ledger.save(d.dir); err == nil {
		t.Fatal("a failed directory sync reported a durable save")
	}
	if err := d.ledger.save(d.dir); err == nil {
		t.Fatal("an identical save after a failed directory sync claimed durability")
	}
	syncDir = counting
	if err := d.ledger.save(d.dir); err != nil || syncs != 1 {
		t.Fatalf("identical save after a failed sync: err %v, %d syncs, want a retried sync", err, syncs)
	}
	if err := d.ledger.save(d.dir); err != nil || syncs != 1 {
		t.Fatalf("durable identical save: err %v, %d syncs, want the write skipped", err, syncs)
	}
	syncDir = syncDirectory
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
}

// CAL-V0-139: a deadline that falls inside a full tick (after the tick's
// start, at or before it settles) keeps the gate unarmed, so the next tick
// reads in full instead of skipping until the safety net.
func TestCALV0139_DeadlineCrossedDuringATickDoesNotArm(t *testing.T) {
	c := testConfig(t, "exit 0")
	c.Roles[0].Match = &Match{Labels: []string{"no-such-label"}}
	clock := time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)
	q := &witnessQueue{witness: "w1"}
	q.obs.Tickets = []Ticket{ticket("T-1", "P1", 1)}
	d, err := Open("prog", c, q, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	d.Now = func() time.Time { return clock }
	tick := func(want int, why string) {
		t.Helper()
		if err := d.Tick(context.Background()); err != nil {
			t.Fatalf("%s: tick: %v", why, err)
		}
		if q.observes != want {
			t.Fatalf("%s: %d full observations, want %d", why, q.observes, want)
		}
	}
	key := "ticket:a:q:T-1"
	tick(1, "first tick")
	d.ledger.Backoff[key] = &BackoffState{Fingerprint: Fingerprint(&q.obs, key), CooldownUntil: clock.Add(10 * time.Second)}
	tick(2, "a full tick saves the cooldown")
	q.onObserve = func() { clock = clock.Add(11 * time.Second) }
	tick(3, "the cooldown ends inside this fixed-point tick")
	q.onObserve = nil
	tick(4, "a deadline crossed during the tick reads in full")
	tick(4, "the fixed point after the deadline re-arms and skips")

	// The same holds for a lease expiry the tick observed.
	d.idle = nil
	m := d.idleBegin()
	d.idleLease = clock.Add(time.Second)
	clock = clock.Add(2 * time.Second)
	d.idleSettle(m, nil)
	if d.idle != nil {
		t.Fatal("a lease that expired during the tick armed the gate")
	}
}

// CAL-V0-139: retained pending pool sweep records block the gate even when
// the current configuration no longer sweeps.
func TestCALV0139_PendingSweepRecordsBlockTheGateWithoutSweeping(t *testing.T) {
	c := testConfig(t, "exit 0")
	d, err := Open("prog", c, &fakeQueue{}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if d.Config.PoolSweep != nil {
		t.Fatal("test config sweeps")
	}
	if !d.idleEligible() {
		t.Fatal("an idle dispatcher is not eligible")
	}
	d.ledger.PoolSweeps = map[string]*PoolSweepRecord{"db": {Phase: "PENDING"}}
	if d.idleEligible() {
		t.Fatal("a retained pending sweep record allowed the gate while sweeping is disabled")
	}
}
