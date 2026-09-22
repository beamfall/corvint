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
	"time"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
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
}

// NewBudget returns a budget of ops operations within total wall time.
func NewBudget(ops int, total time.Duration) *Budget {
	return &Budget{remaining: ops, deadline: time.Now().Add(total)}
}

// NewDefaultBudget returns the frozen 1,024-operation, 30-minute budget.
func NewDefaultBudget() *Budget { return NewBudget(DefaultOperations, DefaultTotalBudget) }

// Options configure one bounded Git invocation.
type Options struct {
	Binary      string // test seam; empty means "git"
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
		binary = "git"
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
	go func() { waited <- command.Wait() }()

	timer := time.NewTimer(perOp)
	defer timer.Stop()
	var failure *cemcode.Error
	select {
	case runErr := <-waited:
		// Normal completion path: sweep the group immediately after the reap.
		killDescendants(command)
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
