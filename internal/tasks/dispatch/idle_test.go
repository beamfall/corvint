//go:build darwin || linux

package dispatch

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// witnessQueue is a fakeQueue with a CAL-V0-139 store witness that counts
// the full observations.
type witnessQueue struct {
	fakeQueue
	witness  string
	fail     error
	observes int
}

func (q *witnessQueue) Observe(ctx context.Context) (*Observation, error) {
	q.observes++
	return q.fakeQueue.Observe(ctx)
}

func (q *witnessQueue) Witness() (string, error) { return q.witness, q.fail }

func TestCALV0139_IdleTickSkipsTheReadUntilSomethingChanges(t *testing.T) {
	c := testConfig(t, "exit 0")
	c.Roles[0].Match = &Match{Labels: []string{"no-such-label"}}
	clock := time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)
	q := &witnessQueue{witness: "w1"}
	q.obs.Tickets = []Ticket{ticket("T-1", "P1", 1)}
	q.obs.Attempts = []Attempt{{ID: "a1", Ticket: "ticket:a:q:T-1", Phase: "IMPLEMENTING", Generation: "g1", Live: true, Holder: "someone", LeaseExpires: clock.Add(45 * time.Second)}}
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
	tick(1, "first tick")
	state := filepath.Join(d.dir, "state.json")
	before, err := os.Stat(state)
	if err != nil {
		t.Fatal(err)
	}
	tick(2, "a tick that records the observation reads in full")
	after, err := os.Stat(state)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(before, after) || !before.ModTime().Equal(after.ModTime()) {
		t.Fatal("an unchanged ledger was rewritten")
	}
	tick(2, "an idle tick skips the read")
	tick(2, "and keeps skipping")

	q.witness = "w2"
	tick(3, "a changed witness reads in full")
	tick(3, "the new fixed point skips")

	if err := os.WriteFile(filepath.Join(d.dir, "requests", "x.json"), []byte(`{"unpark":"ticket:a:q:T-9"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	tick(4, "an operator request reads in full")
	tick(4, "a consumed request skips again")

	q.fail = errors.New("unobservable")
	tick(5, "a witness error reads in full")
	tick(6, "and arms nothing")
	q.fail = nil
	tick(7, "a recovered witness first re-arms")
	tick(7, "then skips")

	clock = clock.Add(46 * time.Second)
	tick(9, "a due lease expiry reads in full, and heal re-observes after its reap")
	if len(q.reaped) != 1 {
		t.Fatalf("reaped %v, want the expired lease", q.reaped)
	}
	tick(10, "the reap changed the ledger")
	tick(10, "the new fixed point skips")

	clock = clock.Add(idleFullEvery)
	tick(11, "the safety net reads in full")
	tick(11, "and re-arms")

	// A cooldown is recorded by a full tick; the gate then wakes for it.
	key := "ticket:a:q:T-1"
	d.ledger.Backoff[key] = &BackoffState{Fingerprint: Fingerprint(&q.obs, key), CooldownUntil: clock.Add(10 * time.Second)}
	q.witness = "w3"
	tick(12, "a full tick saves the cooldown")
	tick(13, "the fixed point re-arms")
	tick(13, "and skips until the cooldown")
	clock = clock.Add(11 * time.Second)
	tick(14, "a due cooldown reads in full")

	clock = clock.Add(-time.Hour)
	tick(15, "a clock behind the armed tick reads in full")
}

func TestCALV0139_IdleGateNeedsAWitnessAndNoWorkers(t *testing.T) {
	c := testConfig(t, "exit 0")
	c.Roles[0].Match = &Match{Labels: []string{"no-such-label"}}
	q := &fakeQueue{}
	q.obs.Tickets = []Ticket{ticket("T-1", "P1", 1)}
	d, err := Open("prog", c, q, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	for i := 0; i < 3; i++ {
		if err := d.Tick(context.Background()); err != nil {
			t.Fatal(err)
		}
		if d.idle != nil {
			t.Fatal("a queue without a witness armed the idle gate")
		}
	}
	w := &witnessQueue{witness: "w"}
	w.obs.Tickets = q.obs.Tickets
	d.Queue = w
	for i := 0; i < 2; i++ {
		if err := d.Tick(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if d.idle == nil {
		t.Fatal("an idle dispatcher with a witness did not arm")
	}
	d.ledger.Workers = append(d.ledger.Workers, &Worker{ID: "w1"})
	if d.idleSkip(context.Background()) {
		t.Fatal("a supervised worker was skipped")
	}
	d.ledger.Workers = nil
	cfg := *c
	cfg.WorkState = &WorkState{Kind: "status-line", Path: "/x/{ticketLocal}.md", Key: "state"}
	if d.Config = &cfg; d.idleEligible() {
		t.Fatal("a work-state reader is an input outside the store witness")
	}
}
