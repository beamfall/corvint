//go:build darwin || linux

package dispatch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// keeperQueue is a fake store with the CAL-V0-311 LeaseKeeper capability:
// a renew moves the observed lease to now plus its minutes, a heartbeat
// moves nothing, and either can be fenced or fail. Like the native store, a
// request ID already committed replays: it reports replayed and acts on
// nothing (CAL-V0-318).
type keeperQueue struct {
	*fakeQueue
	now           *time.Time
	ids           []string
	renews, beats []string
	committed     map[string]bool
	fence         bool
	fail          error
}

func (q *keeperQueue) write(a Attempt, id string) (bool, error) {
	q.ids = append(q.ids, id)
	if q.fence {
		return false, fmt.Errorf("%w: REVISION_CONFLICT FENCED", ErrFenced)
	}
	if q.fail != nil {
		return false, q.fail
	}
	if q.committed[id] {
		return true, nil
	}
	if q.committed == nil {
		q.committed = map[string]bool{}
	}
	q.committed[id] = true
	return false, nil
}

func (q *keeperQueue) Renew(_ context.Context, a Attempt, minutes int, id string) (bool, error) {
	if replayed, err := q.write(a, id); err != nil || replayed {
		return replayed, err
	}
	q.renews = append(q.renews, a.ID+"@"+a.Generation)
	for i := range q.obs.Attempts {
		if q.obs.Attempts[i].ID == a.ID {
			q.obs.Attempts[i].LeaseExpires = q.now.Add(time.Duration(minutes) * time.Minute)
		}
	}
	return false, nil
}

func (q *keeperQueue) Heartbeat(_ context.Context, a Attempt, id string) (bool, error) {
	if replayed, err := q.write(a, id); err != nil || replayed {
		return replayed, err
	}
	q.beats = append(q.beats, a.ID+"@"+a.Generation)
	return false, nil
}

// keeperRig launches one real, quiet worker on t1 under a fake clock whose
// role declares kl (nil: no keeper), with idle and wall caps beyond the
// test's clock moves, and records that it holds live attempt a1 at
// generation 3 whose lease expires at lease.
func keeperRig(t *testing.T, q Queue, fq *fakeQueue, now *time.Time, lease time.Time, kl *KeepLease) (*Dispatcher, *Worker) {
	t.Helper()
	c := testConfig(t, "exec sleep 300")
	c.GlobalCap = 1
	c.Backoff.CooldownSeconds = 3600
	c.Roles[0].IdleSeconds, c.Roles[0].WallSeconds = 3600, 7200
	c.Roles[0].KeepLease = kl
	fq.obs.Tickets = []Ticket{ticket("t1", "P1", 1)}
	d, err := Open("keep", c, q, io.Discard)
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
	fq.obs.Attempts = []Attempt{{ID: "a1", Ticket: w.Ticket, Holder: w.ID, Phase: "RUNNING", Generation: "3", Live: true, LeaseExpires: lease}}
	return d, w
}

// keeperActions lists the action of each lease-keeper event, in order.
func keeperActions(t *testing.T, d *Dispatcher) []string {
	t.Helper()
	events, err := ReadEvents(d.dir, 1000)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range events {
		if e.Kind == "lease-keeper" {
			out = append(out, e.Detail["action"]+":"+e.Detail["reason"])
		}
	}
	return out
}

func TestCALV0311_KeepLeaseConfigIsClosedAndBounded(t *testing.T) {
	decode := func(keep string) error {
		c := testConfig(t, "exit 0")
		raw, _ := json.Marshal(c)
		s := strings.Replace(string(raw), `"idleSeconds":30`, `"idleSeconds":30,"keepLease":`+keep, 1)
		_, err := DecodeConfig([]byte(s))
		return err
	}
	if err := decode(`{"intervalSeconds":60,"leaseMinutes":10,"maxIdleSeconds":900}`); err != nil {
		t.Fatalf("valid keepLease refused: %v", err)
	}
	for name, keep := range map[string]string{
		"unknown member":        `{"intervalSeconds":60,"leaseMinutes":10,"maxIdleSeconds":900,"extra":1}`,
		"missing interval":      `{"leaseMinutes":10,"maxIdleSeconds":900}`,
		"missing lease":         `{"intervalSeconds":60,"maxIdleSeconds":900}`,
		"missing idle":          `{"intervalSeconds":60,"leaseMinutes":10}`,
		"interval too short":    `{"intervalSeconds":29,"leaseMinutes":10,"maxIdleSeconds":900}`,
		"interval too long":     `{"intervalSeconds":3601,"leaseMinutes":1440,"maxIdleSeconds":900}`,
		"lease too short":       `{"intervalSeconds":30,"leaseMinutes":4,"maxIdleSeconds":900}`,
		"lease too long":        `{"intervalSeconds":60,"leaseMinutes":1441,"maxIdleSeconds":900}`,
		"idle too short":        `{"intervalSeconds":60,"leaseMinutes":10,"maxIdleSeconds":29}`,
		"idle too long":         `{"intervalSeconds":60,"leaseMinutes":10,"maxIdleSeconds":86401}`,
		"lease within 2 rounds": `{"intervalSeconds":150,"leaseMinutes":5,"maxIdleSeconds":900}`,
		"not an object":         `60`,
	} {
		if err := decode(keep); err == nil {
			t.Errorf("%s: accepted %s", name, keep)
		}
	}
}

// TestCALV0312_QuietWorkerKeepsItsAttemptAcrossLeasePeriods is acceptance 1
// and 4: a live worker that makes no queue call for four lease periods keeps
// its attempt. Each due tick writes once; a lease that still covers two
// intervals gets only the heartbeat, and a shorter one is renewed.
func TestCALV0312_QuietWorkerKeepsItsAttemptAcrossLeasePeriods(t *testing.T) {
	now := time.Date(2026, 10, 17, 9, 0, 0, 0, time.UTC)
	q := &keeperQueue{fakeQueue: &fakeQueue{}, now: &now}
	d, w := keeperRig(t, q, q.fakeQueue, &now, now.Add(10*time.Minute), &KeepLease{IntervalSeconds: 60, LeaseMinutes: 10, MaxIdleSeconds: 3600})
	start := now
	ticks := 0
	for now.Sub(start) <= 40*time.Minute {
		tick(t, d)
		ticks++
		a := q.obs.Attempts[0]
		if !a.Live || !a.LeaseExpires.After(now) || w.State != "RUNNING" {
			t.Fatalf("at +%s the attempt %+v, worker %s", now.Sub(start), a, w.State)
		}
		if a.LeaseExpires.Sub(now) < time.Minute {
			t.Fatalf("at +%s the lease covers only %s", now.Sub(start), a.LeaseExpires.Sub(now))
		}
		now = now.Add(time.Minute)
	}
	if len(q.ids) != ticks || len(q.renews)+len(q.beats) != ticks {
		t.Fatalf("%d writes (%d renews, %d heartbeats) over %d due ticks", len(q.ids), len(q.renews), len(q.beats), ticks)
	}
	if len(q.renews) < 4 || len(q.beats) < 30 {
		t.Fatalf("renews %d heartbeats %d", len(q.renews), len(q.beats))
	}
	seen := map[string]bool{}
	for _, id := range q.ids {
		if seen[id] {
			t.Fatalf("request %s repeated", id)
		}
		seen[id] = true
	}
	if len(q.reaped) != 0 || has(kinds(t, d), "lease-expired") || has(kinds(t, d), "alert") {
		t.Fatalf("reaped %v events %v", q.reaped, kinds(t, d))
	}
	k := w.Keeper
	if k.State != keeperKeeping || k.Attempt != "a1" || k.Generation != "3" || k.Renewals != len(q.renews) || k.Heartbeats != len(q.beats) || !k.Due.Equal(k.LastKept.Add(time.Minute)) {
		t.Fatalf("keeper %+v", k)
	}
	acts := keeperActions(t, d)
	if count(acts, "bound:") != 1 || count(acts, "renewed:") != len(q.renews) || count(acts, "heartbeat:") != len(q.beats) {
		t.Fatalf("keeper events %v", acts)
	}
	// Between due times nothing is written.
	now = k.LastKept.Add(30 * time.Second)
	tick(t, d)
	if len(q.ids) != ticks {
		t.Fatalf("a tick before the due time wrote: %d writes", len(q.ids))
	}
}

// TestCALV0313_CoveredLeaseIsNotRenewed is acceptance 4: while the lease
// covers more than two intervals, each due write is the heartbeat alone,
// under a deterministic request ID; the renewal comes once it does not.
func TestCALV0313_CoveredLeaseIsNotRenewed(t *testing.T) {
	now := time.Date(2026, 10, 17, 9, 0, 0, 0, time.UTC)
	lease := now.Add(5 * time.Minute)
	q := &keeperQueue{fakeQueue: &fakeQueue{}, now: &now}
	d, _ := keeperRig(t, q, q.fakeQueue, &now, lease, &KeepLease{IntervalSeconds: 60, LeaseMinutes: 30, MaxIdleSeconds: 3600})
	for i := 0; i < 3; i++ { // 5, 4 and 3 minutes left: heartbeat only
		tick(t, d)
		now = now.Add(time.Minute)
	}
	if len(q.renews) != 0 || len(q.beats) != 3 || !q.obs.Attempts[0].LeaseExpires.Equal(lease) {
		t.Fatalf("renews %v beats %v lease %s", q.renews, q.beats, q.obs.Attempts[0].LeaseExpires)
	}
	if q.ids[0] != requestID("keep", "a1", "3", "heartbeat", now.Add(-3*time.Minute).Format(time.RFC3339Nano)) {
		t.Fatalf("first request %s", q.ids[0])
	}
	tick(t, d) // 2 minutes left: no longer more than two intervals
	if len(q.renews) != 1 || !q.obs.Attempts[0].LeaseExpires.Equal(now.Add(30*time.Minute)) {
		t.Fatalf("renews %v lease %s", q.renews, q.obs.Attempts[0].LeaseExpires)
	}
	if q.ids[3] != requestID("keep", "a1", "3", "renew", now.Format(time.RFC3339Nano)) {
		t.Fatalf("renew request %s", q.ids[3])
	}
}

// TestCALV0312_IdleWorkerIsNotKeptPastMaxIdle is acceptance 2: with no
// supervision activity for over maxIdleSeconds the keeper writes nothing,
// recording one idle event however many ticks pass; output resumes it; and
// an idle worker's lease runs out and is reaped as before (CAL-V0-191).
func TestCALV0312_IdleWorkerIsNotKeptPastMaxIdle(t *testing.T) {
	now := time.Date(2026, 10, 17, 9, 0, 0, 0, time.UTC)
	start := now
	q := &keeperQueue{fakeQueue: &fakeQueue{}, now: &now}
	d, w := keeperRig(t, q, q.fakeQueue, &now, now.Add(5*time.Minute), &KeepLease{IntervalSeconds: 30, LeaseMinutes: 5, MaxIdleSeconds: 120})
	for _, at := range []int{0, 60, 120} { // idle for at most maxIdleSeconds
		now = start.Add(time.Duration(at) * time.Second)
		tick(t, d)
	}
	if len(q.ids) != 3 {
		t.Fatalf("%d writes while active", len(q.ids))
	}
	for _, at := range []int{121, 150, 180, 240} {
		now = start.Add(time.Duration(at) * time.Second)
		tick(t, d)
	}
	if len(q.ids) != 3 || w.Keeper.State != keeperIdle {
		t.Fatalf("idle worker kept: %d writes, keeper %+v", len(q.ids), w.Keeper)
	}
	if acts := keeperActions(t, d); count(acts, "idle:") != 1 {
		t.Fatalf("keeper events %v", acts)
	}
	// Output growth is the CAL-V0-218 activity signal: the keeper resumes.
	f, err := os.OpenFile(filepath.Join(d.workerDir(w.ID), "stdout.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString("progress\n")
	f.Close()
	now = start.Add(250 * time.Second)
	tick(t, d)
	if len(q.ids) != 4 || w.Keeper.State != keeperKeeping || !w.LastActive.Equal(now) {
		t.Fatalf("resumed: %d writes, keeper %+v, lastActive %s", len(q.ids), w.Keeper, w.LastActive)
	}
	if acts := keeperActions(t, d); count(acts, "resumed:") != 1 {
		t.Fatalf("keeper events %v", acts)
	}
	// Quiet again: once idle the keeper writes nothing more, so the lease it
	// last left runs out and is reaped.
	idleWrites := -1
	for limit := now.Add(30 * time.Minute); w.State == "RUNNING" && now.Before(limit); now = now.Add(time.Minute) {
		tick(t, d)
		if w.Keeper.State == keeperIdle && idleWrites < 0 {
			idleWrites = len(q.ids)
		}
	}
	if idleWrites < 0 || len(q.ids) != idleWrites || w.Keeper.LastKept.Sub(w.LastActive) > 120*time.Second {
		t.Fatalf("writes %d after idle at %d, last kept %s, last active %s", len(q.ids), idleWrites, w.Keeper.LastKept, w.LastActive)
	}
	if len(q.reaped) != 1 || w.State != "KILLING" || w.KillReason != "LEASE_EXPIRED" {
		t.Fatalf("reaped %v worker %s %s", q.reaped, w.State, w.KillReason)
	}
}

// TestCALV0314_FencedOrMovedAttemptStopsOnce is acceptance 3: a fenced
// write stops the binding with one stop event and no alert, and nothing is
// written for it again; a changed generation or an attempt no longer held
// live stops it the same way, and a new live attempt the worker holds is
// bound afresh. A failure that is not a fence alerts once per episode and
// retries each tick.
func TestCALV0314_FencedOrMovedAttemptStopsOnce(t *testing.T) {
	now := time.Date(2026, 10, 17, 9, 0, 0, 0, time.UTC)
	q := &keeperQueue{fakeQueue: &fakeQueue{}, now: &now}
	d, w := keeperRig(t, q, q.fakeQueue, &now, now.Add(10*time.Minute), &KeepLease{IntervalSeconds: 60, LeaseMinutes: 10, MaxIdleSeconds: 3600})
	step := func(n int) {
		for i := 0; i < n; i++ {
			tick(t, d)
			now = now.Add(time.Minute)
		}
	}
	step(1)
	q.fence = true
	step(4)
	if len(q.ids) != 2 || w.Keeper.State != keeperStopped || w.Keeper.Reason != keeperFenced {
		t.Fatalf("fenced: %d writes keeper %+v", len(q.ids), w.Keeper)
	}
	if acts := keeperActions(t, d); count(acts, "stopped:FENCED") != 1 || has(kinds(t, d), "alert") {
		t.Fatalf("fenced events %v kinds %v", acts, kinds(t, d))
	}

	// The worker re-claims at generation 4: a new binding, kept at once.
	q.fence = false
	q.obs.Attempts[0].Generation = "4"
	step(1)
	if len(q.ids) != 3 || w.Keeper.State != keeperKeeping || w.Keeper.Generation != "4" || q.beats[len(q.beats)-1] != "a1@4" {
		t.Fatalf("rebound: %d writes keeper %+v beats %v", len(q.ids), w.Keeper, q.beats)
	}
	// The generation moves again: the old binding stops once, never written.
	q.obs.Attempts[0].Generation = "5"
	step(1)
	if acts := keeperActions(t, d); count(acts, "stopped:GENERATION_CHANGED") != 1 || q.beats[len(q.beats)-1] != "a1@5" {
		t.Fatalf("generation change events %v beats %v", acts, q.beats)
	}
	for _, b := range q.beats[1:] {
		if b == "a1@3" {
			t.Fatalf("wrote the fenced binding again: %v", q.beats)
		}
	}
	// A failure that is not a fence: one alert, a retry each tick.
	q.fail = errors.New("LOCK_TIMEOUT")
	writes := len(q.ids)
	step(3)
	if len(q.ids) != writes+3 || !w.Keeper.Failing || count(kinds(t, d), "alert") != 1 || w.Keeper.State != keeperKeeping {
		t.Fatalf("failing: writes %d keeper %+v kinds %v", len(q.ids)-writes, w.Keeper, kinds(t, d))
	}
	q.fail = nil
	step(1)
	if w.Keeper.Failing || count(kinds(t, d), "alert") != 1 {
		t.Fatalf("recovered keeper %+v", w.Keeper)
	}
	// The attempt is no longer live: one stop, then nothing.
	q.obs.Attempts[0].Live, q.obs.Attempts[0].Phase = false, "SUBMITTED"
	writes = len(q.ids)
	step(3)
	if len(q.ids) != writes || w.Keeper.State != keeperStopped || w.Keeper.Reason != keeperNotHeld || count(keeperActions(t, d), "stopped:NOT_HELD") != 1 {
		t.Fatalf("not held: %d writes keeper %+v events %v", len(q.ids)-writes, w.Keeper, keeperActions(t, d))
	}
}

// TestCALV0314_QueueWithoutKeeperStopsOnce: a queue that cannot renew or
// heartbeat stops the binding UNSUPPORTED with one alert.
func TestCALV0314_QueueWithoutKeeperStopsOnce(t *testing.T) {
	now := time.Date(2026, 10, 17, 9, 0, 0, 0, time.UTC)
	q := &fakeQueue{}
	d, w := keeperRig(t, q, q, &now, now.Add(10*time.Minute), &KeepLease{IntervalSeconds: 60, LeaseMinutes: 10, MaxIdleSeconds: 3600})
	for i := 0; i < 3; i++ {
		tick(t, d)
		now = now.Add(time.Minute)
	}
	if w.Keeper.State != keeperStopped || w.Keeper.Reason != keeperNoKeeper || count(kinds(t, d), "alert") != 1 || count(keeperActions(t, d), "stopped:UNSUPPORTED") != 1 {
		t.Fatalf("keeper %+v kinds %v", w.Keeper, kinds(t, d))
	}
}

// TestCALV0312_ExitedWorkerIsNotKept: a worker whose tree has ended is
// never kept, even while its attempt is still live.
func TestCALV0312_ExitedWorkerIsNotKept(t *testing.T) {
	now := time.Date(2026, 10, 17, 9, 0, 0, 0, time.UTC)
	q := &keeperQueue{fakeQueue: &fakeQueue{}, now: &now}
	c := &KeepLease{IntervalSeconds: 60, LeaseMinutes: 10, MaxIdleSeconds: 3600}
	d, w := keeperRig(t, q, q.fakeQueue, &now, now.Add(10*time.Minute), c)
	tick(t, d)
	if len(q.ids) != 1 {
		t.Fatalf("%d writes for a live worker", len(q.ids))
	}
	syscall.Kill(-w.PID, syscall.SIGKILL)
	syscall.Kill(w.PID, syscall.SIGKILL)
	if !gone(w.PID) {
		t.Fatal("worker survived SIGKILL")
	}
	now = now.Add(time.Minute)
	tick(t, d)
	if len(q.ids) != 1 {
		t.Fatalf("an exited worker was kept: %v", q.ids)
	}
}

// TestCALV0317_UnobservedTreeIsNotKept: when the process table cannot be
// read after the worker exited, its recorded tree still names a live
// leader, but the keeper writes nothing for a tree this tick did not
// observe; the tick's alert records the failed observation. Once the
// table reads again the exited worker ends and is never kept.
func TestCALV0317_UnobservedTreeIsNotKept(t *testing.T) {
	now := time.Date(2026, 10, 17, 9, 0, 0, 0, time.UTC)
	q := &keeperQueue{fakeQueue: &fakeQueue{}, now: &now}
	c := &KeepLease{IntervalSeconds: 60, LeaseMinutes: 5, MaxIdleSeconds: 3600}
	d, w := keeperRig(t, q, q.fakeQueue, &now, now.Add(90*time.Second), c)
	tick(t, d)
	if len(q.renews) != 1 {
		t.Fatalf("%d renews for a live worker", len(q.renews))
	}
	syscall.Kill(-w.PID, syscall.SIGKILL)
	syscall.Kill(w.PID, syscall.SIGKILL)
	if !gone(w.PID) {
		t.Fatal("worker survived SIGKILL")
	}
	prev := superviseProcs
	defer func() { superviseProcs = prev }()
	superviseProcs = func() (map[int]proc, error) { return nil, errors.New("ps: resource unavailable") }
	for i := 0; i < 6; i++ { // past the lease the first renewal left
		now = now.Add(time.Minute)
		tick(t, d)
		if w.State != "RUNNING" || len(w.Members) == 0 || !leaderAlive(w) {
			t.Fatalf("the unobserved worker's recorded tree changed: %s %v", w.State, w.Members)
		}
	}
	if len(q.ids) != 1 {
		t.Fatalf("an unobserved, exited worker was kept: %v", q.ids)
	}
	if count(kinds(t, d), "alert") == 0 {
		t.Fatalf("the failed process observation is not visible: %v", kinds(t, d))
	}
	superviseProcs = prev
	now = now.Add(time.Minute)
	tick(t, d)
	if len(q.ids) != 1 {
		t.Fatalf("an exited worker was kept once observed: %v", q.ids)
	}
}

// TestCALV0318_ReplayedRenewalAfterRestartWritesAgain is the lost-save
// recovery: interval 60s, lease 5 minutes. A due renewal at 12:00 moves
// the lease to 12:05, then the dispatcher loses the save of its due time
// and next runs at 12:04:30. The repeated request replays without moving
// the lease, so the keeper renews once more under a request ID of this
// time; the lease never lapses and the next due time is one interval on.
func TestCALV0318_ReplayedRenewalAfterRestartWritesAgain(t *testing.T) {
	now := time.Date(2026, 10, 17, 11, 59, 0, 0, time.UTC)
	q := &keeperQueue{fakeQueue: &fakeQueue{}, now: &now}
	c := &KeepLease{IntervalSeconds: 60, LeaseMinutes: 5, MaxIdleSeconds: 3600}
	d, w := keeperRig(t, q, q.fakeQueue, &now, now.Add(3*time.Minute), c)
	tick(t, d) // 11:59, 3 minutes left: bound and heartbeat, due 12:00
	if len(q.beats) != 1 || len(q.renews) != 0 {
		t.Fatalf("beats %v renews %v", q.beats, q.renews)
	}
	saved := *w.Keeper
	now = time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)
	tick(t, d) // 12:00, 2 minutes left: renewed to 12:05
	if len(q.renews) != 1 || !q.obs.Attempts[0].LeaseExpires.Equal(now.Add(5*time.Minute)) {
		t.Fatalf("renews %v lease %s", q.renews, q.obs.Attempts[0].LeaseExpires)
	}
	// The ledger save of 12:00 is lost: the keeper restarts from 11:59's record.
	*w.Keeper = saved
	now = time.Date(2026, 10, 17, 12, 4, 30, 0, time.UTC)
	writes := len(q.ids)
	tick(t, d)
	if len(q.ids) != writes+2 || q.ids[writes] != q.ids[writes-1] {
		t.Fatalf("writes after restart %v", q.ids[writes:])
	}
	lease := q.obs.Attempts[0].LeaseExpires
	if len(q.renews) != 2 || !lease.Equal(now.Add(5*time.Minute)) {
		t.Fatalf("renews %v lease %s", q.renews, lease)
	}
	if k := w.Keeper; !k.Due.Equal(now.Add(time.Minute)) || k.LastAction != keeperRenewed || k.Failing {
		t.Fatalf("keeper %+v", k)
	}
	for end := now.Add(10 * time.Minute); now.Before(end); now = now.Add(30 * time.Second) {
		tick(t, d)
		if a := q.obs.Attempts[0]; !a.LeaseExpires.After(now) || w.Keeper.State != keeperKeeping {
			t.Fatalf("at %s the lease %s lapsed, keeper %+v", now, a.LeaseExpires, w.Keeper)
		}
	}
	if len(q.reaped) != 0 || has(kinds(t, d), "alert") {
		t.Fatalf("reaped %v events %v", q.reaped, kinds(t, d))
	}
}

// TestCALV0311_AbsentOptionChangesNothing is acceptance 6: without
// keepLease no write, keeper record or event appears, and the ledger has
// no keeper member.
func TestCALV0311_AbsentOptionChangesNothing(t *testing.T) {
	now := time.Date(2026, 10, 17, 9, 0, 0, 0, time.UTC)
	q := &keeperQueue{fakeQueue: &fakeQueue{}, now: &now}
	d, w := keeperRig(t, q, q.fakeQueue, &now, now.Add(10*time.Minute), nil)
	for i := 0; i < 5; i++ {
		tick(t, d)
		now = now.Add(time.Minute)
	}
	if len(q.ids) != 0 || w.Keeper != nil || len(keeperActions(t, d)) != 0 {
		t.Fatalf("writes %v keeper %+v", q.ids, w.Keeper)
	}
	raw, err := os.ReadFile(filepath.Join(d.dir, "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), `"keeper"`) {
		t.Fatal("the ledger records a keeper without keepLease")
	}
}

// TestCALV0315_LedgerVersion6RecordsTheKeeper: the keeper record round-trips
// in taskman-dispatch-state/6; a /5 ledger is adopted as recorded with its
// running workers, and a /5 ledger carrying a keeper is refused, as is an
// invalid record.
func TestCALV0315_LedgerVersion6RecordsTheKeeper(t *testing.T) {
	dir := t.TempDir()
	write := func(profile, worker string) (*Ledger, error) {
		l := `{"profile":"` + profile + `","program":"prog","launchSeq":1,"eventSeq":0,"workers":[{"id":"w1","role":"dev","key":"ticket:a:q:t1","ticket":"t1","pid":4242,"leaderIdentity":"x","started":"2026-10-17T00:00:00Z"` + worker + `}],"backoff":{}}`
		if err := os.WriteFile(filepath.Join(dir, "state.json"), []byte(l), 0o600); err != nil {
			t.Fatal(err)
		}
		return LoadLedger(dir, "prog")
	}
	if l, err := write(drainedState5Profile, ``); err != nil || l.Profile != StateProfile || len(l.Workers) != 1 || l.Workers[0].Keeper != nil {
		t.Fatalf("/5 with a running worker = %+v %v", l, err)
	}
	keeper := `,"keeper":{"state":"KEEPING","attempt":"a1","generation":"3","due":"2026-10-17T00:01:00Z","lastKept":"2026-10-17T00:00:00Z","lastAction":"RENEWED","renewals":1,"heartbeats":0}`
	if _, err := write(drainedState5Profile, keeper); wire.CodeOf(err) != wire.CodeUnsupportedVersion {
		t.Fatalf("/5 carrying a keeper: %v", err)
	}
	l, err := write(StateProfile, keeper)
	if err != nil || l.Workers[0].Keeper == nil || l.Workers[0].Keeper.Renewals != 1 || l.Workers[0].Keeper.LastAction != keeperRenewed {
		t.Fatalf("/6 keeper = %+v %v", l, err)
	}
	if err := l.save(dir); err != nil {
		t.Fatal(err)
	}
	again, err := LoadLedger(dir, "prog")
	if err != nil || *again.Workers[0].Keeper != *l.Workers[0].Keeper {
		t.Fatalf("round trip = %+v %v", again, err)
	}
	for name, k := range map[string]string{
		"unknown state":       `,"keeper":{"state":"NAPPING","due":"2026-10-17T00:00:00Z","lastKept":"0001-01-01T00:00:00Z","renewals":0,"heartbeats":0}`,
		"keeping unbound":     `,"keeper":{"state":"KEEPING","due":"2026-10-17T00:00:00Z","lastKept":"0001-01-01T00:00:00Z","renewals":0,"heartbeats":0}`,
		"stopped no reason":   `,"keeper":{"state":"STOPPED","attempt":"a1","generation":"3","due":"2026-10-17T00:00:00Z","lastKept":"0001-01-01T00:00:00Z","renewals":0,"heartbeats":0}`,
		"waiting bound":       `,"keeper":{"state":"WAITING","attempt":"a1","due":"0001-01-01T00:00:00Z","lastKept":"0001-01-01T00:00:00Z","renewals":0,"heartbeats":0}`,
		"negative count":      `,"keeper":{"state":"KEEPING","attempt":"a1","generation":"3","due":"2026-10-17T00:00:00Z","lastKept":"0001-01-01T00:00:00Z","renewals":-1,"heartbeats":0}`,
		"unknown last action": `,"keeper":{"state":"KEEPING","attempt":"a1","generation":"3","due":"2026-10-17T00:00:00Z","lastKept":"0001-01-01T00:00:00Z","lastAction":"PINGED","renewals":0,"heartbeats":0}`,
		"unknown member":      `,"keeper":{"state":"WAITING","due":"0001-01-01T00:00:00Z","lastKept":"0001-01-01T00:00:00Z","renewals":0,"heartbeats":0,"extra":1}`,
	} {
		if _, err := write(StateProfile, k); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}
