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

// monoGap records one suspend gap of length gap starting at wall time from,
// as an earlier tick of this process would have.
func monoGap(d *Dispatcher, from time.Time, gap time.Duration) {
	d.loadSleep()
	from = from.Round(0).UTC().Truncate(time.Second)
	d.sleep.Gaps = append(d.sleep.Gaps, SleepGap{From: from.Format(time.RFC3339), To: from.Add(gap).Format(time.RFC3339), Seconds: int(gap / time.Second)})
}

// ATR-V0-064: timestamps that carry Go's monotonic reading already exclude a
// suspend, so a recorded gap inside them is not subtracted a second time
// from the wall cap, the idle cap or the lease keeper's idle check.
func TestATRV0064_MonotonicTimestampsAreNotDiscountedTwice(t *testing.T) {
	for _, tc := range []struct {
		name    string
		after   time.Duration
		reason  string
		wall    int
		idle    int
		gapFrom time.Duration
	}{
		{"wall", 2*time.Hour + time.Second, "WALL", 7200, 100000, 10 * time.Minute},
		{"idle", time.Hour + time.Second, "IDLE", 100000, 3600, 10 * time.Minute},
	} {
		t.Run(tc.name, func(t *testing.T) {
			start := time.Now() // carries a monotonic reading
			if !hasMonotonic(start) {
				t.Fatal("time.Now has no monotonic reading")
			}
			now := start
			q := &refusingQueue{fakeQueue: &fakeQueue{}}
			d, w := leaseRig(t, q, &now, start.Add(48*time.Hour), func(c *Config) {
				c.Roles[0].WallSeconds, c.Roles[0].IdleSeconds = tc.wall, tc.idle
			})
			monoGap(d, start.Add(tc.gapFrom), 30*time.Minute)
			now = start.Add(tc.after) // monotonic and wall advance together
			tick(t, d)
			if w.State != "KILLING" || w.KillReason != tc.reason {
				t.Fatalf("state %q reason %q, want %s: the recorded gap was subtracted from a monotonic difference", w.State, w.KillReason, tc.reason)
			}
		})
	}
	t.Run("keeper", func(t *testing.T) {
		start := time.Now()
		now := start
		q := &keeperQueue{fakeQueue: &fakeQueue{}, now: &now}
		d, w := keeperRig(t, q, q.fakeQueue, &now, start.Add(5*time.Minute), &KeepLease{IntervalSeconds: 30, LeaseMinutes: 5, MaxIdleSeconds: 120})
		tick(t, d)
		monoGap(d, start.Add(10*time.Second), 100*time.Second)
		now = start.Add(150 * time.Second)
		tick(t, d)
		if w.Keeper == nil || w.Keeper.State != keeperIdle {
			t.Fatalf("keeper %+v, want idle after 150 awake seconds over maxIdleSeconds 120", w.Keeper)
		}
	})
	// Without monotonic readings the recorded gap is still excluded.
	d := &Dispatcher{}
	d.sleep = &HostSleep{Profile: HostSleepProfile, Program: "p"}
	base := time.Date(2026, 10, 17, 9, 0, 0, 0, time.UTC)
	monoGap(d, base, time.Hour)
	if got := d.awake(base, base.Add(90*time.Minute)); got != 30*time.Minute {
		t.Fatalf("wall awake %s, want 30m", got)
	}
}
