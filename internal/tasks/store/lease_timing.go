package store

import (
	"context"
	"time"
)

// LeaseTiming accumulates the wall time one lease command spends in each
// writer phase, summed over every lease transaction the command runs (a claim
// may first commit reap transactions). It is the issue 494 diagnostic behind
// `--timing`: never written to a receipt, request record, evidence, ranking
// or authority, and never an input to any decision.
//
// JournalWrite excludes Fsync; LockHold contains both. A collector belongs to
// one command and is not safe for concurrent use.
type LeaseTiming struct {
	Transactions  int
	Rounds        int
	AdmissionWait time.Duration // registered-order preparation admission
	Guards        time.Duration // writer guards and orphan-stage recovery
	SnapshotRead  time.Duration // head, inventory and full audit or verified cache reuse
	Validation    time.Duration // request replay lookup, model and handoff history audit
	MonitorClose  time.Duration // change-monitor teardown outside the lock
	LockWait      time.Duration
	LockHold      time.Duration
	JournalWrite  time.Duration // session, staged writes, links and redo, less Fsync
	Fsync         time.Duration // file and directory sync calls under the lock
}

type leaseTimingKey struct{}

// WithLeaseTiming returns a context whose lease writes add their phase times
// to t.
func WithLeaseTiming(ctx context.Context, t *LeaseTiming) context.Context {
	return context.WithValue(ctx, leaseTimingKey{}, t)
}

// leaseTimingOf returns the command's collector, or a discarded one, so
// writer code records unconditionally.
func leaseTimingOf(ctx context.Context) *LeaseTiming {
	if t, ok := ctx.Value(leaseTimingKey{}).(*LeaseTiming); ok && t != nil {
		return t
	}
	return &LeaseTiming{}
}

// prepared splits one preparation at the end of its snapshot read; a
// preparation that ended before the read finished is all snapshot read.
func (t *LeaseTiming) prepared(start, readEnd time.Time) {
	end := time.Now()
	if readEnd.IsZero() {
		t.SnapshotRead += end.Sub(start)
		return
	}
	t.SnapshotRead += readEnd.Sub(start)
	t.Validation += end.Sub(readEnd)
}

// wrote charges one journal write, separating its sync calls.
func (t *LeaseTiming) wrote(start time.Time, sync time.Duration) {
	t.Fsync += sync
	t.JournalWrite += time.Since(start) - sync
}
