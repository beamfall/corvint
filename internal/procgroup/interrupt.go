package procgroup

import (
	"context"
	"io"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

// stopSignalRecorder records the last stop signal run sent to the owned
// process group while the leader's exit was not yet observed (PWP-V0-029).
// Signal 0 probes and signals sent after the exit (straggler kills) are not
// stop signals and are not recorded.
//
// leaderHeld is set only when the exit was observed successfully, so the
// leader is held unreaped and pins the group ID; only then may the pre-reap
// grace consult the held-group proof (PGO-V0-022).
type stopSignalRecorder struct {
	leaderGone atomic.Bool
	leaderHeld atomic.Bool
	mu         sync.Mutex
	last       syscall.Signal
}

// exited records the leader's exit observation; observed is its error.
func (r *stopSignalRecorder) exited(observed error) {
	r.leaderHeld.Store(observed == nil)
	r.leaderGone.Store(true)
}

func (r *stopSignalRecorder) wrap(send func(int, syscall.Signal) error) func(int, syscall.Signal) error {
	return func(pid int, signal syscall.Signal) error {
		// The exit observation is read before the send: a signal that makes
		// the leader exit at once is still the stop signal that caused it.
		exited := r.leaderGone.Load()
		err := send(pid, signal)
		if signal != 0 && err == nil && !exited {
			r.mu.Lock()
			r.last = signal
			r.mu.Unlock()
		}
		return err
	}
}

func (r *stopSignalRecorder) name() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	switch r.last {
	case syscall.SIGINT:
		return "SIGINT"
	case syscall.SIGTERM:
		return "SIGTERM"
	case syscall.SIGKILL:
		return "SIGKILL"
	case 0:
		return ""
	default:
		return "unknown"
	}
}

// interruptBeforeStop sends SIGINT to the owned group of a timed-out leader
// and waits up to grace for its exit (PWP-V0-030). It reports whether the
// exit was observed, and that observation's error. Cancellation ends the grace.
func interruptBeforeStop(ctx context.Context, pid int, grace time.Duration, signal func(int, syscall.Signal) error, exited <-chan error) (bool, error) {
	if err := signal(pid, syscall.SIGINT); err != nil {
		return false, nil
	}
	timer := time.NewTimer(grace)
	defer stopProcessTimer(timer)
	select {
	case err := <-exited:
		return true, err
	case <-ctx.Done():
	case <-timer.C:
	}
	return false, nil
}

// teeQueueChunks bounds the console chunks a tee may hold while its sink is
// slow (one drain read each, at most 32 KiB). teeStallBound is how long a
// drain waits for room before the tee copy ends early (PWP-V0-028).
const (
	teeQueueChunks = 64
	teeStallBound  = time.Second
)

// teeFlushBound bounds how long Run waits, after the drains end, for the tee
// forwarders to hand their queued bytes to the sinks.
const teeFlushBound = 2 * time.Second

// IncompleteTee is implemented by a tee sink that wants to learn its copy
// ended early: its sink stalled past teeStallBound, failed, or had not
// accepted every byte when Run returned (PWP-V0-028).
type IncompleteTee interface {
	TeeIncomplete()
}

// teeOutput hands the bounded capture every byte first, then queues a copy
// for a forwarder goroutine that writes the tee sink. A sink slower than the
// child slows the drain as a direct write would; a sink that stalls past
// teeStallBound, or fails, ends that tee only, so it never holds the drain,
// the capture or the other stream for longer (PWP-V0-028).
type teeOutput struct {
	capture io.Writer
	tee     io.Writer
	mu      sync.Mutex
	closed  bool
	queue   chan []byte
	done    chan struct{}
	failed  atomic.Bool
}

func withTee(capture, tee io.Writer) (io.Writer, *teeOutput) {
	if tee == nil {
		return capture, nil
	}
	t := &teeOutput{capture: capture, tee: tee, queue: make(chan []byte, teeQueueChunks), done: make(chan struct{})}
	go t.forward()
	return t, t
}

func (t *teeOutput) Write(value []byte) (int, error) {
	n, err := t.capture.Write(value)
	t.enqueue(value)
	return n, err
}

func (t *teeOutput) enqueue(value []byte) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed || t.failed.Load() || len(value) == 0 {
		return
	}
	chunk := append([]byte(nil), value...)
	select {
	case t.queue <- chunk:
		return
	default:
	}
	timer := time.NewTimer(teeStallBound)
	defer stopProcessTimer(timer)
	select {
	case t.queue <- chunk:
	case <-timer.C:
		t.markFailed()
	}
}

func (t *teeOutput) forward() {
	defer close(t.done)
	for chunk := range t.queue {
		if !t.failed.Load() {
			if _, err := t.tee.Write(chunk); err != nil {
				t.markFailed()
			}
		}
	}
}

func (t *teeOutput) markFailed() {
	if t.failed.CompareAndSwap(false, true) {
		if sink, ok := t.tee.(IncompleteTee); ok {
			sink.TeeIncomplete()
		}
	}
}

func (t *teeOutput) close() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.closed {
		t.closed = true
		close(t.queue)
	}
}

// finishTees ends both tee queues and waits, up to deadline, for their
// forwarders. A forwarder still blocked on its sink is left to finish on its
// own; its tee is reported incomplete and Run does not wait for it.
func finishTees(deadline time.Time, tees ...*teeOutput) {
	for _, t := range tees {
		if t != nil {
			t.close()
		}
	}
	for _, t := range tees {
		if t == nil {
			continue
		}
		select {
		case <-t.done:
			continue
		default:
		}
		timer := time.NewTimer(max(time.Until(deadline), 0))
		select {
		case <-t.done:
		case <-timer.C:
			t.markFailed()
		}
		stopProcessTimer(timer)
	}
}
