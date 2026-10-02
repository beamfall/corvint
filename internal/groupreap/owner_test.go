package groupreap

import (
	"errors"
	"os/exec"
	"slices"
	"strings"
	"sync"
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
			f.mu.Lock()
			f.reaped = true
			f.mu.Unlock()
			return f.reapErr
		},
		NewTimer: f.newTimer,
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
		}, closed(), "kill,probe"},
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
			} else {
				requireCalls(t, f, c.calls)
			}
		})
	}
}
