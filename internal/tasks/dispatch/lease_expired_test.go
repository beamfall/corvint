//go:build darwin || linux

package dispatch

import (
	"encoding/json"
	"io"
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
	if !reflect.DeepEqual(q.reaped, []string{"a1"}) || !reflect.DeepEqual(q.requests, []string{requestID("reap", "a1", "3", "lease-expired")}) {
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
