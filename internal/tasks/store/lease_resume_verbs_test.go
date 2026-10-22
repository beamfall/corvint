//go:build darwin || linux

package store

import (
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestPSRV0052_ReapResumesUnderWriter covers V1-1512 criterion 2 for reap:
// with the claimed lease expired, a targeted reap and a reap of every expired
// lease, each while a dispatcher commits after every preparation audit,
// resume on the writer route (PSR-V0-052) with one audit per transaction, so
// no SNAPSHOT_MOVED escapes; the attempt is reaped. The dispatcher shares the
// later clock so receipts stay in order. Rewritten-prefix detection on the
// resumed route is TestPSRV0053_ResumeKeepsRewrittenPrefixDetection.
func TestPSRV0052_ReapResumesUnderWriter(t *testing.T) {
	t.Parallel()
	if !journal.ResumableAudits() {
		t.Skip("platform reports no change time")
	}
	for _, tc := range []struct {
		name     string
		targeted bool
	}{{"targeted", true}, {"all-expired", false}} {
		t.Run(tc.name, func(t *testing.T) {
			f := newResumeFixture(t)
			at, err := time.Parse(time.RFC3339, string(f.now))
			if err != nil {
				t.Fatal(err)
			}
			f.now = wire.Timestamp(at.Add(61 * time.Minute).UTC().Format(time.RFC3339))
			l := transaction.LeaseRequest{Verb: transaction.LeaseReap}
			if tc.targeted {
				l.AttemptID, l.Generation = f.claim.AttemptID, f.claim.Generation
			}
			run := moving(t, f, f.lease("resume-reap-"+tc.name, l), -1, nil)
			if tc.targeted {
				// One reap transaction, resumed once on its one audit.
				if !run.completed() || run.timing.Resumed != 1 || run.audits != 1 || run.rep.AttemptID != f.claim.AttemptID {
					t.Fatalf("targeted reap under a moving journal: %+v %v resumed %d audits %d", run.rep, run.err, run.timing.Resumed, run.audits)
				}
				return
			}
			// A receiptless survey and one child reap transaction, each
			// resumed once on its one audit.
			if run.err != nil || run.timing.Resumed != 2 || run.audits != 2 || len(run.rep.Reaped) != 1 || run.rep.Reaped[0].AttemptID != f.claim.AttemptID || len(run.rep.ReapReceipts) != 1 {
				t.Fatalf("reap of every expired lease under a moving journal: %+v %v resumed %d audits %d", run.rep, run.err, run.timing.Resumed, run.audits)
			}
		})
	}
}

// TestPSRV0052_PoolReleaseResumesUnderWriter covers V1-1512 criterion 2 for
// pool release, which stays off the retained-checkpoint route: released
// while a dispatcher commits after every preparation audit, it resumes from
// its own audit (PSR-V0-052) and completes with one audit, so no
// SNAPSHOT_MOVED escapes.
func TestPSRV0052_PoolReleaseResumesUnderWriter(t *testing.T) {
	t.Parallel()
	if !journal.ResumableAudits() {
		t.Skip("platform reports no change time")
	}
	f := newResumeFixture(t)
	acquired := stagedWrite(f.acquire("release-acquire"), nil)
	if !acquired.completed() || acquired.rep.PoolAllocation == nil {
		t.Fatalf("acquire: %+v %v", acquired.rep, acquired.err)
	}
	l := transaction.LeaseRequest{Verb: transaction.LeasePoolRelease, AttemptID: f.claim.AttemptID, Generation: f.claim.Generation, Allocation: string(acquired.rep.PoolAllocation.AllocationID)}
	run := moving(t, f, f.lease("resume-pool-release", l), -1, nil)
	if !run.completed() || run.timing.Resumed != 1 || run.audits != 1 || run.rep.ReleasedPoolAllocation == nil {
		t.Fatalf("pool release under a moving journal: %+v %v resumed %d audits %d", run.rep, run.err, run.timing.Resumed, run.audits)
	}
}
