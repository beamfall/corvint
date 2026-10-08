//go:build darwin || linux

package dispatch

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"
)

// leaseRig launches one real, quiet worker on t1 under a fake clock, with
// idle and wall caps far beyond the test's clock moves, and records that it
// holds live attempt a1 whose lease expires at lease.
func leaseRig(t *testing.T, q *refusingQueue, now *time.Time, lease time.Time, configure func(*Config)) (*Dispatcher, *Worker) {
	t.Helper()
	c := testConfig(t, "exec sleep 300")
	c.GlobalCap = 1
	c.Backoff.CooldownSeconds = 3600
	c.Roles[0].IdleSeconds, c.Roles[0].WallSeconds = 3600, 7200
	if configure != nil {
		configure(c)
	}
	q.obs.Tickets = []Ticket{ticket("t1", "P1", 1)}
	d, err := Open("lease", c, q, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	d.Now = func() time.Time { return *now }
	tick(t, d)
	if d.Running() != 1 {
		t.Fatalf("running %d, want one launched worker", d.Running())
	}
	w := d.ledger.Workers[0]
	pid := w.PID
	t.Cleanup(func() { syscall.Kill(-pid, syscall.SIGKILL); syscall.Kill(pid, syscall.SIGKILL) })
	q.obs.Attempts = []Attempt{{ID: "a1", Ticket: w.Ticket, Holder: w.ID, Phase: "RUNNING", Generation: "3", Live: true, LeaseExpires: lease}}
	return d, w
}

func intPtr(n int) *int { return &n }

func TestCALV0191_RunningWorkerPastGraceIsReapedAndStopped(t *testing.T) {
	now := time.Date(2026, 10, 8, 9, 0, 0, 0, time.UTC)
	lease := now.Add(time.Minute)
	q := &refusingQueue{fakeQueue: &fakeQueue{}}
	d, w := leaseRig(t, q, &now, lease, func(c *Config) { c.Roles[0].ExpiredLeaseGraceSeconds = intPtr(120) })
	now = lease.Add(120 * time.Second) // exactly the grace: still untouched
	tick(t, d)
	if len(q.reaped) != 0 || w.State == "KILLING" || has(kinds(t, d), "lease-expired") {
		t.Fatalf("reaped %v state %q at the grace boundary", q.reaped, w.State)
	}
	now = now.Add(time.Second)
	tick(t, d)
	if !reflect.DeepEqual(q.reaped, []string{"a1"}) || !reflect.DeepEqual(q.requests, []string{requestID("reap", "a1", "3", "lease-expired", lease.Format(time.RFC3339))}) {
		t.Fatalf("reaped %v requests %v", q.reaped, q.requests)
	}
	if w.State != "KILLING" || w.KillReason != "LEASE_EXPIRED" {
		t.Fatalf("worker state %q reason %q", w.State, w.KillReason)
	}
	events, err := ReadEvents(d.dir, 1000)
	if err != nil {
		t.Fatal(err)
	}
	var expired *Event
	for i := range events {
		if events[i].Kind == "lease-expired" {
			expired = &events[i]
		}
	}
	if expired == nil || expired.Worker != w.ID || expired.Detail["attempt"] != "a1" || expired.Detail["graceSeconds"] != "120" || expired.Detail["leaseExpires"] != lease.Format(time.RFC3339) {
		t.Fatalf("lease-expired event %+v", expired)
	}
	pid := w.PID
	tick(t, d) // the next supervision pass stops the tree as the wall cap does
	if !gone(pid) {
		t.Fatal("the worker survived its reaped lease")
	}
	got := kinds(t, d)
	if !has(got, "killing") || !has(got, "killed") || !has(got, "finished") || d.Running() != 0 {
		t.Fatalf("events %v running %d", got, d.Running())
	}
	if len(q.reaped) != 1 {
		t.Fatalf("reaped %v, want one reap", q.reaped)
	}
}

func TestCALV0191_WithinGraceOrRenewedIsUntouched(t *testing.T) {
	start := time.Date(2026, 10, 8, 9, 0, 0, 0, time.UTC)
	for name, tc := range map[string]struct {
		lease, at time.Time
		reap      bool
	}{
		"within the default grace": {lease: start.Add(time.Minute), at: start.Add(time.Minute + 600*time.Second)},
		"renewed lease":            {lease: start.Add(time.Hour), at: start.Add(30 * time.Minute)},
		"heal.reap off":            {lease: start.Add(time.Minute), at: start.Add(time.Hour)},
	} {
		t.Run(name, func(t *testing.T) {
			now := start
			q := &refusingQueue{fakeQueue: &fakeQueue{}}
			d, w := leaseRig(t, q, &now, tc.lease, func(c *Config) { c.Heal.Reap = name != "heal.reap off" })
			now = tc.at
			tick(t, d)
			tick(t, d)
			if len(q.requests) != 0 || w.State == "KILLING" || d.Running() != 1 || has(kinds(t, d), "lease-expired") {
				t.Fatalf("requests %v state %q running %d events %v", q.requests, w.State, d.Running(), kinds(t, d))
			}
		})
	}
}

func TestCALV0191_RefusedReapKeepsTheWorker(t *testing.T) {
	now := time.Date(2026, 10, 8, 9, 0, 0, 0, time.UTC)
	lease := now.Add(time.Minute)
	q := &refusingQueue{fakeQueue: &fakeQueue{}, reapFails: 1}
	d, w := leaseRig(t, q, &now, lease, nil)
	now = lease.Add(601 * time.Second)
	tick(t, d)
	if w.State == "KILLING" || has(kinds(t, d), "lease-expired") || !has(kinds(t, d), "alert") {
		t.Fatalf("refused reap: state %q events %v", w.State, kinds(t, d))
	}
	tick(t, d) // the next pass retries with the same request ID
	if len(q.requests) != 2 || q.requests[0] != q.requests[1] || w.State != "KILLING" {
		t.Fatalf("requests %v state %q", q.requests, w.State)
	}
}

// TestCALV0191_WorkerThatWinsTheRaceKeepsRunning: a release or a renewal
// that lands between the observation and the reap leaves the worker
// running; only a reap the store reports as made stops it.
func TestCALV0191_WorkerThatWinsTheRaceKeepsRunning(t *testing.T) {
	start := time.Date(2026, 10, 8, 9, 0, 0, 0, time.UTC)
	lease := start.Add(time.Minute)
	renewed := lease.Add(time.Hour)
	for name, race := range map[string]func(q *refusingQueue) func(Attempt){
		"released": func(q *refusingQueue) func(Attempt) { return func(a Attempt) { q.end(a.ID) } },
		"renewed": func(q *refusingQueue) func(Attempt) {
			return func(Attempt) { q.obs.Attempts[0].LeaseExpires = renewed }
		},
	} {
		t.Run(name, func(t *testing.T) {
			now := start
			q := &refusingQueue{fakeQueue: &fakeQueue{}}
			d, w := leaseRig(t, q, &now, lease, nil)
			q.beforeReap = race(q)
			now = lease.Add(601 * time.Second)
			tick(t, d)
			q.beforeReap = nil
			got := kinds(t, d)
			if len(q.requests) != 1 || len(q.reaped) != 0 || w.State == "KILLING" || has(got, "lease-expired") || has(got, "killing") {
				t.Fatalf("requests %v reaped %v state %q events %v", q.requests, q.reaped, w.State, got)
			}
			if has(got, "alert") != (name == "renewed") {
				t.Fatalf("events %v", got)
			}
			tick(t, d)
			if len(q.requests) != 1 || d.Running() != 1 || w.State == "KILLING" {
				t.Fatalf("after the race: requests %v running %d state %q", q.requests, d.Running(), w.State)
			}
		})
	}
}

// TestCALV0191_StopAfterReapIsRecoveredAfterRestart: a dispatcher that
// ends after the store reaped but before its ledger recorded the stop
// restarts with the worker RUNNING; the store's reaped attempt stops it
// without a second reap.
func TestCALV0191_StopAfterReapIsRecoveredAfterRestart(t *testing.T) {
	now := time.Date(2026, 10, 8, 9, 0, 0, 0, time.UTC)
	lease := now.Add(time.Minute)
	q := &refusingQueue{fakeQueue: &fakeQueue{}}
	d, w := leaseRig(t, q, &now, lease, nil)
	var crashed []byte
	q.afterReap = func(Attempt) {
		raw, err := os.ReadFile(filepath.Join(d.dir, "state.json"))
		if err != nil {
			t.Fatal(err)
		}
		crashed = raw
	}
	now = lease.Add(601 * time.Second)
	tick(t, d)
	q.afterReap = nil
	var l struct {
		Workers []struct {
			State string `json:"state"`
		} `json:"workers"`
	}
	if err := json.Unmarshal(crashed, &l); err != nil || len(l.Workers) != 1 || l.Workers[0].State != "RUNNING" {
		t.Fatalf("ledger at the store write: %v %s", err, crashed)
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(d.dir, "state.json"), crashed, 0o600); err != nil {
		t.Fatal(err)
	}
	restarted, err := Open("lease", d.Config, q, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { restarted.Close() })
	restarted.Now = func() time.Time { return now }
	tick(t, restarted)
	if len(restarted.ledger.Workers) != 1 {
		t.Fatalf("workers after restart: %d", len(restarted.ledger.Workers))
	}
	again := restarted.ledger.Workers[0]
	if len(q.requests) != 1 || len(q.reaped) != 1 {
		t.Fatalf("requests %v reaped %v", q.requests, q.reaped)
	}
	if again.ID != w.ID || again.State != "KILLING" || again.KillReason != "LEASE_EXPIRED" {
		t.Fatalf("restarted worker %+v", again)
	}
	tick(t, restarted)
	if !gone(w.PID) || restarted.Running() != 0 {
		t.Fatalf("the worker survived the restart: running %d", restarted.Running())
	}
}

// TestCALV0191_ReapedAttemptSparesAWorkerWithALiveOne: a reaped attempt
// stops its running holder only while that worker holds no live attempt.
func TestCALV0191_ReapedAttemptSparesAWorkerWithALiveOne(t *testing.T) {
	now := time.Date(2026, 10, 8, 9, 0, 0, 0, time.UTC)
	q := &refusingQueue{fakeQueue: &fakeQueue{}}
	d, w := leaseRig(t, q, &now, now.Add(time.Hour), nil)
	q.obs.Attempts = append(q.obs.Attempts, Attempt{ID: "a0", Ticket: w.Ticket, Holder: w.ID, Phase: "FAILED", Cause: "LEASE_EXPIRED", Generation: "1", LeaseExpires: now.Add(-time.Hour)})
	tick(t, d)
	if w.State == "KILLING" || len(q.requests) != 0 {
		t.Fatalf("state %q requests %v with a live attempt", w.State, q.requests)
	}
	q.obs.Attempts[0].Live, q.obs.Attempts[0].Phase, q.obs.Attempts[0].Cause = false, "FAILED", "LEASE_EXPIRED" // reaped by another writer
	tick(t, d)
	if w.State != "KILLING" || w.KillReason != "LEASE_EXPIRED" || len(q.requests) != 0 {
		t.Fatalf("state %q reason %q requests %v", w.State, w.KillReason, q.requests)
	}
}

// TestCALV0191_ReclaimAfterTheStopDecisionKeepsTheWorker: a recovery stop
// decided on an observation that predates the worker's new claim is
// cancelled on the next tick before any signal; a stop that has already
// signalled is never cancelled.
func TestCALV0191_ReclaimAfterTheStopDecisionKeepsTheWorker(t *testing.T) {
	t.Run("before the signal", func(t *testing.T) {
		now := time.Date(2026, 10, 8, 9, 0, 0, 0, time.UTC)
		q := &refusingQueue{fakeQueue: &fakeQueue{}}
		d, w := leaseRig(t, q, &now, now.Add(time.Hour), nil)
		q.obs.Attempts[0].Live, q.obs.Attempts[0].Phase, q.obs.Attempts[0].Cause = false, "FAILED", "LEASE_EXPIRED"
		tick(t, d)
		if w.State != "KILLING" || !w.KillDeadline.IsZero() {
			t.Fatalf("recovery: state %q deadline %v", w.State, w.KillDeadline)
		}
		// The worker claimed a new generation after the deciding observation.
		q.obs.Attempts = append(q.obs.Attempts, Attempt{ID: "a2", Ticket: w.Ticket, Holder: w.ID, Phase: "RUNNING", Generation: "1", Live: true, LeaseExpires: now.Add(time.Hour)})
		tick(t, d)
		if w.State != "RUNNING" || w.KillReason != "" || gone(w.PID) || d.Running() != 1 || !has(kinds(t, d), "alert") {
			t.Fatalf("reclaimed worker: state %q reason %q running %d events %v", w.State, w.KillReason, d.Running(), kinds(t, d))
		}
		tick(t, d)
		if w.State != "RUNNING" || gone(w.PID) {
			t.Fatalf("a worker with a live attempt was stopped again: state %q", w.State)
		}
	})
	t.Run("after the signal", func(t *testing.T) {
		now := time.Date(2026, 10, 8, 9, 0, 0, 0, time.UTC)
		q := &refusingQueue{fakeQueue: &fakeQueue{}}
		d, w := leaseRig(t, q, &now, now.Add(time.Hour), nil)
		w.State, w.KillReason, w.KillDeadline = "KILLING", "LEASE_EXPIRED", time.Now().Add(time.Minute) // TERM already sent
		tick(t, d)
		if w.State != "KILLING" || !gone(w.PID) || d.Running() != 0 {
			t.Fatalf("a signalled stop was cancelled: state %q running %d", w.State, d.Running())
		}
	})
}

func TestCALV0191_ExpiredLeaseGraceConfig(t *testing.T) {
	c := testConfig(t, "exit 0")
	raw, _ := json.Marshal(c)
	if strings.Contains(string(raw), "expiredLeaseGrace") {
		t.Fatalf("an absent grace is written: %s", raw)
	}
	got, err := DecodeConfig(raw)
	if err != nil || got.Roles[0].ExpiredLeaseGrace() != 600*time.Second || (*Role)(nil).ExpiredLeaseGrace() != 600*time.Second {
		t.Fatalf("absent grace: %v", err)
	}
	for _, n := range []int{0, 1, 86400} {
		c := testConfig(t, "exit 0")
		c.Roles[0].ExpiredLeaseGraceSeconds = intPtr(n)
		raw, _ := json.Marshal(c)
		got, err := DecodeConfig(raw)
		if err != nil || got.Roles[0].ExpiredLeaseGrace() != time.Duration(n)*time.Second {
			t.Errorf("grace %d: %v", n, err)
		}
	}
	for _, n := range []int{-1, 86401} {
		c := testConfig(t, "exit 0")
		c.Roles[0].ExpiredLeaseGraceSeconds = intPtr(n)
		raw, _ := json.Marshal(c)
		if _, err := DecodeConfig(raw); err == nil || !strings.Contains(err.Error(), "expiredLeaseGraceSeconds") {
			t.Errorf("grace %d accepted: %v", n, err)
		}
	}
	c.Roles[0].ExpiredLeaseGraceSeconds = intPtr(60)
	raw, _ = json.Marshal(c)
	raw = []byte(strings.Replace(string(raw), `"expiredLeaseGraceSeconds"`, `"expiredLeaseGrace"`, 1))
	if !strings.Contains(string(raw), `"expiredLeaseGrace":60`) {
		t.Fatalf("renamed key missing: %s", raw)
	}
	if _, err := DecodeConfig(raw); err == nil {
		t.Error("unknown role key accepted")
	}
}
