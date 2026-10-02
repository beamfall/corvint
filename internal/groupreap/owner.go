package groupreap

import (
	"errors"
	"os/exec"
	"sync"
	"time"
)

// State names one step of an owned process group's lifecycle.
type State string

// Owner states. HOLD is terminal and retains the handle: cleanup was not
// observed, and nothing further is signalled or reaped through the owner.
const (
	NotStarted        State = "NOT_STARTED"
	OwnedRunning      State = "OWNED_RUNNING"
	OwnedExitObserved State = "OWNED_EXIT_OBSERVED"
	Retiring          State = "RETIRING"
	Reaping           State = "REAPING"
	Released          State = "RELEASED"
	Hold              State = "HOLD"
)

// Probe is one signal-0 observation of an owned group.
type Probe int

const (
	// ProbeLive: a signalable member remains.
	ProbeLive Probe = iota
	// ProbeQuiet: the group ID still exists but holds no signalable member
	// (EPERM while only the unreaped leader remains).
	ProbeQuiet
	// ProbeAbsent: the group ID names nothing (ESRCH).
	ProbeAbsent
)

// ErrOwnerUnavailable reports a platform without the unreaped-leader owner.
var ErrOwnerUnavailable = errors.New("groupreap: owned process groups are unavailable on this platform")

// Primitives are the host operations an Owner uses. A nil field means the
// platform default; tests replace fields to inject faults.
type Primitives struct {
	// WaitExit blocks until the leader has exited and leaves it unreaped.
	WaitExit func(leader int) error
	// KillGroup sends SIGKILL to the leader's group. A group with no
	// signalable member is not an error.
	KillGroup func(leader int) error
	// ProbeGroup is one signal-0 observation. It never creates kill authority.
	ProbeGroup func(leader int) (Probe, error)
	// Reap collects the leader. A non-nil *exec.ExitError is an ordinary status.
	Reap func(command *exec.Cmd) error
	// NewTimer starts a timer. Tests replace it to drive fixed deadlines without
	// waiting on wall time.
	NewTimer func(time.Duration) Timer
}

// Timer is the timer subset Owner needs.
type Timer interface {
	C() <-chan time.Time
	Stop() bool
}

type realTimer struct{ timer *time.Timer }

func (t realTimer) C() <-chan time.Time { return t.timer.C }
func (t realTimer) Stop() bool          { return t.timer.Stop() }

func newRealTimer(d time.Duration) Timer {
	return realTimer{timer: time.NewTimer(d)}
}

// Result is the outcome of Finish. Only State == Released proves cleanup.
type Result struct {
	State   State
	WaitErr error // the leader's exit status as reported by the reap
	Err     error // why the owner is in HOLD
	// PostReap is the last observation made after the reap; it is
	// meaningful only when PostReapObserved is set.
	PostReap         Probe
	PostReapObserved bool
}

// Owner owns one process group through an unreaped leader. The group is
// signalled only while the leader is unreaped, at most once, and never after
// the reap. Absence is observed in two steps: signal 0 is polled with the
// leader unreaped until no signalable member remains, the leader is reaped,
// and signal 0 is polled for a bounded interval until the group is absent.
type Owner struct {
	mu         sync.Mutex
	command    *exec.Cmd
	leader     int
	p          Primitives
	state      State
	signalled  bool
	signalErr  error
	observeErr error
	exited     chan struct{}
	events     []string
	result     Result
}

const (
	// probeInterval separates signal-0 observations.
	probeInterval = time.Millisecond
	// postReapProbeDeadline is the fixed OQ-12b bound for signal-0 polling
	// after the leader has been reaped. No real signal is sent in this window.
	postReapProbeDeadline = 2 * time.Second
)

// Start starts command as the leader of a new process group and owns it. On
// a platform without the owner it refuses before any process is started.
func Start(command *exec.Cmd) (*Owner, error) { return StartWith(command, Primitives{}) }

// StartWith is Start with fault-injection primitives.
func StartWith(command *exec.Cmd, p Primitives) (*Owner, error) {
	if !OwnerAvailable() {
		return nil, ErrOwnerUnavailable
	}
	containLeader(command)
	if err := command.Start(); err != nil {
		return nil, err
	}
	return adopt(command, command.Process.Pid, p), nil
}

func adopt(command *exec.Cmd, leader int, p Primitives) *Owner {
	defaults := defaultPrimitives()
	if p.WaitExit == nil {
		p.WaitExit = defaults.WaitExit
	}
	if p.KillGroup == nil {
		p.KillGroup = defaults.KillGroup
	}
	if p.ProbeGroup == nil {
		p.ProbeGroup = defaults.ProbeGroup
	}
	if p.Reap == nil {
		p.Reap = defaults.Reap
	}
	if p.NewTimer == nil {
		p.NewTimer = newRealTimer
	}
	o := &Owner{command: command, leader: leader, p: p, state: OwnedRunning, exited: make(chan struct{})}
	go o.observe()
	return o
}

func (o *Owner) observe() {
	err := o.p.WaitExit(o.leader)
	o.mu.Lock()
	if err != nil {
		o.observeErr = err
		o.recordEvent("exit-unobserved")
	} else if o.state == OwnedRunning {
		o.state = OwnedExitObserved
		o.recordEvent("exit-observed")
	}
	o.mu.Unlock()
	close(o.exited)
}

func (o *Owner) recordEvent(event string) { o.events = append(o.events, event) }

// Exited is closed once the leader's exit was observed (it stays unreaped) or
// the observation failed; Finish distinguishes the two.
func (o *Owner) Exited() <-chan struct{} { return o.exited }

// State returns the current state.
func (o *Owner) State() State {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.state
}

// Events returns the ordered lifecycle events, for tests and timing ledgers.
func (o *Owner) Events() []string {
	o.mu.Lock()
	defer o.mu.Unlock()
	return append([]string(nil), o.events...)
}

// Stop retires the group immediately: one SIGKILL to the group while the
// leader is unreaped. It never signals twice, never after the reap has begun,
// and never when the exit observation failed.
func (o *Owner) Stop() {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.state == OwnedRunning || o.state == OwnedExitObserved {
		o.kill()
	}
}

// kill sends the single group signal. The caller holds o.mu.
func (o *Owner) kill() error {
	if o.signalled || o.observeErr != nil {
		return nil
	}
	o.signalled = true
	o.recordEvent("kill-group")
	err := o.p.KillGroup(o.leader)
	if err != nil {
		o.signalErr = err
	}
	return err
}

func (o *Owner) hold(err error) Result {
	o.state = Hold
	o.recordEvent("hold")
	o.result = Result{State: Hold, Err: err}
	return o.result
}

// Finish completes the lifecycle. It waits for the observed exit, retires the
// group, proves absence and reaps. limit bounds everything before the reap;
// after the reap a fixed signal-0-only deadline proves absence or HOLDs. A
// second call returns the first result and performs no action.
func (o *Owner) Finish(limit <-chan struct{}) Result {
	o.mu.Lock()
	if o.state == Released || o.state == Hold {
		defer o.mu.Unlock()
		return o.result
	}
	o.mu.Unlock()
	select {
	case <-o.exited:
	case <-limit:
		// Prefer an exit that was already observed.
		select {
		case <-o.exited:
		default:
			o.mu.Lock()
			defer o.mu.Unlock()
			return o.hold(errors.New("groupreap: leader exit was not observed within the retirement bound"))
		}
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.observeErr != nil {
		// Without the unreaped-leader observation the group ID is not
		// provably ours: no numeric signal is sent.
		return o.hold(errors.Join(errors.New("groupreap: leader exit observation failed"), o.observeErr))
	}
	o.state = Retiring
	if !o.signalled {
		if err := o.kill(); err != nil {
			return o.hold(errors.Join(errors.New("groupreap: group signal failed"), err))
		}
	}
	if o.signalErr != nil {
		return o.hold(errors.Join(errors.New("groupreap: group signal failed"), o.signalErr))
	}
	// Step one: leader unreaped, poll signal 0 until no signalable member.
	for {
		probe, err := o.p.ProbeGroup(o.leader)
		if err != nil {
			return o.hold(errors.Join(errors.New("groupreap: group probe failed"), err))
		}
		if probe != ProbeLive {
			o.recordEvent("probe-quiet")
			break
		}
		o.recordEvent("probe-live")
		o.mu.Unlock()
		expired := false
		timer := o.p.NewTimer(probeInterval)
		select {
		case <-limit:
			expired = true
		case <-timer.C():
		}
		timer.Stop()
		o.mu.Lock()
		if expired {
			return o.hold(errors.New("groupreap: group members remained signalable at the retirement bound"))
		}
	}
	o.state = Reaping
	o.recordEvent("reap")
	waitErr := o.p.Reap(o.command)
	var exit *exec.ExitError
	if waitErr != nil && !errors.As(waitErr, &exit) {
		return o.hold(errors.Join(errors.New("groupreap: leader reap failed"), waitErr))
	}
	// Step two: poll signal 0 after the reap until absence or the fixed
	// OQ-12b deadline. No real signal follows the reap.
	deadline := o.p.NewTimer(postReapProbeDeadline)
	defer deadline.Stop()
	finalNoted := false
	for {
		probe, err := o.p.ProbeGroup(o.leader)
		if !finalNoted {
			o.recordEvent("probe-final")
			finalNoted = true
		}
		if err != nil {
			o.hold(errors.Join(errors.New("groupreap: group absence probe failed after the reap"), err))
			o.result.WaitErr, o.result.PostReap, o.result.PostReapObserved = waitErr, probe, false
			return o.result
		}
		if probe == ProbeAbsent {
			o.state = Released
			o.recordEvent("released")
			o.result = Result{State: Released, WaitErr: waitErr, PostReap: ProbeAbsent, PostReapObserved: true}
			return o.result
		}
		o.mu.Unlock()
		expired := false
		timer := o.p.NewTimer(probeInterval)
		select {
		case <-deadline.C():
			expired = true
		case <-timer.C():
		}
		timer.Stop()
		o.mu.Lock()
		if expired {
			o.hold(errors.New("groupreap: group absence was not observed after the reap before the post-reap deadline"))
			o.result.WaitErr, o.result.PostReap, o.result.PostReapObserved = waitErr, probe, true
			return o.result
		}
	}
}
