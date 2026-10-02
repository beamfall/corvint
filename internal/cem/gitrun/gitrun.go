// Package gitrun executes bounded, contained Git child processes for the CEM
// seams. Every failure is typed: cancellation, per-operation timeout, budget
// exhaustion, output-bound excess, start failure, and non-zero exit each carry
// a distinct registered code and are never collapsed into one another.
//
// Containment: on Unix the child runs in its own process group. On every
// abnormal path (cancel, timeout, output excess) the whole group is killed
// BEFORE the child is reaped, so the group ID cannot be recycled while
// descendants are signalled. On the normal-exit path the group is swept
// immediately after the reap; the theoretical group-ID-reuse window there is
// accepted because the sweep is immediate and hostile same-UID interference is
// outside the approved trust boundary. Windows containment is deliberately not
// claimed here: Windows native execution is a separate unpromoted lane, and the
// stub kills only the direct child.
package gitrun

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/groupreap"
)

// Frozen operational bounds.
const (
	DefaultOperations   = 1024
	DefaultTotalBudget  = 30 * time.Minute
	DefaultPerOpTimeout = 10 * time.Second
	DefaultStderrLimit  = 64 << 10
)

// Budget bounds one verification's Git usage: an operation count and a total
// wall deadline shared across operations.
type Budget struct {
	remaining int
	deadline  time.Time
	stable    *stableState // nil for every legacy caller
}

// NewBudget returns a budget of ops operations within total wall time.
func NewBudget(ops int, total time.Duration) *Budget {
	return &Budget{remaining: ops, deadline: time.Now().Add(total)}
}

// NewDefaultBudget returns the frozen 1,024-operation, 30-minute budget.
func NewDefaultBudget() *Budget { return NewBudget(DefaultOperations, DefaultTotalBudget) }

// pinnedBinary is the absolute Git executable a long-lived host fixed at
// start; nil leaves each spawn to look up "git" on PATH.
var pinnedBinary atomic.Pointer[string]

// PinBinary makes every later spawn with an empty Options.Binary run path, so
// a Git placed on PATH after a host started never runs (MCPV0-016). The
// caller passes an absolute path it already resolved; the CLI never pins.
func PinBinary(path string) { pinnedBinary.Store(&path) }

func defaultBinary() string {
	if pinned := pinnedBinary.Load(); pinned != nil {
		return *pinned
	}
	return "git"
}

// Options configure one bounded Git invocation.
type Options struct {
	Binary      string // test seam; empty means the pinned Git, else "git"
	Dir         string
	Env         []string // complete child environment; nil means empty
	Stdin       []byte
	StdoutLimit int
	// StdoutSizeHint, when > 0, preallocates stdout's backing capacity to
	// min(StdoutSizeHint, StdoutLimit) instead of growing it off nil capacity
	// through Go's append growth ladder (roughly 5x the eventual payload).
	// Set it only when the caller can compute the exact expected output size
	// (e.g. a `git cat-file --batch` over blobs of known sizes); leave it
	// zero when StdoutLimit is a generic ceiling unrelated to the actual
	// output size, or preallocation would itself become the huge allocation.
	StdoutSizeHint int
	StderrLimit    int           // 0 means DefaultStderrLimit
	PerOpTimeout   time.Duration // 0 means DefaultPerOpTimeout
}

// limitedBuffer caps how many bytes one Git subprocess may return.
//
// data is a named field and is never embedded as bytes.Buffer. Embedding
// would promote ReadFrom, WriteString, WriteByte and WriteRune onto this
// type, and os/exec collects a non-*os.File stdout with io.Copy, which
// prefers an io.ReaderFrom destination over the capping Write below. An
// embedded buffer would therefore bypass the cap completely and never set
// exceeded, leaving every declared StdoutLimit/StderrLimit inert against a
// hostile or very large repository.
type limitedBuffer struct {
	mu       sync.Mutex
	data     []byte
	limit    int
	exceeded bool
	overrun  func()
}

// newLimitedBuffer constructs a limitedBuffer capped at limit, preallocating
// its backing capacity to hint when hint is positive. The preallocation is
// clamped to limit so a bogus or oversized hint cannot itself trigger a huge
// allocation; hint <= 0 leaves data nil, matching the prior unhinted growth.
func newLimitedBuffer(limit, hint int, overrun func()) *limitedBuffer {
	capacity := hint
	if capacity > limit {
		capacity = limit
	}
	var data []byte
	if capacity > 0 {
		data = make([]byte, 0, capacity)
	}
	return &limitedBuffer{data: data, limit: limit, overrun: overrun}
}

func (b *limitedBuffer) Write(data []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	remaining := b.limit - len(b.data)
	if remaining < len(data) {
		if !b.exceeded {
			b.exceeded = true
			if b.overrun != nil {
				b.overrun()
			}
		}
		if remaining > 0 {
			b.data = append(b.data, data[:remaining]...)
		}
		return len(data), nil
	}
	b.data = append(b.data, data...)
	return len(data), nil
}

func (b *limitedBuffer) snapshot() ([]byte, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.data, b.exceeded
}

// ReserveOperation charges one logical operation using the same count and wall
// bounds as Run. Immutable request memo hits consume this budget without a child.
func (b *Budget) ReserveOperation(perOp time.Duration) (time.Duration, error) {
	if b.stable != nil {
		reservation, err := b.Reserve()
		if err != nil {
			return 0, err
		}
		return reservation.Deadline.Sub(b.stable.now()), nil
	}
	if b.remaining <= 0 {
		return 0, cemcode.New(cemcode.GitBudgetExceeded, "Git operation budget exhausted")
	}
	b.remaining--
	untilDeadline := time.Until(b.deadline)
	if untilDeadline <= 0 {
		return 0, cemcode.New(cemcode.GitTimeout, "Git verification budget elapsed")
	}
	if perOp == 0 {
		perOp = DefaultPerOpTimeout
	}
	if untilDeadline < perOp {
		perOp = untilDeadline
	}
	return perOp, nil
}

// Run executes one bounded Git operation and returns its stdout bytes.
func Run(ctx context.Context, budget *Budget, options Options, args ...string) ([]byte, error) {
	return runWithStream(ctx, budget, options, nil, args...)
}

// RunStream sends stdout to a trusted, synchronous, nonblocking consumer.
// The consumer owns stdout's memory and framing bounds; StdoutLimit and
// StdoutSizeHint apply only to Run. All other process bounds remain in force.
// Consumer state is provisional until this call and its framing check succeed.
func RunStream(ctx context.Context, budget *Budget, options Options, consumer io.Writer, args ...string) error {
	if consumer == nil {
		return cemcode.New(cemcode.InvalidArguments, "Git stdout consumer is required")
	}
	_, err := runWithStream(ctx, budget, options, consumer, args...)
	return err
}

func runWithStream(ctx context.Context, budget *Budget, options Options, consumer io.Writer, args ...string) ([]byte, error) {
	if budget.stable != nil {
		reservation, err := budget.Reserve()
		if err != nil {
			return nil, err
		}
		return runOwned(ctx, budget.stable, reservation, false, options, consumer, args...)
	}
	perOp, err := budget.ReserveOperation(options.PerOpTimeout)
	if err != nil {
		return nil, err
	}
	return runReservedStream(ctx, perOp, options, consumer, args...)
}

// RunReserved executes one bounded Git operation whose budget operation the
// caller already reserved, with perOp as its timeout. A Session request that
// must be replayed as its original one-shot invocation uses it, so the logical
// operation is charged exactly once.
func RunReserved(ctx context.Context, perOp time.Duration, options Options, args ...string) ([]byte, error) {
	return runReservedStream(ctx, perOp, options, nil, args...)
}

func runReservedStream(ctx context.Context, perOp time.Duration, options Options, consumer io.Writer, args ...string) ([]byte, error) {
	binary := options.Binary
	if binary == "" {
		binary = defaultBinary()
	}
	command := exec.Command(binary, args...)
	command.Dir = options.Dir
	command.Env = options.Env
	if command.Env == nil {
		command.Env = []string{}
	}
	if options.Stdin != nil {
		command.Stdin = bytes.NewReader(options.Stdin)
	}
	command.WaitDelay = pipeDrainDelay
	containChild(command)

	overrun := make(chan struct{})
	var overrunOnce sync.Once
	stderrLimit := options.StderrLimit
	if stderrLimit == 0 {
		stderrLimit = DefaultStderrLimit
	}
	signalOverrun := func() { overrunOnce.Do(func() { close(overrun) }) }
	sizeHint := options.StdoutSizeHint
	if consumer != nil {
		sizeHint = 0
	}
	stdout := newLimitedBuffer(options.StdoutLimit, sizeHint, signalOverrun)
	stderr := newLimitedBuffer(stderrLimit, 0, signalOverrun)
	command.Stdout, command.Stderr = stdout, stderr
	var stream *streamOutput
	if consumer != nil {
		stream = &streamOutput{consumer: consumer, abort: signalOverrun}
		command.Stdout = stream
	}

	if err := command.Start(); err != nil {
		return nil, cemcode.NewGitStartFailure(fmt.Sprintf("Git could not start: %v", err), err)
	}
	waited := make(chan error, 1)
	go func() { waited <- groupreap.Wait(command) }()

	timer := time.NewTimer(perOp)
	defer timer.Stop()
	var failure *cemcode.Error
	select {
	case runErr := <-waited:
		if streamErr := stream.failure(); streamErr != nil {
			return nil, streamErr
		}
		if _, exceeded := stdout.snapshot(); exceeded {
			failure = cemcode.New(cemcode.GitOutputExceeded, "Git stdout exceeded its byte bound")
		} else if _, exceeded := stderr.snapshot(); exceeded {
			failure = cemcode.New(cemcode.GitOutputExceeded, "Git stderr exceeded its byte bound")
		} else if runErr != nil {
			errText, _ := stderr.snapshot()
			exitCode := -1
			var exitError *exec.ExitError
			if errors.As(runErr, &exitError) {
				exitCode = exitError.ExitCode()
			}
			failure = cemcode.NewGitExitFailure(
				"Git exited unsuccessfully: "+firstLine(errText), exitCode, errText,
			)
		}
	case <-overrun:
		killGroupThenReap(command, waited)
		if streamErr := stream.failure(); streamErr != nil {
			return nil, streamErr
		}
		failure = cemcode.New(cemcode.GitOutputExceeded, "Git output exceeded its byte bound")
	case <-ctx.Done():
		killGroupThenReap(command, waited)
		failure = cemcode.New(cemcode.GitCancelled, "Git operation cancelled")
	case <-timer.C:
		killGroupThenReap(command, waited)
		failure = cemcode.New(cemcode.GitTimeout, "Git operation timed out")
	}
	if failure != nil {
		return nil, failure
	}
	data, _ := stdout.snapshot()
	return data, nil
}

func killGroupThenReap(command *exec.Cmd, waited <-chan error) {
	killGroup(command)
	<-waited
}

// pipeDrainDelay bounds how long a reap waits, after the child exits, for
// pipes still held by a descendant that left the child's process group. It is
// a hang detector: a forced close makes Wait fail, so output is never silently
// truncated.
var pipeDrainDelay = DefaultPerOpTimeout

func firstLine(data []byte) string {
	for index, c := range data {
		if c == '\n' {
			return string(data[:index])
		}
	}
	return string(data)
}

// ProcessContainment is the code of a refusal caused by process ownership or
// cleanup that was not proven. It is never retried or downgraded.
const ProcessContainment = "unsupported-process-containment"

// EmergencyAllowance is the single, non-renewable retirement allowance that
// starts at the first caller cancellation or outer expiry.
const EmergencyAllowance = 10 * time.Second

// watchInterval is how often a stable wait re-reads its one clock.
const watchInterval = time.Millisecond

// Event is one synchronous lifecycle notification of a stable budget.
type Event struct {
	// Name is one of: reserved, op-start, retire-started, cause-committed,
	// released, hold, session-close-start, refused-before-spawn.
	// cause-committed is the commitment point: the outer cause is sampled
	// immediately before it and a later outer expiry does not replace the
	// committed cause.
	Name    string
	Ordinal int
	Session bool
	Replay  bool
}

// Seam is the test seam of a stable budget. The zero value is production.
type Seam struct {
	Now        func() time.Time
	Event      func(Event)
	Command    func(ordinal int, replay bool, binary string, args []string) (string, []string)
	Primitives func(ordinal int, session bool) groupreap.Primitives
}

// Reservation is one charged logical operation: its ordinal and its absolute
// deadline. A replay of the same logical operation reuses it unchanged.
type Reservation struct {
	Ordinal  int
	Deadline time.Time
}

type stableState struct {
	mu        sync.Mutex
	seam      Seam
	deadline  time.Time
	expire    func()
	expired   bool
	ordinal   int
	held      bool
	holds     []*groupreap.Owner // retained handles of owners in HOLD
	emergency time.Time
}

// NewStableBudget returns a budget whose every child is run by the owned
// runner under one clock: ops logical operations before the absolute outer
// deadline. expire is called once when the budget first observes that
// deadline passed. Only the Stable verifier constructs one.
func NewStableBudget(ops int, deadline time.Time, expire func(), seam Seam) *Budget {
	return &Budget{remaining: ops, deadline: deadline, stable: &stableState{seam: seam, deadline: deadline, expire: expire}}
}

// Stable reports whether the budget routes to the owned runner.
func (b *Budget) Stable() bool { return b.stable != nil }

// Held reports whether any owned process reached HOLD under this budget.
func (b *Budget) Held() bool {
	if b.stable == nil {
		return false
	}
	b.stable.mu.Lock()
	defer b.stable.mu.Unlock()
	return b.stable.held
}

// Now reads the budget's clock.
func (b *Budget) Now() time.Time {
	if b.stable == nil {
		return time.Now()
	}
	return b.stable.now()
}

// OuterExpired samples the outer deadline, calling expire on first expiry.
func (b *Budget) OuterExpired() bool { return b.stable != nil && b.stable.outerExpired() }

// AnchorEmergency anchors the emergency allowance at a caller cancellation.
// Only the first terminal event anchors it.
func (b *Budget) AnchorEmergency() {
	if b.stable != nil {
		b.stable.anchor(b.stable.now())
	}
}

// Notify emits one verifier-level seam event.
func (b *Budget) Notify(name string) {
	if b.stable != nil {
		b.stable.event(Event{Name: name})
	}
}

// Reserve charges one logical operation of a stable budget. A held budget
// refuses before any spawn; the count bound precedes the outer deadline.
func (b *Budget) Reserve() (Reservation, error) {
	st := b.stable
	st.mu.Lock()
	if st.held {
		st.mu.Unlock()
		return Reservation{}, containment()
	}
	if b.remaining <= 0 {
		st.ordinal++
		ordinal := st.ordinal
		st.mu.Unlock()
		st.event(Event{Name: "refused-before-spawn", Ordinal: ordinal})
		return Reservation{}, cemcode.New(cemcode.GitBudgetExceeded, "Git operation budget exhausted")
	}
	b.remaining--
	st.ordinal++
	ordinal := st.ordinal
	st.mu.Unlock()
	if st.outerExpired() {
		return Reservation{}, cemcode.New(cemcode.GitCancelled, "Git operation cancelled")
	}
	deadline := st.now().Add(DefaultPerOpTimeout)
	if deadline.After(st.deadline) {
		deadline = st.deadline
	}
	st.event(Event{Name: "reserved", Ordinal: ordinal})
	return Reservation{Ordinal: ordinal, Deadline: deadline}, nil
}

func containment() *cemcode.Error {
	return cemcode.New(ProcessContainment, "owned Git process cleanup was not observed")
}

func (st *stableState) now() time.Time {
	if st.seam.Now != nil {
		return st.seam.Now()
	}
	return time.Now()
}

func (st *stableState) event(event Event) {
	if st.seam.Event != nil {
		st.seam.Event(event)
	}
}

func (st *stableState) anchor(at time.Time) {
	st.mu.Lock()
	if st.emergency.IsZero() {
		st.emergency = at
	}
	st.mu.Unlock()
}

func (st *stableState) outerExpired() bool {
	if st.now().Before(st.deadline) {
		return false
	}
	st.anchor(st.deadline)
	st.mu.Lock()
	first := !st.expired
	st.expired = true
	st.mu.Unlock()
	if first && st.expire != nil {
		st.expire()
	}
	return true
}

// terminal reports caller cancellation or outer expiry, anchoring the
// emergency allowance at the first one.
func (st *stableState) terminal(ctx context.Context) bool {
	if st.outerExpired() {
		return true
	}
	if ctx.Err() != nil {
		st.anchor(st.now())
		return true
	}
	return false
}

// until returns a channel closed once cond holds on the budget's clock.
func (st *stableState) until(cond func() bool) (<-chan struct{}, func()) {
	reached, stop := make(chan struct{}), make(chan struct{})
	go func() {
		ticker := time.NewTicker(watchInterval)
		defer ticker.Stop()
		for {
			if cond() {
				close(reached)
				return
			}
			select {
			case <-stop:
				return
			case <-ticker.C:
			}
		}
	}()
	return reached, sync.OnceFunc(func() { close(stop) })
}

// retireLimit bounds one teardown: ten seconds from start within the outer
// deadline, or, once a terminal event occurred, the single emergency
// allowance anchored at the first terminal event.
func (st *stableState) retireLimit(ctx context.Context, start time.Time) (<-chan struct{}, func()) {
	return st.until(func() bool {
		if st.terminal(ctx) {
			st.mu.Lock()
			anchor := st.emergency
			st.mu.Unlock()
			return !st.now().Before(anchor.Add(EmergencyAllowance))
		}
		return !st.now().Before(start.Add(DefaultPerOpTimeout))
	})
}

// finish retires one owned group within its bound and records a HOLD.
func (st *stableState) finish(ctx context.Context, owner *groupreap.Owner, event Event) groupreap.Result {
	limit, stop := st.retireLimit(ctx, st.now())
	result := owner.Finish(limit)
	stop()
	event.Name = "released"
	if result.State != groupreap.Released {
		st.mu.Lock()
		st.held = true
		st.holds = append(st.holds, owner)
		st.mu.Unlock()
		event.Name = "hold"
	}
	st.event(event)
	return result
}

func (st *stableState) primitives(ordinal int, session bool) groupreap.Primitives {
	if st.seam.Primitives != nil {
		return st.seam.Primitives(ordinal, session)
	}
	return groupreap.Primitives{}
}

func (st *stableState) rewrite(ordinal int, replay bool, binary string, args []string) (string, []string) {
	if binary == "" {
		binary = defaultBinary()
	}
	if st.seam.Command != nil {
		return st.seam.Command(ordinal, replay, binary, args)
	}
	return binary, args
}

// RunReservation replays one already reserved logical operation of a stable
// budget as a one-shot child. It charges nothing and keeps the deadline.
func RunReservation(ctx context.Context, budget *Budget, reservation Reservation, options Options, args ...string) ([]byte, error) {
	return runOwned(ctx, budget.stable, reservation, true, options, nil, args...)
}

// runOwned runs one child under a groupreap.Owner. Every cause is latched and
// the result is arbitrated only after owned cleanup completed: cleanup
// failure, outer cancellation or deadline, per-operation deadline, typed
// consumer failure, stdout then stderr overflow, launch failure, unsuccessful
// exit, success. No select branch order decides the result.
func runOwned(ctx context.Context, st *stableState, reservation Reservation, replay bool, options Options, consumer io.Writer, args ...string) ([]byte, error) {
	event := Event{Ordinal: reservation.Ordinal, Replay: replay}
	st.mu.Lock()
	held := st.held
	st.mu.Unlock()
	if held || !groupreap.OwnerAvailable() {
		return nil, containment()
	}
	if st.terminal(ctx) {
		return nil, cemcode.New(cemcode.GitCancelled, "Git operation cancelled")
	}
	binary, args := st.rewrite(reservation.Ordinal, replay, options.Binary, args)
	command := exec.Command(binary, args...)
	command.Dir = options.Dir
	command.Env = options.Env
	if command.Env == nil {
		command.Env = []string{}
	}
	if options.Stdin != nil {
		command.Stdin = bytes.NewReader(options.Stdin)
	}
	command.WaitDelay = pipeDrainDelay

	overrun := make(chan struct{})
	var overrunOnce sync.Once
	stderrLimit := options.StderrLimit
	if stderrLimit == 0 {
		stderrLimit = DefaultStderrLimit
	}
	signalOverrun := func() { overrunOnce.Do(func() { close(overrun) }) }
	sizeHint := options.StdoutSizeHint
	if consumer != nil {
		sizeHint = 0
	}
	stdout := newLimitedBuffer(options.StdoutLimit, sizeHint, signalOverrun)
	stderr := newLimitedBuffer(stderrLimit, 0, signalOverrun)
	command.Stdout, command.Stderr = stdout, stderr
	var stream *streamOutput
	if consumer != nil {
		stream = &streamOutput{consumer: consumer, abort: signalOverrun}
		command.Stdout = stream
	}

	event.Name = "op-start"
	st.event(event)
	owner, err := groupreap.StartWith(command, st.primitives(reservation.Ordinal, false))
	if err != nil {
		if st.terminal(ctx) {
			return nil, cemcode.New(cemcode.GitCancelled, "Git operation cancelled")
		}
		return nil, cemcode.NewGitStartFailure(fmt.Sprintf("Git could not start: %v", err), err)
	}
	expired := func() bool { return !st.now().Before(reservation.Deadline) }
	opExpired, stopWatch := st.until(expired)
	var outer, timedOut bool
	select {
	case <-owner.Exited():
		stopWatch()
		outer = st.terminal(ctx)
	case <-overrun:
		stopWatch()
		outer, timedOut = st.retire(ctx, owner, event, expired)
	case <-ctx.Done():
		stopWatch()
		outer, timedOut = st.retire(ctx, owner, event, expired)
	case <-opExpired:
		outer, timedOut = st.retire(ctx, owner, event, expired)
	}
	result := st.finish(ctx, owner, event)
	_, stdoutExceeded := stdout.snapshot()
	errText, stderrExceeded := stderr.snapshot()
	switch {
	case result.State != groupreap.Released:
		return nil, containment()
	case outer:
		return nil, cemcode.New(cemcode.GitCancelled, "Git operation cancelled")
	case timedOut:
		return nil, cemcode.New(cemcode.GitTimeout, "Git operation timed out")
	case stream.failure() != nil:
		return nil, stream.failure()
	case stdoutExceeded:
		return nil, cemcode.New(cemcode.GitOutputExceeded, "Git stdout exceeded its byte bound")
	case stderrExceeded:
		return nil, cemcode.New(cemcode.GitOutputExceeded, "Git stderr exceeded its byte bound")
	case result.WaitErr != nil:
		exitCode := -1
		var exitError *exec.ExitError
		if errors.As(result.WaitErr, &exitError) {
			exitCode = exitError.ExitCode()
		}
		return nil, cemcode.NewGitExitFailure("Git exited unsuccessfully: "+firstLine(errText), exitCode, errText)
	}
	data, _ := stdout.snapshot()
	return data, nil
}

// retire is the abnormal sequence up to the commitment point: the group is
// retired with zero grace, then the outer and per-operation causes are
// sampled once, then cause-committed is emitted. An outer expiry after that
// sample does not replace the committed cause.
func (st *stableState) retire(ctx context.Context, owner *groupreap.Owner, event Event, expired func() bool) (outer, timedOut bool) {
	owner.Stop()
	event.Name = "retire-started"
	st.event(event)
	outer, timedOut = st.terminal(ctx), expired()
	event.Name = "cause-committed"
	st.event(event)
	return outer, timedOut
}
