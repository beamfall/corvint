//go:build darwin || linux

package dispatch

import (
	"os"
	"reflect"
	"testing"
	"time"
)

// sleepRig is leaseRig with an injected monotonic clock that the test
// advances separately from the wall clock (ATR-V0-063).
func sleepRig(t *testing.T, now *time.Time, mono *time.Duration, lease time.Time) (*Dispatcher, *Worker, *refusingQueue) {
	t.Helper()
	q := &refusingQueue{fakeQueue: &fakeQueue{}}
	d, w := leaseRig(t, q, now, lease, func(c *Config) { c.Roles[0].ExpiredLeaseGraceSeconds = intPtr(120) })
	d.Mono = func() time.Duration { return *mono }
	return d, w, q
}

func sleptAlerts(t *testing.T, d *Dispatcher) []Event {
	t.Helper()
	var out []Event
	for _, e := range eventsOf(t, d, "alert") {
		if e.Detail["hostSlept"] != "" {
			out = append(out, e)
		}
	}
	return out
}

// ATR-V0-063, ATR-V0-064: a three-hour suspend is recorded once as a gap
// and an alert, the wall and idle caps and an expired lease's grace exclude
// it, and the grace counts again from the wake.
func TestATRV0063_SuspendIsRecordedAndExcludedFromTimers(t *testing.T) {
	start := time.Date(2026, 10, 17, 9, 0, 0, 0, time.UTC)
	now, mono := start, time.Duration(0)
	lease := start.Add(time.Minute)
	d, w, q := sleepRig(t, &now, &mono, lease)

	now, mono = start.Add(3*time.Hour+10*time.Second), 10*time.Second
	tick(t, d)
	tick(t, d)
	if w.State == "KILLING" || d.Running() != 1 || len(q.requests) != 0 {
		t.Fatalf("a timer counted the suspend: state %q reason %q requests %v", w.State, w.KillReason, q.requests)
	}
	alerts := eventsWith(t, d, "alert", "hostSlept", "10800")
	if len(alerts) != 1 || alerts[0].Detail["from"] != start.Format(time.RFC3339) || alerts[0].Detail["to"] != start.Add(3*time.Hour).Format(time.RFC3339) {
		t.Fatalf("suspend alerts %+v", alerts)
	}
	rec, err := ReadHostSleep(HostSleepPath(d.dir))
	want := []SleepGap{{From: start.Format(time.RFC3339), To: start.Add(3 * time.Hour).Format(time.RFC3339), Seconds: 10800}}
	if err != nil || rec.Program != "lease" || !reflect.DeepEqual(rec.Gaps, want) {
		t.Fatalf("record %+v %v", rec, err)
	}

	now, mono = now.Add(111*time.Second), mono+111*time.Second // 1s past the grace counted from the wake
	tick(t, d)
	if !reflect.DeepEqual(q.reaped, []string{"a1"}) || w.KillReason != "LEASE_EXPIRED" {
		t.Fatalf("the grace did not resume after the wake: reaped %v reason %q", q.reaped, w.KillReason)
	}
	if n := len(sleptAlerts(t, d)); n != 1 {
		t.Fatalf("%d suspend alerts after awake ticks", n)
	}
}

// ATR-V0-063: when the monotonic clock keeps pace with the wall clock no gap
// is recorded and the wall cap stops the worker as before.
func TestATRV0063_NoGapLeavesTimersUnchanged(t *testing.T) {
	start := time.Date(2026, 10, 17, 9, 0, 0, 0, time.UTC)
	now, mono := start, time.Duration(0)
	d, w, _ := sleepRig(t, &now, &mono, start.Add(24*time.Hour))
	now, mono = start.Add(7201*time.Second), 7201*time.Second
	tick(t, d)
	if w.State != "KILLING" || w.KillReason != "WALL" {
		t.Fatalf("state %q reason %q, want the wall cap", w.State, w.KillReason)
	}
	if _, err := os.Stat(HostSleepPath(d.dir)); !os.IsNotExist(err) || len(sleptAlerts(t, d)) != 0 {
		t.Fatalf("a gap was recorded without a suspend: %v", err)
	}
}

// ATR-V0-063: a newer-version record is kept unchanged while the gap is
// still excluded; a malformed one is replaced.
func TestATRV0063_RecordVersions(t *testing.T) {
	for name, body := range map[string]string{
		"newer":     `{"profile":"taskman-dispatch-host-sleep/1"}`,
		"malformed": `{"profile":"taskman-dispatch-host-sleep/0","program":"lease","gaps":[]}`,
	} {
		t.Run(name, func(t *testing.T) {
			start := time.Date(2026, 10, 17, 9, 0, 0, 0, time.UTC)
			now, mono := start, time.Duration(0)
			q := &refusingQueue{fakeQueue: &fakeQueue{}}
			d, w := leaseRig(t, q, &now, start.Add(24*time.Hour), nil)
			path := HostSleepPath(d.dir)
			if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
				t.Fatal(err)
			}
			d.sleep, d.lastNow = nil, time.Time{} // as a fresh dispatcher
			d.Mono = func() time.Duration { return mono }
			tick(t, d)
			now, mono = start.Add(3*time.Hour), time.Second
			tick(t, d)
			if w.State == "KILLING" || len(sleptAlerts(t, d)) != 1 {
				t.Fatalf("state %q reason %q", w.State, w.KillReason)
			}
			raw, _ := os.ReadFile(path)
			_, err := ReadHostSleep(path)
			if name == "newer" && string(raw) != body || name == "malformed" && err != nil {
				t.Fatalf("record %q %v", raw, err)
			}
		})
	}
}

// ATR-V0-064, V1-1634: the lease keeper's idle check (CAL-V0-312) counts a
// recorded suspend as no idle time. A quiet worker kept before a three-hour
// suspend is kept again at the first tick after the wake, not stopped being
// kept as idle; idle time that really passes after the wake still counts.
func TestATRV0064_KeeperIdleCheckExcludesSuspend(t *testing.T) {
	start := time.Date(2026, 10, 17, 9, 0, 0, 0, time.UTC)
	now, mono := start, time.Duration(0)
	q := &keeperQueue{fakeQueue: &fakeQueue{}, now: &now}
	d, w := keeperRig(t, q, q.fakeQueue, &now, start.Add(5*time.Minute), &KeepLease{IntervalSeconds: 30, LeaseMinutes: 5, MaxIdleSeconds: 120})
	d.Mono = func() time.Duration { return mono }

	now, mono = start.Add(60*time.Second), 60*time.Second
	tick(t, d)
	if len(q.ids) != 1 || w.Keeper.State != keeperKeeping {
		t.Fatalf("awake: %d writes, keeper %+v", len(q.ids), w.Keeper)
	}

	// Three hours pass on the wall clock and 30 s on the monotonic clock.
	now, mono = now.Add(3*time.Hour+30*time.Second), mono+30*time.Second
	tick(t, d)
	if n := len(sleptAlerts(t, d)); n != 1 {
		t.Fatalf("%d suspend alerts, want one", n)
	}
	if w.Keeper.State != keeperKeeping || len(q.ids) != 2 || count(keeperActions(t, d), "idle:") != 0 {
		t.Fatalf("the suspend counted as idle time: %d writes, keeper %+v, events %v", len(q.ids), w.Keeper, keeperActions(t, d))
	}
	if w.State != "RUNNING" || len(q.reaped) != 0 {
		t.Fatalf("worker %q after the wake, reaped %v", w.State, q.reaped)
	}

	// 90 s of real idleness so far; 40 s more passes the 120 s bound.
	now, mono = now.Add(40*time.Second), mono+40*time.Second
	tick(t, d)
	if w.Keeper.State != keeperIdle || count(keeperActions(t, d), "idle:") != 1 {
		t.Fatalf("real idle time after the wake was not counted: keeper %+v, events %v", w.Keeper, keeperActions(t, d))
	}
}

// ATR-V0-064, V1-1634: the CAL-V0-104 exit recovery of an ended worker's
// attempt keeps no grace and so excludes no gap. A hand-off refused before a
// suspend is retried at the first tick after the wake rather than after a
// further backoff, and an attempt whose lease ran out during the suspend is
// reaped at once, because no worker is left to use the time.
func TestATRV0064_ExitRecoveryActsAtWake(t *testing.T) {
	for _, tc := range []struct {
		name      string
		lease     time.Duration
		released  []string
		reaped    []string
		recovered string
	}{
		{"lease-expired-in-gap", time.Hour, []string{"refused:a1"}, []string{"a1"}, "REAPED"},
		{"lease-live-after-wake", 24 * time.Hour, []string{"refused:a1", "a1"}, nil, "RELEASED"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			start := time.Date(2026, 10, 17, 9, 0, 0, 0, time.UTC)
			now, mono := start, time.Duration(0)
			q := &refusingQueue{fakeQueue: &fakeQueue{}, releaseFails: 1}
			d := exitRig(t, q, start.Add(tc.lease), &now, nil)
			d.Mono = func() time.Duration { return mono }
			tick(t, d) // first refusal: next retry is one backoff away
			if !reflect.DeepEqual(q.released, []string{"refused:a1"}) {
				t.Fatalf("released %v", q.released)
			}
			now, mono = now.Add(3*time.Hour+time.Second), mono+time.Second
			tick(t, d)
			if n := len(sleptAlerts(t, d)); n != 1 {
				t.Fatalf("%d suspend alerts, want one", n)
			}
			if !reflect.DeepEqual(q.released, tc.released) || !reflect.DeepEqual(q.reaped, tc.reaped) {
				t.Fatalf("at the wake: released %v reaped %v, want %v and %v", q.released, q.reaped, tc.released, tc.reaped)
			}
			kind := map[string]string{"REAPED": "reaped", "RELEASED": "handoff"}[tc.recovered]
			if got := recoveryOf(t, d, kind); !reflect.DeepEqual(got, []string{tc.recovered}) || len(d.recoveries) != 0 {
				t.Fatalf("recoveries %v (%s), pending %v", got, kind, d.recoveries)
			}
		})
	}
}

// CAL-V0-361, V1-1634: the dispatcher's heal reaps an expired lease that no
// worker of this dispatcher holds on the first tick after a wake, because
// the ATR-V0-064 grace belongs to a running worker's lease and no gap
// extends another holder's. The running worker's own lease, expired inside
// the same gap, stays within its grace.
func TestCALV0361_UnownedExpiredLeaseIsReapedAtWake(t *testing.T) {
	start := time.Date(2026, 10, 17, 9, 0, 0, 0, time.UTC)
	now, mono := start, time.Duration(0)
	d, w, q := sleepRig(t, &now, &mono, start.Add(time.Minute))
	q.obs.Attempts = append(q.obs.Attempts, Attempt{ID: "a2", Ticket: "t2", Holder: "stranger", Phase: "RUNNING", Generation: "1", Live: true, LeaseExpires: start.Add(2 * time.Minute)})
	now, mono = start.Add(3*time.Hour+10*time.Second), 10*time.Second
	tick(t, d)
	if n := len(sleptAlerts(t, d)); n != 1 {
		t.Fatalf("%d suspend alerts, want one", n)
	}
	if !reflect.DeepEqual(q.reaped, []string{"a2"}) || w.State == "KILLING" {
		t.Fatalf("at the wake: reaped %v, worker %q (%q)", q.reaped, w.State, w.KillReason)
	}
}
