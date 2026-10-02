//go:build darwin || linux

package gitrun

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/groupreap"
)

// lifecycle is a stable budget under a fake clock with recorded events.
type lifecycle struct {
	mu      sync.Mutex
	clock   time.Time
	events  []Event
	hooks   map[string]func()
	expired int
	budget  *Budget
}

func newLifecycle(ops int, primitives func(int, bool) groupreap.Primitives) *lifecycle {
	l := &lifecycle{clock: time.Unix(1_800_000_000, 0), hooks: map[string]func(){}}
	deadline := l.clock.Add(30 * time.Minute)
	l.budget = NewStableBudget(ops, deadline, func() { l.mu.Lock(); l.expired++; l.mu.Unlock() }, Seam{
		Now: func() time.Time { l.mu.Lock(); defer l.mu.Unlock(); return l.clock },
		Event: func(event Event) {
			l.mu.Lock()
			l.events = append(l.events, event)
			hook := l.hooks[event.Name]
			delete(l.hooks, event.Name)
			l.mu.Unlock()
			if hook != nil {
				hook()
			}
		},
		Primitives: primitives,
	})
	return l
}

func (l *lifecycle) advance(d time.Duration) { l.mu.Lock(); l.clock = l.clock.Add(d); l.mu.Unlock() }

func (l *lifecycle) names() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	names := make([]string, len(l.events))
	for i, event := range l.events {
		names[i] = event.Name
	}
	return names
}

func (l *lifecycle) count(name string) int {
	n := 0
	for _, have := range l.names() {
		if have == name {
			n++
		}
	}
	return n
}

func lifecycleShell() Options {
	return Options{Binary: "/bin/sh", StdoutLimit: 1 << 16, Env: []string{"PATH=/usr/bin:/bin"}}
}

func requireCode(t *testing.T, err error, code string) {
	t.Helper()
	if cemcode.CodeOf(err) != code {
		t.Fatalf("error = %v (code %q), want code %q", err, cemcode.CodeOf(err), code)
	}
}

const hang = "exec /bin/sleep 600"

func TestStableSuccessReleasesOwnedGroup(t *testing.T) {
	l := newLifecycle(4, nil)
	out, err := Run(context.Background(), l.budget, lifecycleShell(), "-c", "echo ok")
	if err != nil || string(out) != "ok\n" {
		t.Fatalf("out=%q err=%v", out, err)
	}
	want := []string{"reserved", "op-start", "released"}
	if got := l.names(); len(got) != len(want) || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
		t.Fatalf("events = %v, want %v", got, want)
	}
	if l.budget.Held() {
		t.Fatal("a released owner must not hold the budget")
	}
}

// The per-operation deadline is read from the budget's one clock: no real
// time passes, and the operation ends as soon as the clock reaches it.
func TestStablePerOperationDeadlineUsesOneClock(t *testing.T) {
	l := newLifecycle(4, nil)
	l.hooks["op-start"] = func() { l.advance(DefaultPerOpTimeout) }
	started := time.Now()
	_, err := Run(context.Background(), l.budget, lifecycleShell(), "-c", hang)
	requireCode(t, err, cemcode.GitTimeout)
	if elapsed := time.Since(started); elapsed > 5*time.Second {
		t.Fatalf("fake-clock deadline took %v of real time", elapsed)
	}
	if l.count("cause-committed") != 1 || l.count("released") != 1 || l.expired != 0 {
		t.Fatalf("events = %v expired=%d", l.names(), l.expired)
	}
}

// One instant short of the deadline is not a timeout: the boundary is
// "now is not before the deadline".
func TestStablePerOperationDeadlineBoundary(t *testing.T) {
	l := newLifecycle(4, nil)
	l.hooks["op-start"] = func() { l.advance(DefaultPerOpTimeout - time.Nanosecond) }
	out, err := Run(context.Background(), l.budget, lifecycleShell(), "-c", "sleep 0.2; echo late")
	if err != nil || string(out) != "late\n" {
		t.Fatalf("one nanosecond before the deadline: out=%q err=%v", out, err)
	}
}

func TestStableCausePrecedence(t *testing.T) {
	overflow := "/bin/dd if=/dev/zero bs=65536 count=4 2>/dev/null; " + hang
	failingProbe := func(int, bool) groupreap.Primitives {
		return groupreap.Primitives{ProbeGroup: func(int) (groupreap.Probe, error) {
			return groupreap.ProbeLive, errors.New("injected probe failure")
		}}
	}
	cases := []struct {
		name       string
		script     string
		primitives func(int, bool) groupreap.Primitives
		at         string
		cancel     bool
		advance    time.Duration
		want       string
	}{
		{"cancel", hang, nil, "op-start", true, 0, cemcode.GitCancelled},
		{"outer-expiry", hang, nil, "op-start", false, 30 * time.Minute, cemcode.GitCancelled},
		{"overflow", overflow, nil, "", false, 0, cemcode.GitOutputExceeded},
		{"overflow-and-nonzero", "/bin/dd if=/dev/zero bs=65536 count=4 2>/dev/null; exit 3", nil, "", false, 0, cemcode.GitOutputExceeded},
		{"nonzero", "exit 3", nil, "", false, 0, cemcode.GitExitFailure},
		{"overflow-then-cancel", overflow, nil, "retire-started", true, 0, cemcode.GitCancelled},
		{"overflow-then-per-op-deadline", overflow, nil, "retire-started", false, DefaultPerOpTimeout, cemcode.GitTimeout},
		{"timeout-then-outer-before-commit", hang, nil, "retire-started", false, 30*time.Minute - DefaultPerOpTimeout, cemcode.GitCancelled},
		{"cancel-and-unobserved-cleanup", hang, failingProbe, "op-start", true, 0, ProcessContainment},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			l := newLifecycle(4, c.primitives)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if c.name == "timeout-then-outer-before-commit" {
				l.hooks["op-start"] = func() { l.advance(DefaultPerOpTimeout) }
			}
			if c.at != "" {
				l.hooks[c.at] = func() {
					if c.cancel {
						cancel()
					}
					l.advance(c.advance)
				}
			}
			_, err := Run(ctx, l.budget, lifecycleShell(), "-c", c.script)
			requireCode(t, err, c.want)
			if held := l.budget.Held(); held != (c.want == ProcessContainment) {
				t.Fatalf("held = %v", held)
			}
		})
	}
}

// cause-committed is the commitment point: an outer expiry first observed
// after it does not replace the committed per-operation timeout.
func TestStableCommittedCauseSurvivesLaterOuterExpiry(t *testing.T) {
	l := newLifecycle(4, nil)
	l.hooks["op-start"] = func() { l.advance(DefaultPerOpTimeout) }
	l.hooks["cause-committed"] = func() { l.advance(30*time.Minute - DefaultPerOpTimeout) }
	_, err := Run(context.Background(), l.budget, lifecycleShell(), "-c", hang)
	requireCode(t, err, cemcode.GitTimeout)
	if !l.budget.OuterExpired() || l.expired != 1 {
		t.Fatalf("the outer expiry must still be observable by the caller exactly once; expired=%d", l.expired)
	}
}

// HOLD is sticky: no later operation spawns, is retried or is downgraded.
func TestStableHoldIsSticky(t *testing.T) {
	l := newLifecycle(8, func(int, bool) groupreap.Primitives {
		return groupreap.Primitives{ProbeGroup: func(int) (groupreap.Probe, error) {
			return groupreap.ProbeLive, errors.New("injected probe failure")
		}}
	})
	_, err := Run(context.Background(), l.budget, lifecycleShell(), "-c", "exit 0")
	requireCode(t, err, ProcessContainment)
	if l.count("hold") != 1 {
		t.Fatalf("events = %v", l.names())
	}
	before := l.count("op-start")
	_, err = Run(context.Background(), l.budget, lifecycleShell(), "-c", "exit 0")
	requireCode(t, err, ProcessContainment)
	if _, err := l.budget.Reserve(); cemcode.CodeOf(err) != ProcessContainment {
		t.Fatalf("Reserve on a held budget = %v", err)
	}
	if l.count("op-start") != before || l.count("reserved") != 1 {
		t.Fatalf("a held budget spawned or reserved again: %v", l.names())
	}
}

// The count bound is exact, precedes the outer deadline, and a replay reuses
// its reservation instead of charging the logical operation twice.
func TestStableOperationLedger(t *testing.T) {
	l := newLifecycle(1024, nil)
	var last Reservation
	for i := 1; i <= 1024; i++ {
		reservation, err := l.budget.Reserve()
		if err != nil || reservation.Ordinal != i {
			t.Fatalf("reservation %d = %+v, %v", i, reservation, err)
		}
		last = reservation
	}
	out, err := RunReservation(context.Background(), l.budget, last, lifecycleShell(), "-c", "echo replay")
	if err != nil || string(out) != "replay\n" {
		t.Fatalf("replay under reservation 1024: out=%q err=%v", out, err)
	}
	if l.count("reserved") != 1024 {
		t.Fatalf("reserved events = %d, want 1024", l.count("reserved"))
	}
	l.advance(31 * time.Minute)
	starts := l.count("op-start")
	_, err = Run(context.Background(), l.budget, lifecycleShell(), "-c", "echo never")
	requireCode(t, err, cemcode.GitBudgetExceeded)
	if l.count("op-start") != starts || l.count("reserved") != 1024 || l.count("refused-before-spawn") != 1 {
		t.Fatal("operation 1025 was not refused before spawn")
	}
}

func TestStableReservationDeadlineIsCappedAtOuter(t *testing.T) {
	l := newLifecycle(4, nil)
	l.advance(30*time.Minute - 3*time.Second)
	reservation, err := l.budget.Reserve()
	if err != nil {
		t.Fatal(err)
	}
	if want := l.budget.Now().Add(3 * time.Second); !reservation.Deadline.Equal(want) {
		t.Fatalf("deadline = %v, want the outer deadline %v", reservation.Deadline, want)
	}
	l.advance(3 * time.Second)
	_, err = l.budget.Reserve()
	requireCode(t, err, cemcode.GitCancelled)
	_, err = Run(context.Background(), l.budget, lifecycleShell(), "-c", "echo never")
	requireCode(t, err, cemcode.GitCancelled)
	if l.count("op-start") != 0 || l.expired != 1 {
		t.Fatalf("spawned after outer expiry or expire called %d times: %v", l.expired, l.names())
	}
}

// The emergency allowance is one non-renewable window anchored at the first
// terminal event: cleanup that is still unproved when it ends is HOLD.
func TestStableEmergencyAllowanceIsSingle(t *testing.T) {
	var l *lifecycle
	var once sync.Once
	l = newLifecycle(4, func(int, bool) groupreap.Primitives {
		return groupreap.Primitives{ProbeGroup: func(int) (groupreap.Probe, error) {
			// The group never becomes quiet; the fake clock then passes the
			// whole allowance.
			once.Do(func() { l.advance(EmergencyAllowance) })
			return groupreap.ProbeLive, nil
		}}
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	l.hooks["op-start"] = cancel
	started := time.Now()
	_, err := Run(ctx, l.budget, lifecycleShell(), "-c", hang)
	requireCode(t, err, ProcessContainment)
	if elapsed := time.Since(started); elapsed > 5*time.Second {
		t.Fatalf("emergency allowance took %v of real time under the fake clock", elapsed)
	}
	if !l.budget.Held() {
		t.Fatal("unproved cleanup at the end of the allowance must hold")
	}
}

func TestStableCancelledBeforeSpawn(t *testing.T) {
	l := newLifecycle(4, nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := Run(ctx, l.budget, lifecycleShell(), "-c", "echo never")
	requireCode(t, err, cemcode.GitCancelled)
	if l.count("op-start") != 0 {
		t.Fatalf("spawned after cancellation: %v", l.names())
	}
}
