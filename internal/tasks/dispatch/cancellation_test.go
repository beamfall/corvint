//go:build darwin || linux

package dispatch

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// An exec-ed reader isolates tick cancellation from descendant cleanup,
// which has separate lifecycle witnesses.
func TestCALV0053_CancelledTickKeepsState(t *testing.T) {
	c := testConfig(t, "exit 0")
	c.Roles[0].Match.Labels = []string{"do-not-launch"}
	q := &fakeQueue{obs: Observation{Tickets: []Ticket{ticket("t", "P1", 1)}}}
	d, err := Open("cancel", c, q, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if d != nil {
			_ = d.Close()
		}
	})
	if err := d.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	seen := *d.ledger.Seen
	seq := d.LastEvent()
	key := q.obs.Tickets[0].ID
	backoff := &BackoffState{NoProgress: 1, Parked: true, Fingerprint: Fingerprint(&q.obs, key)}
	d.ledger.Backoff[key] = backoff
	request := filepath.Join(d.dir, "requests", "unpark.json")
	raw, _ := json.Marshal(UnparkRequest{Unpark: key})
	if err := os.WriteFile(request, raw, 0600); err != nil {
		t.Fatal(err)
	}
	reader := filepath.Join(c.WorkRoot, "reader.sh")
	ready := filepath.Join(c.WorkRoot, "ready")
	if err := os.WriteFile(reader, []byte("#!/bin/sh\necho ready > ready\nexec /bin/sleep 30\n"), 0700); err != nil {
		t.Fatal(err)
	}
	c.WorkState = &WorkState{Kind: "command", Argv: []string{reader}}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan error, 1)
	go func() { result <- d.Tick(ctx) }()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(ready); err == nil {
			break
		}
		if time.Now().After(deadline) {
			cancel()
			<-result
			t.Fatal("reader did not start")
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("Tick error %v, want context.Canceled", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("cancelled tick did not return")
	}
	if !reflect.DeepEqual(*d.ledger.Seen, seen) {
		t.Errorf("cancelled tick changed Seen: %+v -> %+v", seen, *d.ledger.Seen)
	}
	if d.LastEvent() != seq {
		t.Errorf("cancelled tick emitted %d events", d.LastEvent()-seq)
	}
	if d.ledger.Backoff[key] != backoff || backoff.NoProgress != 1 {
		t.Error("cancelled tick changed backoff")
	}
	if _, err := os.Stat(request); err != nil {
		t.Error("cancelled tick consumed unpark request")
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	d = nil
	// Close and reopen must retain the good baseline, avoiding the restart
	// bounce that originally flooded subscribers with state events.
	c.WorkState = nil
	d, err = Open("cancel", c, q, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(*d.ledger.Seen, seen) {
		t.Error("restart lost the last good baseline")
	}
	if err := os.Remove(request); err != nil {
		t.Fatal(err)
	}
	seq = d.LastEvent()
	if err := d.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if d.LastEvent() != seq {
		t.Error("restart emitted a synthetic state bounce")
	}
}

func TestCALV0053_ReaderFailureAndRealUnknownRemainObservable(t *testing.T) {
	c := testConfig(t, "exit 0")
	c.Roles[0].Match.Labels = []string{"do-not-launch"}
	q := &fakeQueue{obs: Observation{Tickets: []Ticket{ticket("t", "P1", 1)}}}
	d, err := Open("failure", c, q, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	reader := filepath.Join(c.WorkRoot, "reader.sh")
	write := func(body string) {
		t.Helper()
		if err := os.WriteFile(reader, []byte("#!/bin/sh\n"+body+"\n"), 0700); err != nil {
			t.Fatal(err)
		}
	}
	c.WorkState = &WorkState{Kind: "command", Argv: []string{reader}}
	write("exit 3")
	seq := d.LastEvent()
	if err := d.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := d.LastEvent() - seq; got != 2 {
		t.Fatalf("reader error events %d, want alert and state", got)
	}
	if got := d.ledger.Seen.Tickets[q.obs.Tickets[0].ID]; !strings.Contains(got, "|UNKNOWN|") {
		t.Fatalf("error state %q", got)
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	write("echo '{\"t\":\"built\"}'")
	d, err = Open("failure", c, q, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	seq = d.LastEvent()
	if err := d.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if d.LastEvent()-seq != 1 {
		t.Fatal("recovery state transition lost")
	}
	write("echo '{\"t\":\"UNKNOWN\"}'")
	seq = d.LastEvent()
	if err := d.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if d.LastEvent()-seq != 1 {
		t.Fatal("real UNKNOWN state transition lost")
	}
}

type cancelHealingQueue struct {
	*fakeQueue
	cancel   context.CancelFunc
	healKind string
}

func (q *cancelHealingQueue) Release(ctx context.Context, a Attempt, evidence, request string) error {
	err := q.fakeQueue.Release(ctx, a, evidence, request)
	if q.healKind == "release" && len(q.released) == 1 {
		q.cancel()
	}
	return err
}
func (q *cancelHealingQueue) Reap(ctx context.Context, a Attempt, request string) error {
	err := q.fakeQueue.Reap(ctx, a, request)
	if q.healKind == "reap" && len(q.reaped) == 1 {
		q.cancel()
	}
	return err
}
func TestCALV0056_CancelledHealingStopsNextWrite(t *testing.T) {
	for _, kind := range []string{"release", "reap"} {
		t.Run(kind, func(t *testing.T) {
			c := testConfig(t, "exit 0")
			c.Roles[0].Match.Labels = []string{"do-not-launch"}
			q := &cancelHealingQueue{fakeQueue: &fakeQueue{obs: Observation{Tickets: []Ticket{ticket("t1", "P1", 1), ticket("t2", "P1", 2)}}}, healKind: kind}
			d, err := Open("heal", c, q, io.Discard)
			if err != nil {
				t.Fatal(err)
			}
			defer d.Close()
			if err := d.Tick(context.Background()); err != nil {
				t.Fatal(err)
			}
			seen := *d.ledger.Seen
			key := q.obs.Tickets[0].ID
			b := &BackoffState{NoProgress: 1, Parked: true, Fingerprint: Fingerprint(&q.obs, key)}
			d.ledger.Backoff[key] = b
			request := filepath.Join(d.dir, "requests", "unpark.json")
			raw, _ := json.Marshal(UnparkRequest{Unpark: key})
			if err := os.WriteFile(request, raw, 0600); err != nil {
				t.Fatal(err)
			}
			for i, tk := range q.obs.Tickets {
				holder := tk.Local + "-ended"
				expires := time.Now().Add(-time.Minute)
				if kind == "release" {
					expires = time.Now().Add(time.Hour)
					d.ledger.Workers = append(d.ledger.Workers, &Worker{ID: holder, Role: "impl", Host: "sh", Key: tk.ID, Ticket: tk.ID, PID: -1, Started: time.Now(), LastActive: time.Now()})
				}
				q.obs.Attempts = append(q.obs.Attempts, Attempt{ID: []string{"a1", "a2"}[i], Ticket: tk.ID, Holder: holder, Phase: "RUNNING", Generation: "1", Live: true, LeaseExpires: expires})
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			q.cancel = cancel
			if err := d.Tick(ctx); !errors.Is(err, context.Canceled) {
				t.Errorf("Tick=%v", err)
			}
			writes := q.reaped
			if kind == "release" {
				writes = q.released
			}
			if !reflect.DeepEqual(writes, []string{"a1"}) {
				t.Errorf("writes=%v, want only a1", writes)
			}
			if !reflect.DeepEqual(*d.ledger.Seen, seen) {
				t.Error("canceled healing changed Seen")
			}
			if d.ledger.Backoff[key] != b || b.NoProgress != 1 || !b.Parked {
				t.Error("canceled healing changed backoff")
			}
			if _, err := os.Stat(request); err != nil {
				t.Error("canceled healing consumed unpark request")
			}
			q.cancel = func() {}
			if err := d.Tick(context.Background()); err != nil {
				t.Fatal(err)
			}
			writes = q.reaped
			if kind == "release" {
				writes = q.released
			}
			if !reflect.DeepEqual(writes, []string{"a1", "a2"}) {
				t.Errorf("healthy reconciliation writes=%v", writes)
			}
		})
	}
}

type cancelObservationQueue struct {
	*fakeQueue
	cancel          context.CancelFunc
	reads, cancelAt int
}

func (q *cancelObservationQueue) Observe(ctx context.Context) (*Observation, error) {
	q.reads++
	o, err := q.fakeQueue.Observe(ctx)
	if q.reads == q.cancelAt {
		q.cancel()
	}
	return o, err
}
func TestCALV0053_CancelledReobservationKeepsDurableState(t *testing.T) {
	for _, stage := range []string{"after-supervision", "after-heal"} {
		t.Run(stage, func(t *testing.T) {
			c := testConfig(t, "exit 0")
			c.Roles[0].Match.Labels = []string{"do-not-launch"}
			q := &cancelObservationQueue{fakeQueue: &fakeQueue{obs: Observation{Tickets: []Ticket{ticket("t", "P1", 1)}}}}
			d, err := Open("reobserve", c, q, io.Discard)
			if err != nil {
				t.Fatal(err)
			}
			defer d.Close()
			if err := d.Tick(context.Background()); err != nil {
				t.Fatal(err)
			}
			seen := *d.ledger.Seen
			q.obs.Tickets[0].Status = "HELD"
			key := q.obs.Tickets[0].ID
			backoff := &BackoffState{NoProgress: 1, Parked: true, Fingerprint: "old"}
			d.ledger.Backoff[key] = backoff
			request := filepath.Join(d.dir, "requests", "unpark.json")
			raw, _ := json.Marshal(UnparkRequest{Unpark: key})
			if err := os.WriteFile(request, raw, 0600); err != nil {
				t.Fatal(err)
			}
			d.ledger.Workers = []*Worker{{ID: "ended", Role: "impl", Host: "sh", PID: -1, Key: key, Ticket: key, Started: time.Now()}}
			q.reads = 0
			q.cancelAt = 2
			if stage == "after-heal" {
				q.cancelAt = 3
				q.obs.Attempts = []Attempt{{ID: "a", Ticket: key, Holder: "ended", Live: true, Phase: "RUNNING", Generation: "1"}}
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			q.cancel = cancel
			if err := d.Tick(ctx); !errors.Is(err, context.Canceled) {
				t.Errorf("Tick=%v", err)
			}
			if !reflect.DeepEqual(*d.ledger.Seen, seen) {
				t.Error("interrupted reobservation changed Seen")
			}
			if d.ledger.Backoff[key] != backoff || backoff.NoProgress != 1 {
				t.Error("interrupted reobservation changed backoff")
			}
			if d.Running() != 1 {
				t.Error("interrupted reobservation accounted worker")
			}
			if _, err := os.Stat(request); err != nil {
				t.Error("interrupted reobservation consumed unpark request")
			}
			if stage == "after-heal" && len(q.released) != 1 {
				t.Error("completed heal was lost")
			}
			q.cancelAt = 0
			if err := d.Tick(context.Background()); err != nil {
				t.Fatal(err)
			}
			if d.Running() != 0 {
				t.Error("healthy tick did not reconcile ended worker")
			}
		})
	}
}
func TestCALV0052_RunNormalizesShutdown(t *testing.T) {
	for _, ticks := range []int{1, 0} {
		t.Run(map[int]string{1: "bounded", 0: "loop"}[ticks], func(t *testing.T) {
			c := testConfig(t, "exit 0")
			c.Roles[0].Match.Labels = []string{"do-not-launch"}
			q := &cancelObservationQueue{fakeQueue: &fakeQueue{obs: Observation{Tickets: []Ticket{ticket("t", "P1", 1)}}}}
			d, err := Open("run", c, q, io.Discard)
			if err != nil {
				t.Fatal(err)
			}
			defer d.Close()
			if err := d.Tick(context.Background()); err != nil {
				t.Fatal(err)
			}
			seq := d.LastEvent()
			q.obs.Tickets[0].Status = "HELD"
			q.reads = 0
			q.cancelAt = 1
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			q.cancel = cancel
			if err := d.Run(ctx, ticks); err != nil {
				t.Errorf("Run=%v", err)
			}
			if d.LastEvent() != seq {
				t.Error("shutdown run emitted interrupted observation events")
			}
		})
	}
}

func TestCALV0053_EndedContextDoesNotObserve(t *testing.T) {
	for _, deadline := range []bool{false, true} {
		t.Run(map[bool]string{false: "cancelled", true: "deadline"}[deadline], func(t *testing.T) {
			c := testConfig(t, "exit 0")
			q := &cancelObservationQueue{fakeQueue: &fakeQueue{}}
			d, err := Open("ended", c, q, io.Discard)
			if err != nil {
				t.Fatal(err)
			}
			defer d.Close()
			var ctx context.Context
			var cancel context.CancelFunc
			if deadline {
				ctx, cancel = context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
			} else {
				ctx, cancel = context.WithCancel(context.Background())
				cancel()
			}
			defer cancel()
			seq := d.LastEvent()
			if err := d.Tick(ctx); !errors.Is(err, ctx.Err()) {
				t.Fatalf("Tick error %v, want %v", err, ctx.Err())
			}
			if q.reads != 0 || d.LastEvent() != seq || d.ledger.Seen != nil {
				t.Fatal("ended context observed or published state")
			}
		})
	}
}
