package main

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

// workDrainFixture drives workDrainCapture with a fake clock: each tick
// advances the clock by its gap and delivers one tick.
type workDrainFixture struct {
	drained chan error
	ticks   chan time.Time
	hang    chan time.Time
	closing chan struct{}
	clock   atomic.Int64
	closed  atomic.Int32
}

func newWorkDrainFixture() *workDrainFixture {
	return &workDrainFixture{drained: make(chan error, 2), ticks: make(chan time.Time), hang: make(chan time.Time), closing: make(chan struct{})}
}

func (fixture *workDrainFixture) closeReads() {
	if fixture.closed.Add(1) == 1 {
		close(fixture.closing)
		fixture.drained <- nil
		fixture.drained <- nil
	}
}

func (fixture *workDrainFixture) now() time.Time { return time.Unix(0, fixture.clock.Load()) }

func (fixture *workDrainFixture) run(t *testing.T, ctx context.Context, drive func()) error {
	t.Helper()
	done := make(chan error, 1)
	go func() {
		done <- workDrainCapture(ctx, fixture.drained, fixture.closeReads, fixture.ticks, fixture.hang, fixture.now)
	}()
	drive()
	select {
	case err := <-done:
		return err
	case <-time.After(10 * time.Second):
		t.Fatal("workDrainCapture did not return")
		return nil
	}
}

// tick advances the clock and delivers a tick; it reports false once the
// drain has closed the pipes instead of receiving it.
func (fixture *workDrainFixture) tick(gap time.Duration) bool {
	fixture.clock.Add(int64(gap))
	select {
	case fixture.ticks <- fixture.now():
		return true
	case <-fixture.closing:
		return false
	}
}

// WQO-V0-920 (V1-2741): a host stall delivers the expiry and EOF together.
// Late tick intervals do not count as awake time, so the capture completes.
func TestWorkDrainCaptureIgnoresHostStalls(t *testing.T) {
	fixture := newWorkDrainFixture()
	err := fixture.run(t, context.Background(), func() {
		for range 20 {
			fixture.tick(10 * time.Second)
		}
		fixture.tick(workDrainTick)
		fixture.drained <- nil
		fixture.drained <- nil
	})
	if err != nil || fixture.closed.Load() != 0 {
		t.Fatalf("stalled drain = %v closed=%d, want a complete capture", err, fixture.closed.Load())
	}
}

// WQO-V0-034: an idle pipe holder watched while the process is awake still
// expires after workDrainIdle and is reported as incomplete capture.
func TestWorkDrainCaptureExpiresWhileAwake(t *testing.T) {
	fixture := newWorkDrainFixture()
	ticks := 0
	err := fixture.run(t, context.Background(), func() {
		for fixture.tick(workDrainTick) {
			ticks++
		}
	})
	if err == nil || err.Error() != "incomplete pipe capture" || fixture.closed.Load() != 1 {
		t.Fatalf("idle drain = %v closed=%d", err, fixture.closed.Load())
	}
	// The drain's first clock reading may follow the first advance, which
	// then counts as a zero gap.
	if want := int(workDrainIdle / workDrainTick); ticks != want && ticks != want+1 {
		t.Fatalf("expired after %d on-time ticks, want %d", ticks, want)
	}
}

// WQO-V0-920: the wall-time hang detector ends a drain on a stalled host.
func TestWorkDrainCaptureHangDetector(t *testing.T) {
	fixture := newWorkDrainFixture()
	err := fixture.run(t, context.Background(), func() { fixture.hang <- fixture.now() })
	if err == nil || err.Error() != "incomplete pipe capture" || fixture.closed.Load() != 1 {
		t.Fatalf("hung drain = %v closed=%d", err, fixture.closed.Load())
	}
}

// WQO-V0-920: cancellation closes the pipes at once and leaves the error to
// the caller, which reports the context error.
func TestWorkDrainCaptureStopsOnCancellation(t *testing.T) {
	fixture := newWorkDrainFixture()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := fixture.run(t, ctx, func() {})
	if err != nil || fixture.closed.Load() != 1 {
		t.Fatalf("cancelled drain = %v closed=%d", err, fixture.closed.Load())
	}
}

// WQO-V0-032: a copy error is incomplete capture without closing the pipes.
func TestWorkDrainCaptureCopyError(t *testing.T) {
	fixture := newWorkDrainFixture()
	err := fixture.run(t, context.Background(), func() {
		fixture.drained <- context.DeadlineExceeded
		fixture.drained <- nil
	})
	if err == nil || err.Error() != "incomplete pipe capture" || fixture.closed.Load() != 0 {
		t.Fatalf("copy error drain = %v closed=%d", err, fixture.closed.Load())
	}
}
