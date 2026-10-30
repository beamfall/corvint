//go:build darwin || linux

package store

import (
	"context"
	"fmt"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
)

// PSR-V0-115 (V1-1617, GitHub #1063): a journal whose heartbeat receipts
// `receipt compact` elided still yields a resume base. The prefix walk
// restores each elided receipt, the acquire resumes after one complete
// audit, and compaction is not what keeps an acquire on complete audits.
func TestPSRV0115_ResumeAfterCompaction(t *testing.T) {
	t.Parallel()
	if !journal.ResumableAudits() {
		t.Skip("platform reports no change time")
	}
	f := newResumeFixture(t)
	c := f.otherClaims(t, 1)[0]
	for i := range 3 {
		run := stagedWrite(f.lease(fmt.Sprintf("compact-heartbeat-%d", i), transaction.LeaseRequest{Verb: transaction.LeaseHeartbeat, AttemptID: c.AttemptID, Generation: c.Generation}), nil)
		if !run.completed() {
			t.Fatalf("heartbeat %d: %+v %v", i, run.rep, run.err)
		}
		c = run.rep
	}
	rep, err := CompactLiveness(context.Background(), f.repo, false)
	if err != nil || rep.Elided == 0 {
		t.Fatalf("compact: %+v %v", rep, err)
	}
	run := moving(t, f, f.acquire("compact-acquire"), 1, nil)
	if !run.completed() || run.timing.Resumed != 1 || run.audits != 1 {
		t.Fatalf("acquire after compaction: %+v %v resumed %d audits %d stages %v", run.rep, run.err, run.timing.Resumed, run.audits, run.stages)
	}
}
