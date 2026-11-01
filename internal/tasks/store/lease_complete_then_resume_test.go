//go:build darwin || linux

package store

import (
	"fmt"
	"os"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
)

// JSG-V0-780, V1-2592: a complete-route write on a segmented store retains a
// writer checkpoint with a base at the sealed sequence, so the next widen or
// reap resumes from the sealed base instead of paying the complete audit.
// Before the change the complete route retained FullSeq at head and every
// such widen and reap declined the resume ("no base at the sealed
// sequence").
func TestJSGV0780_CompleteRouteWriteThenWidenReapResume(t *testing.T) {
	t.Parallel()
	if !journal.ResumableAudits() {
		t.Skip("platform reports no change time")
	}
	for _, c := range sealedWidenReapCases[:2] {
		for _, route := range []string{"mutate", "claim"} {
			t.Run(c.name+"/"+route, func(t *testing.T) {
				t.Parallel()
				f := c.fixture(t)
				plainCheckpointStores.Delete(f.repo.StateDir)
				through := segmentThrough(t, f.repo)
				wcPath := journal.WriterCheckpointPath(f.repo.StateDir)
				for _, p := range []string{wcPath, wcPath + ".base"} {
					if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
						t.Fatal(err)
					}
				}
				// Without a checkpoint the write takes the complete route.
				var write writerRun
				switch route {
				case "mutate":
					write = writerMutate(t, f.repo, "complete-route", nil)
				case "claim":
					created := stagedWrite(writerCreate(f.repo, "complete-claim", f.now), nil)
					if !created.completed() {
						t.Fatalf("create: %+v %v", created.rep, created.err)
					}
					if err := os.Remove(wcPath); err != nil && !os.IsNotExist(err) {
						t.Fatal(err)
					}
					attemptEntropies.Store(f.repo.StateDir, fixedEntropy(77))
					write = stagedWrite(f.lease("complete-claim-lease", transaction.LeaseRequest{Verb: transaction.LeaseClaim, TicketID: created.rep.Ticket, Holder: "agent-9", LeaseMinutes: "60", Scope: []string{"other/"}}), nil)
				}
				if !write.completed() || write.fast() {
					t.Fatalf("%s did not take the complete route: %+v %v %v", route, write.rep, write.err, write.stages)
				}
				wc := readWriterCheckpoint(f.repo)
				if wc == nil || wc.FullSeq != through {
					t.Fatalf("complete-route %s retained %+v, want FullSeq %d (the sealed sequence)", route, wc, through)
				}
				run := sealedRun(t, f, f.lease("after-"+route+"-"+c.name, c.lease(f)), nil)
				if !run.completed() || run.timing.Resumed != c.resumed || run.audits != 0 {
					t.Fatalf("%s after a complete-route %s: %+v %v resumed %d audits %d stages %v", c.name, route, run.rep, run.err, run.timing.Resumed, run.audits, fmt.Sprint(run.stages))
				}
				mustSegmentAudit(t, f.repo, headSeqOf(t, f.repo))
			})
		}
	}
}
