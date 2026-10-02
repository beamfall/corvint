package groupreap

import (
	"errors"
	"fmt"
	"os/exec"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// fakeHost records every primitive call so a test can prove order and count.
type fakeHost struct {
	mu                        sync.Mutex
	calls                     []string
	exit                      chan error
	killErr                   error
	probes                    []Probe // consumed in order; the last one repeats
	probeErr                  error
	reapErr                   error
	reaped                    bool
	reapStarted               chan struct{}
	reapRelease               chan struct{}
	postReapDeadlineAfter     int
	postReapIntervalTimers    int
	postReapDeadlineTimer     *fakeTimer
	postReapDeadlineRequested bool
	blockPreReapIntervals     bool
}

func newFakeHost(probes ...Probe) *fakeHost {
	return &fakeHost{exit: make(chan error, 1), probes: probes}
}

func (f *fakeHost) record(call string) {
	f.mu.Lock()
	f.calls = append(f.calls, call)
	f.mu.Unlock()
}

func (f *fakeHost) seen() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.calls)
}

func (f *fakeHost) primitives() Primitives {
	return Primitives{
		WaitExit:  func(int) error { return <-f.exit },
		KillGroup: func(int) error { f.record("kill"); return f.killErr },
		ProbeGroup: func(int) (Probe, error) {
			f.mu.Lock()
			defer f.mu.Unlock()
			f.calls = append(f.calls, "probe")
			probe := f.probes[0]
			if len(f.probes) > 1 {
				f.probes = f.probes[1:]
			}
			return probe, f.probeErr
		},
		Reap: func(*exec.Cmd) error {
			f.record("reap")
			if f.reapStarted != nil {
				close(f.reapStarted)
			}
			if f.reapRelease != nil {
				<-f.reapRelease
			}
			f.mu.Lock()
			f.reaped = true
			f.mu.Unlock()
			return f.reapErr
		},
		RetirementMode: RequirePreReapQuiet,
		NewTimer:       f.newTimer,
	}
}

type fakeTimer struct {
	ch chan time.Time
}

func readyFakeTimer() *fakeTimer {
	t := &fakeTimer{ch: make(chan time.Time, 1)}
	t.ch <- time.Time{}
	return t
}

func blockedFakeTimer() *fakeTimer {
	return &fakeTimer{ch: make(chan time.Time, 1)}
}

func (t *fakeTimer) C() <-chan time.Time { return t.ch }
func (t *fakeTimer) Stop() bool          { return true }

func (t *fakeTimer) fire() {
	select {
	case t.ch <- time.Time{}:
	default:
	}
}

func (f *fakeHost) newTimer(d time.Duration) Timer {
	f.mu.Lock()
	defer f.mu.Unlock()
	if d == postReapProbeDeadline {
		f.postReapDeadlineRequested = true
		f.postReapDeadlineTimer = blockedFakeTimer()
		return f.postReapDeadlineTimer
	}
	if d == probeInterval && !f.reaped && f.blockPreReapIntervals {
		return blockedFakeTimer()
	}
	if d == probeInterval && f.reaped && f.postReapDeadlineAfter > 0 {
		f.postReapIntervalTimers++
		if f.postReapIntervalTimers >= f.postReapDeadlineAfter {
			if f.postReapDeadlineTimer != nil {
				f.postReapDeadlineTimer.fire()
			}
			return blockedFakeTimer()
		}
	}
	return readyFakeTimer()
}

func open() chan struct{} { return make(chan struct{}) }

func closed() chan struct{} {
	c := make(chan struct{})
	close(c)
	return c
}

func requireCalls(t *testing.T, f *fakeHost, want string) {
	t.Helper()
	if got := strings.Join(f.seen(), ","); got != want {
		t.Fatalf("primitive calls = %s, want %s", got, want)
	}
}

func requirePostReapOnlyProbes(t *testing.T, f *fakeHost, wantAtLeast int) {
	t.Helper()
	calls := f.seen()
	reapIndex := slices.Index(calls, "reap")
	if reapIndex < 0 {
		t.Fatalf("primitive calls have no reap: %s", strings.Join(calls, ","))
	}
	post := calls[reapIndex+1:]
	if len(post) < wantAtLeast {
		t.Fatalf("post-reap primitive calls = %s, want at least %d probes", strings.Join(post, ","), wantAtLeast)
	}
	for _, call := range post {
		if call != "probe" {
			t.Fatalf("post-reap primitive call = %q in %s, want only probes", call, strings.Join(post, ","))
		}
	}
}

func TestOwnerNormalExitRetiresProbesReapsAndObservesAbsence(t *testing.T) {
	f := newFakeHost(ProbeLive, ProbeLive, ProbeQuiet, ProbeAbsent)
	o := adopt(nil, 4242, f.primitives())
	if o.State() != OwnedRunning {
		t.Fatalf("state = %s", o.State())
	}
	f.exit <- nil
	<-o.Exited()
	if o.State() != OwnedExitObserved {
		t.Fatalf("state = %s", o.State())
	}
	result := o.Finish(open())
	if result.State != Released || result.Err != nil || result.WaitErr != nil {
		t.Fatalf("result = %+v", result)
	}
	// One group signal, before the reap; exactly one probe after it; nothing later.
	requireCalls(t, f, "kill,probe,probe,probe,reap,probe")
	want := "exit-observed,kill-group,probe-live,probe-live,probe-quiet,reap,probe-final,released"
	if got := strings.Join(o.Events(), ","); got != want {
		t.Fatalf("events = %s", got)
	}
	o.Stop()
	if again := o.Finish(open()); again.State != Released {
		t.Fatalf("second finish = %+v", again)
	}
	requireCalls(t, f, "kill,probe,probe,probe,reap,probe")
}

func TestOwnerStopSignalsOnceAndFinishDoesNotSignalAgain(t *testing.T) {
	f := newFakeHost(ProbeAbsent)
	o := adopt(nil, 4242, f.primitives())
	o.Stop()
	o.Stop()
	f.exit <- nil
	status := &exec.ExitError{}
	f.reapErr = status
	result := o.Finish(open())
	if result.State != Released || result.WaitErr != error(status) {
		t.Fatalf("result = %+v", result)
	}
	requireCalls(t, f, "kill,probe,reap,probe")
}

func TestOwnerStopKillErrorThenFinishHolds(t *testing.T) {
	f := newFakeHost(ProbeAbsent)
	f.killErr = errors.New("EINVAL")
	o := adopt(nil, 4242, f.primitives())
	o.Stop()
	f.exit <- nil
	<-o.Exited()
	result := o.Finish(open())
	if result.State != Hold || result.Err == nil {
		t.Fatalf("result = %+v", result)
	}
	if !strings.Contains(result.Err.Error(), "group signal failed") {
		t.Fatalf("result err = %v, want group signal failed", result.Err)
	}
	requireCalls(t, f, "kill")
	if again := o.Finish(open()); again.State != Hold {
		t.Fatalf("second finish = %+v", again)
	}
	requireCalls(t, f, "kill")
}

func TestOwnerHoldsWithoutSignallingWhenExitObservationFails(t *testing.T) {
	f := newFakeHost(ProbeAbsent)
	o := adopt(nil, 4242, f.primitives())
	f.exit <- errors.New("ECHILD")
	<-o.Exited()
	o.Stop()
	result := o.Finish(open())
	if result.State != Hold || result.Err == nil {
		t.Fatalf("result = %+v", result)
	}
	requireCalls(t, f, "")
}

func TestOwnerHoldsWhenExitIsNotObservedWithinTheBound(t *testing.T) {
	f := newFakeHost(ProbeAbsent)
	o := adopt(nil, 4242, f.primitives())
	o.Stop()
	result := o.Finish(closed())
	if result.State != Hold {
		t.Fatalf("result = %+v", result)
	}
	requireCalls(t, f, "kill")
	// HOLD is sticky: a late exit changes nothing and nothing more is called.
	f.exit <- nil
	<-o.Exited()
	o.Stop()
	if again := o.Finish(open()); again.State != Hold || o.State() != Hold {
		t.Fatalf("second finish = %+v", again)
	}
	requireCalls(t, f, "kill")
}

func TestOwnerHoldPaths(t *testing.T) {
	cases := []struct {
		name  string
		set   func(*fakeHost)
		limit chan struct{}
		calls string
	}{
		{"signal error", func(f *fakeHost) { f.killErr = errors.New("EINVAL") }, open(), "kill"},
		{"probe error", func(f *fakeHost) { f.probeErr = errors.New("EINVAL") }, open(), "kill,probe"},
		{"members stay signalable", func(f *fakeHost) {
			f.probes = []Probe{ProbeLive}
			f.blockPreReapIntervals = true
		}, closed(), "none"},
		{"reap failure", func(f *fakeHost) { f.reapErr = exec.ErrWaitDelay }, open(), "kill,probe,reap"},
		{"quiet after reap", func(f *fakeHost) {
			f.probes = []Probe{ProbeQuiet}
			f.postReapDeadlineAfter = 3
		}, open(), ""},
		{"live after reap", func(f *fakeHost) {
			f.probes = []Probe{ProbeQuiet, ProbeLive}
			f.postReapDeadlineAfter = 3
		}, open(), ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newFakeHost(ProbeAbsent)
			c.set(f)
			o := adopt(nil, 4242, f.primitives())
			f.exit <- nil
			<-o.Exited()
			result := o.Finish(c.limit)
			if result.State != Hold || result.Err == nil {
				t.Fatalf("result = %+v", result)
			}
			if c.calls == "" {
				if !f.postReapDeadlineRequested {
					t.Fatal("post-reap deadline timer was not requested")
				}
				requirePostReapOnlyProbes(t, f, 3)
			} else if c.calls == "none" {
				requireCalls(t, f, "")
			} else {
				requireCalls(t, f, c.calls)
			}
			o.Stop()
			if again := o.Finish(open()); again.State != Hold {
				t.Fatalf("second finish = %+v", again)
			}
			// No retry, no further signal and no further probe after HOLD.
			if c.calls == "" {
				requirePostReapOnlyProbes(t, f, 3)
			} else if c.calls == "none" {
				requireCalls(t, f, "")
			} else {
				requireCalls(t, f, c.calls)
			}
		})
	}
}

func TestOwnerRejectsInvalidRetirementMode(t *testing.T) {
	if _, err := resolvePrimitives(Primitives{RetirementMode: RetirementMode(99)}); !errors.Is(err, errInvalidRetirementMode) {
		t.Fatalf("resolve invalid mode err = %v", err)
	}
	command := exec.Command("must-not-spawn-invalid-owner-mode")
	_, err := StartWith(command, Primitives{RetirementMode: RetirementMode(99)})
	if OwnerAvailable() && !errors.Is(err, errInvalidRetirementMode) {
		t.Fatalf("StartWith invalid mode err = %v", err)
	}
	if command.Process != nil {
		t.Fatal("invalid mode spawned a process")
	}
}

func TestOwnerReapAfterSuccessfulSignalSkipsPreReapQuiet(t *testing.T) {
	f := newFakeHost(ProbeAbsent)
	p := f.primitives()
	p.RetirementMode = ReapAfterSuccessfulSignal
	o := adopt(nil, 4242, p)
	f.exit <- nil
	<-o.Exited()
	result := o.Finish(open())
	if result.State != Released || result.PostReap != ProbeAbsent || !result.PostReapObserved {
		t.Fatalf("result = %+v events = %v", result, o.Events())
	}
	requireCalls(t, f, "kill,reap,probe")
	if events := strings.Join(o.Events(), ","); strings.Contains(events, "probe-quiet") || strings.Contains(events, "probe-live") {
		t.Fatalf("pre-reap probe in events = %s", events)
	}
}

func TestOwnerBoundCheckedBeforeAcceptingPostReapAbsence(t *testing.T) {
	f := newFakeHost(ProbeAbsent)
	p := f.primitives()
	p.RetirementMode = ReapAfterSuccessfulSignal
	expired := false
	p.ProbeGroup = func(int) (Probe, error) {
		f.record("probe")
		expired = true
		return ProbeAbsent, nil
	}
	o := adopt(nil, 4242, p)
	f.exit <- nil
	<-o.Exited()
	result := o.FinishBounded(RetirementBound{Done: open(), Expired: func() bool { return expired }})
	if result.State != Hold || result.Err == nil || result.PostReapObserved {
		t.Fatalf("result = %+v events = %v", result, o.Events())
	}
	requireCalls(t, f, "kill,reap,probe")
}

func TestOwnerSlowReapLateCompletionKeepsStickyHold(t *testing.T) {
	f := newFakeHost(ProbeAbsent)
	f.reapStarted = make(chan struct{})
	f.reapRelease = make(chan struct{})
	release := sync.OnceFunc(func() { close(f.reapRelease) })
	t.Cleanup(release)
	p := f.primitives()
	p.RetirementMode = ReapAfterSuccessfulSignal
	o := adopt(nil, 4242, p)
	f.exit <- nil
	<-o.Exited()
	done := make(chan Result, 1)
	limit := make(chan struct{})
	go func() { done <- o.Finish(limit) }()
	awaitOwnerTest(t, f.reapStarted)
	close(limit)
	result := awaitOwnerTest(t, done)
	if result.State != Hold || result.Err == nil {
		t.Fatalf("result = %+v events = %v", result, o.Events())
	}
	release()
	awaitOwnerTest(t, o.reapDone)
	if again := o.Finish(open()); again.State != Hold {
		t.Fatalf("second finish = %+v", again)
	}
	requireCalls(t, f, "kill,reap")
}

func TestOwnerConcurrentFinishAndStopSignalAndReapOnce(t *testing.T) {
	f := newFakeHost(ProbeAbsent)
	f.reapStarted = make(chan struct{})
	f.reapRelease = make(chan struct{})
	p := f.primitives()
	p.RetirementMode = ReapAfterSuccessfulSignal
	o := adopt(nil, 4242, p)
	f.exit <- nil
	<-o.Exited()
	release := sync.OnceFunc(func() { close(f.reapRelease) })
	t.Cleanup(release)
	done := make(chan Result, 2)
	go func() { done <- o.Finish(open()) }()
	awaitOwnerTest(t, f.reapStarted)
	if o.finishMu.TryLock() {
		o.finishMu.Unlock()
		t.Fatal("Finish does not retain serialization during reap")
	}
	secondStarted := make(chan struct{})
	go func() {
		close(secondStarted)
		done <- o.Finish(open())
	}()
	awaitOwnerTest(t, secondStarted)
	o.Stop()
	release()
	result := awaitOwnerTest(t, done)
	if result.State != Released {
		t.Fatalf("result = %+v events = %v", result, o.Events())
	}
	if second := awaitOwnerTest(t, done); second != result {
		t.Fatalf("concurrent result = %+v, first = %+v", second, result)
	}
	requireCalls(t, f, "kill,reap,probe")
}

func awaitOwnerTest[Value any](t *testing.T, channel <-chan Value) Value {
	t.Helper()
	select {
	case value := <-channel:
		return value
	case <-time.After(5 * time.Second):
		t.Fatal("bounded owner test wait expired")
		var zero Value
		return zero
	}
}

func TestOwnerExpiredAllowanceDoesNotStartWork(t *testing.T) {
	for _, observed := range []bool{false, true} {
		t.Run(fmt.Sprint(observed), func(t *testing.T) {
			host := newFakeHost(ProbeAbsent)
			primitives := host.primitives()
			primitives.RetirementMode = ReapAfterSuccessfulSignal
			owner := adopt(nil, 4242, primitives)
			if observed {
				host.exit <- nil
				awaitOwnerTest(t, owner.Exited())
			} else {
				t.Cleanup(func() { host.exit <- nil; awaitOwnerTest(t, owner.Exited()) })
			}
			result := owner.FinishBounded(RetirementBound{Done: open(), Expired: func() bool { return true }})
			if result.State != Hold {
				t.Fatalf("result = %+v", result)
			}
			requireCalls(t, host, "")
		})
	}
}

func TestOwnerSignalConsumesAllowanceBeforeReap(t *testing.T) {
	host := newFakeHost(ProbeAbsent)
	primitives := host.primitives()
	primitives.RetirementMode = ReapAfterSuccessfulSignal
	expired := false
	primitives.KillGroup = func(int) error { host.record("kill"); expired = true; return nil }
	owner := adopt(nil, 4242, primitives)
	host.exit <- nil
	awaitOwnerTest(t, owner.Exited())
	result := owner.FinishBounded(RetirementBound{Done: open(), Expired: func() bool { return expired }})
	if result.State != Hold {
		t.Fatalf("result = %+v", result)
	}
	requireCalls(t, host, "kill")
}

func TestOwnerPostReapCapWins(t *testing.T) {
	for _, insideProbe := range []bool{false, true} {
		t.Run(fmt.Sprint(insideProbe), func(t *testing.T) {
			host := newFakeHost(ProbeAbsent)
			primitives := host.primitives()
			primitives.RetirementMode = ReapAfterSuccessfulSignal
			deadline := blockedFakeTimer()
			primitives.NewTimer = func(duration time.Duration) Timer {
				if duration == postReapProbeDeadline {
					return deadline
				}
				deadline.fire()
				return readyFakeTimer()
			}
			primitives.ProbeGroup = func(int) (Probe, error) {
				host.record("probe")
				if insideProbe {
					deadline.fire()
					return ProbeAbsent, nil
				}
				return ProbeLive, nil
			}
			owner := adopt(nil, 4242, primitives)
			host.exit <- nil
			awaitOwnerTest(t, owner.Exited())
			result := owner.Finish(open())
			if result.State != Hold {
				t.Fatalf("result = %+v", result)
			}
			requireCalls(t, host, "kill,reap,probe")
		})
	}
}

func TestOwnerLinuxModeRetirementBoundary(t *testing.T) {
	for _, moment := range []int64{99, 100, 101} {
		t.Run(fmt.Sprint(moment), func(t *testing.T) {
			host := newFakeHost(ProbeAbsent)
			primitives := host.primitives()
			primitives.RetirementMode = ReapAfterSuccessfulSignal
			var clock atomic.Int64
			primitives.ProbeGroup = func(int) (Probe, error) { host.record("probe"); clock.Store(moment); return ProbeAbsent, nil }
			owner := adopt(nil, 4242, primitives)
			host.exit <- nil
			awaitOwnerTest(t, owner.Exited())
			result := owner.FinishBounded(RetirementBound{Expired: func() bool { return clock.Load() >= 100 }})
			want := Hold
			if moment < 100 {
				want = Released
			}
			if result.State != want {
				t.Fatalf("result = %+v, want %s", result, want)
			}
		})
	}
}

func TestOwnerExpirationAndReapCompletionTogether(t *testing.T) {
	host := newFakeHost(ProbeAbsent)
	primitives := host.primitives()
	primitives.RetirementMode = ReapAfterSuccessfulSignal
	var expired atomic.Bool
	primitives.Reap = func(*exec.Cmd) error { host.record("reap"); expired.Store(true); return nil }
	owner := adopt(nil, 4242, primitives)
	host.exit <- nil
	awaitOwnerTest(t, owner.Exited())
	result := owner.FinishBounded(RetirementBound{Expired: expired.Load})
	if result.State != Hold {
		t.Fatalf("result = %+v", result)
	}
	requireCalls(t, host, "kill,reap")
}

func TestOwnerLinuxModeNonabsence(t *testing.T) {
	for _, probe := range []Probe{ProbeLive, ProbeQuiet} {
		for _, persistent := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d/%t", probe, persistent), func(t *testing.T) {
				host := newFakeHost(probe, ProbeAbsent)
				if persistent {
					host.probes = []Probe{probe}
					host.postReapDeadlineAfter = 3
				}
				primitives := host.primitives()
				primitives.RetirementMode = ReapAfterSuccessfulSignal
				owner := adopt(nil, 4242, primitives)
				host.exit <- nil
				awaitOwnerTest(t, owner.Exited())
				result := owner.Finish(open())
				if persistent {
					if result.State != Hold || result.PostReap != probe {
						t.Fatalf("result = %+v", result)
					}
					requirePostReapOnlyProbes(t, host, 3)
				} else {
					if result.State != Released || result.PostReap != ProbeAbsent {
						t.Fatalf("result = %+v", result)
					}
					requireCalls(t, host, "kill,reap,probe,probe")
				}
			})
		}
	}
}
