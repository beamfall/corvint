package store

import (
	"context"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/journal"
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
	// Syncs counts those sync calls (LGL-V0-006, proposed). The CLI does not
	// report it.
	Syncs int
	// Resumed counts writes served by resuming from the command's own audit
	// from receipt 1 (PSR-V0-052, proposed). The CLI does not report it.
	Resumed int
	// The walk split (PSR-V0-151, V1-1626): how SnapshotRead divides into
	// complete walks from receipt 1, prefix walks to a captured head and
	// resumed walks from the command's base, with how many of the first two
	// ended SNAPSHOT_MOVED and how many resumes were declined. The detached
	// PENDING progress member reports it; `--timing` does not.
	CompleteWalks  int
	CompleteMoved  int
	PrefixWalks    int
	PrefixMoved    int
	ResumedWalks   int
	ResumeDeclines int
	CompleteWalk   time.Duration
	PrefixWalk     time.Duration
	ResumedWalk    time.Duration
}

// walked adds one write audit's walks.
func (t *LeaseTiming) walked(w journal.WalkStats) {
	t.CompleteWalks += w.Complete
	t.CompleteMoved += w.CompleteMoved
	t.PrefixWalks += w.Prefix
	t.PrefixMoved += w.PrefixMoved
	t.ResumedWalks += w.Resumed
	t.CompleteWalk += w.CompleteTime
	t.PrefixWalk += w.PrefixTime
	t.ResumedWalk += w.ResumedTime
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

type leaseProgressKey struct{}

// WithLeaseProgress returns a context whose lease writes call report with a
// copy of the command's timing after every preparation round, on the writing
// goroutine (PSR-V0-112, V1-1617). The copy is a diagnostic like the timing
// itself: report must not block for long and decides nothing.
func WithLeaseProgress(ctx context.Context, report func(LeaseTiming)) context.Context {
	return context.WithValue(ctx, leaseProgressKey{}, report)
}

// leaseProgressOf returns the command's progress callback, or one that
// discards.
func leaseProgressOf(ctx context.Context) func(LeaseTiming) {
	if f, ok := ctx.Value(leaseProgressKey{}).(func(LeaseTiming)); ok && f != nil {
		return f
	}
	return func(LeaseTiming) {}
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
func (t *LeaseTiming) wrote(start time.Time, sync time.Duration, syncs int) {
	t.Fsync += sync
	t.Syncs += syncs
	t.JournalWrite += time.Since(start) - sync
}
