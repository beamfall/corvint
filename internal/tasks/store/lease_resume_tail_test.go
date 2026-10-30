//go:build darwin || linux

package store

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// otherClaims claims n more tickets for other holders, as a dispatcher's
// workers hold them while an agent acquires from the pool.
func (f resumeFixture) otherClaims(t *testing.T, n int) []*Report {
	t.Helper()
	var claims []*Report
	for i := range n {
		created := stagedWrite(writerCreate(f.repo, fmt.Sprintf("tail-ticket-%d", i), f.now), nil)
		if !created.completed() {
			t.Fatalf("create %d: %+v %v", i, created.rep, created.err)
		}
		// newResumeFixture fixed the attempt entropy; each claim needs its own.
		attemptEntropies.Store(f.repo.StateDir, fixedEntropy(byte(40+i)))
		claim := stagedWrite(f.lease(fmt.Sprintf("tail-claim-%d", i), transaction.LeaseRequest{Verb: transaction.LeaseClaim, TicketID: created.rep.Ticket, Holder: fmt.Sprintf("worker-%d", i), LeaseMinutes: "60", Scope: []string{fmt.Sprintf("w%d/", i)}}), nil)
		if !claim.completed() {
			t.Fatalf("claim %d: %+v %v", i, claim.rep, claim.err)
		}
		claims = append(claims, claim.rep)
	}
	return claims
}

// heartbeat commits a heartbeat of c, the keepLease write of issue 1010.
func (f resumeFixture) heartbeat(t *testing.T, id string, c *Report) {
	t.Helper()
	run := stagedWrite(f.lease(id, transaction.LeaseRequest{Verb: transaction.LeaseHeartbeat, AttemptID: c.AttemptID, Generation: c.Generation}), nil)
	if !run.completed() {
		t.Fatalf("heartbeat %s: %+v %v", id, run.rep, run.err)
	}
}

// replaceKeepingStamps replaces the first old in file p with new and
// restores its modification time.
func replaceKeepingStamps(t *testing.T, p string, old, new []byte) {
	t.Helper()
	info, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(raw, old) {
		t.Fatalf("%s holds no %q: %s", p, old, raw)
	}
	historyWrite(t, p, bytes.Replace(raw, old, new, 1))
	if err := os.Chtimes(p, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
}

// PSR-V0-113 (V1-1617): a tail that is not only heartbeats is decided on
// the tail, never on the audited head. Two other workers acquire both pool
// members after the acquire's base audit; the resumed write re-models at
// the new head and does not allocate a member its stale FREE view showed,
// and it decides exactly as the complete route decides that journal.
func TestPSRV0113_ResumeDecidesNonHeartbeatTail(t *testing.T) {
	t.Parallel()
	if !journal.ResumableAudits() {
		t.Skip("platform reports no change time")
	}
	f := newResumeFixture(t)
	others := f.otherClaims(t, 2)
	run := moving(t, f, f.acquire("tail-acquire"), 1, func(int) {
		f.heartbeat(t, "tail-heartbeat", others[0])
		for i, c := range others {
			got := stagedWrite(f.lease(fmt.Sprintf("tail-pool-%d", i), transaction.LeaseRequest{Verb: transaction.LeasePoolAcquire, AttemptID: c.AttemptID, Generation: c.Generation, Pool: "db"}), nil)
			if !got.completed() || got.rep.PoolAllocation == nil {
				t.Fatalf("worker %d acquire: %+v %v", i, got.rep, got.err)
			}
		}
	})
	if run.completed() && run.rep.PoolAllocation != nil {
		t.Fatalf("acquire allocated %+v from the audited head's FREE view", run.rep.PoolAllocation)
	}
	if run.timing.Resumed != 1 || run.audits != 1 {
		t.Fatalf("resumed %d, complete audits %d, stages %v", run.timing.Resumed, run.audits, run.stages)
	}
	oracle := stagedWrite(f.acquire("tail-oracle"), nil)
	if oracle.completed() && oracle.rep.PoolAllocation != nil {
		t.Fatalf("complete route allocated %+v with both members held", oracle.rep.PoolAllocation)
	}
	sameDecision(t, run.writerRun, oracle)
}

// PSR-V0-113 (V1-1617): a heartbeat in the tail that changes pool membership
// or allocation, a forged receipt kind and a forged request record are never
// committed over. Each is written after the heartbeat commits with its
// modification time restored; the resumed write declines, the head stays,
// and the acquire ends with the complete route's own decision for that
// journal. No receipt kind or verb is trusted to bound what a tail changed.
func TestPSRV0113_ForgedHeartbeatTailNeverCommits(t *testing.T) {
	t.Parallel()
	if !journal.ResumableAudits() {
		t.Skip("platform reports no change time")
	}
	for _, tc := range []struct {
		name   string
		change func(t *testing.T, f resumeFixture)
	}{
		{"heartbeat-adds-member", func(t *testing.T, f resumeFixture) {
			replaceKeepingStamps(t, filepath.Join(f.repo.IntentRoot(), ".taskman", "policy.json"), []byte(`"members":["a","b"]`), []byte(`"members":["a","b","c"]`))
		}},
		{"heartbeat-frees-allocation", func(t *testing.T, f resumeFixture) {
			replaceKeepingStamps(t, filepath.Join(f.repo.StateDir, "pools.json"), []byte(`"ALLOCATED"`), []byte(`"FREE"`))
		}},
		{"forged-receipt-kind", forgeHeadReceiptKind},
		{"forged-request-record", forgeHeartbeatRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newResumeFixture(t)
			others := f.otherClaims(t, 1)
			// One allocation the forged view could free.
			held := stagedWrite(f.lease("forged-pool", transaction.LeaseRequest{Verb: transaction.LeasePoolAcquire, AttemptID: others[0].AttemptID, Generation: others[0].Generation, Pool: "db"}), nil)
			if !held.completed() || held.rep.PoolAllocation == nil {
				t.Fatalf("held allocation: %+v %v", held.rep, held.err)
			}
			others[0] = held.rep
			var tampered uint64
			run := moving(t, f, f.acquire("forged-acquire"), 1, func(int) {
				f.heartbeat(t, "forged-heartbeat", others[0])
				tc.change(t, f)
				tampered = headSeqOf(t, f.repo)
			})
			if run.completed() || run.fast() || run.timing.Resumed != 0 {
				t.Fatalf("acquire over a forged tail: %+v %v resumed %d stages %v", run.rep, run.err, run.timing.Resumed, run.stages)
			}
			if wire.CodeOf(run.err) == wire.CodeSnapshotMoved {
				t.Fatalf("held journal answered as moved: %v", run.err)
			}
			if got := headSeqOf(t, f.repo); got != tampered {
				t.Fatalf("head moved from %d to %d over a forged tail", tampered, got)
			}
			oracle := stagedWrite(f.acquire("forged-oracle"), nil)
			sameDecision(t, run.writerRun, oracle)
		})
	}
}

// forgeHeadReceiptKind rewrites the head receipt's kind, keeping its stamps.
func forgeHeadReceiptKind(t *testing.T, f resumeFixture) {
	name, _ := snapshot.ReceiptName(headSeqOf(t, f.repo))
	p := filepath.Join(f.repo.StateDir, "receipts", name)
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	rc, err := snapshot.DecodeReceipt(raw)
	if err != nil {
		t.Fatal(err)
	}
	forged := "MUTATION"
	if rc.Kind == forged {
		forged = "TRANSITION"
	}
	replaceKeepingStamps(t, p, []byte(`"kind":"`+rc.Kind+`"`), []byte(`"kind":"`+forged+`"`))
}

// forgeHeartbeatRequest rewrites the "forged-heartbeat" request record to a
// well-formed digest of another mutation, keeping its stamps.
func forgeHeartbeatRequest(t *testing.T, f resumeFixture) {
	path, err := snapshot.RequestPath("forged-heartbeat")
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(f.repo.StateDir, path)
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	// A well-formed digest of another mutation.
	key := []byte(`"mutationSha256":"`)
	i := bytes.Index(raw, key) + len(key)
	digit := raw[i : i+1]
	swapped := []byte{'0'}
	if digit[0] == '0' {
		swapped[0] = '1'
	}
	replaceKeepingStamps(t, p, append(append([]byte{}, key...), digit...), append(append([]byte{}, key...), swapped...))
}

// PSR-V0-116 (V1-1617): a tail receipt or tail request record rewritten
// after the resumed audit walked it, under the writer lock and after
// modelling, is caught by the resume recheck: the write declines, the head
// stays, and the acquire ends with the complete route's own decision.
func TestPSRV0116_TailRewrittenAfterAuditNeverCommits(t *testing.T) {
	t.Parallel()
	if !journal.ResumableAudits() {
		t.Skip("platform reports no change time")
	}
	for _, tc := range []struct {
		name   string
		change func(t *testing.T, f resumeFixture)
	}{
		{"tail-receipt-kind", forgeHeadReceiptKind},
		{"tail-request-record", forgeHeartbeatRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newResumeFixture(t)
			others := f.otherClaims(t, 1)
			var posted, tampered uint64
			run := movingStaged(t, f, f.acquire("late-acquire"), 1, func(int) {
				f.heartbeat(t, "forged-heartbeat", others[0])
				posted = headSeqOf(t, f.repo)
			}, func(stage string) {
				if tampered == 0 && posted != 0 && strings.HasPrefix(stage, "fast.lease.model=") {
					tc.change(t, f)
					tampered = headSeqOf(t, f.repo)
				}
			})
			if tampered == 0 {
				t.Fatalf("the resumed write never reached fast.lease.model after the heartbeat: stages %v", run.stages)
			}
			if run.completed() || run.fast() || run.timing.Resumed != 0 {
				t.Fatalf("acquire over a tail rewritten after its audit: %+v %v resumed %d stages %v", run.rep, run.err, run.timing.Resumed, run.stages)
			}
			if !slices.ContainsFunc(run.stages, func(s string) bool { return strings.Contains(s, "resume recheck") }) {
				t.Fatalf("declined before the resume recheck: stages %v", run.stages)
			}
			if got := headSeqOf(t, f.repo); got != tampered {
				t.Fatalf("head moved from %d to %d over a rewritten tail", tampered, got)
			}
			oracle := stagedWrite(f.acquire("late-oracle"), nil)
			sameDecision(t, run.writerRun, oracle)
		})
	}
}

// PSR-V0-112 (V1-1617): a lease write reports a copy of its timing after
// every preparation round, on the writing goroutine, so a detached child can
// keep its round count and audit time where a PENDING attach reads them.
func TestPSRV0112_LeaseProgressAfterEachRound(t *testing.T) {
	t.Parallel()
	if !journal.ResumableAudits() {
		t.Skip("platform reports no change time")
	}
	f := newResumeFixture(t)
	var seen []LeaseTiming
	acquire := f.acquire("progress-acquire")
	run := moving(t, f, func(ctx context.Context) (*Report, error) {
		return acquire(WithLeaseProgress(ctx, func(t LeaseTiming) { seen = append(seen, t) }))
	}, -1, nil)
	if !run.completed() || run.timing.Resumed != 1 {
		t.Fatalf("acquire: %+v %v resumed %d", run.rep, run.err, run.timing.Resumed)
	}
	if len(seen) != 1 || seen[0].Rounds != 1 || seen[0].Resumed != 0 || seen[0].SnapshotRead <= 0 {
		t.Fatalf("progress %+v, want one report after the first round's audit", seen)
	}
}
