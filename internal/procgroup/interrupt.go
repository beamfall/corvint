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
type stopSignalRecorder struct {
	leaderGone atomic.Bool
	mu         sync.Mutex
	last       syscall.Signal
}

func (r *stopSignalRecorder) wrap(send func(int, syscall.Signal) error) func(int, syscall.Signal) error {
	return func(pid int, signal syscall.Signal) error {
		err := send(pid, signal)
		if signal != 0 && err == nil && !r.leaderGone.Load() {
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

// teeOutput copies every byte the child writes to tee before the bounded
// capture sees it, so the tee is never truncated (PWP-V0-028). The first tee
// write error stops the tee; the capture and the run are unaffected.
type teeOutput struct {
	capture io.Writer
	tee     io.Writer
	failed  bool
}

func withTee(capture, tee io.Writer) io.Writer {
	if tee == nil {
		return capture
	}
	return &teeOutput{capture: capture, tee: tee}
}

func (t *teeOutput) Write(value []byte) (int, error) {
	if !t.failed {
		if _, err := t.tee.Write(value); err != nil {
			t.failed = true
		}
	}
	return t.capture.Write(value)
}
