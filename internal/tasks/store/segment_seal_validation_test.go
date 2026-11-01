package store

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// sealTriggerHead is the head a write observes when its own commit leaves
// 1,024 receipts after S and so seals the next segment (JSG-V0-008).
func sealTriggerHead(t *testing.T, repo *intent.Repository) uint64 {
	return segmentThrough(t, repo) + 2*snapshot.SegmentReceipts - 1
}

// padToSeal runs writes, each after padded receipts that keep the writer's
// tail between 64 and 200, until the head is sealTriggerHead: the next write
// seals. check runs after every write. The final tail is at least 99, so
// the sealing write's own advance would retain a checkpoint.
func padToSeal(t *testing.T, repo *intent.Repository, check func(writerRun)) {
	t.Helper()
	for {
		head := headSeqOf(t, repo)
		wc := readWriterCheckpoint(repo)
		if wc == nil {
			t.Fatal("no writer checkpoint while padding")
		}
		tail := head - wc.Seq.Uint64()
		target := sealTriggerHead(t, repo)
		room, left := 200-int(tail), int(target-head)
		if left <= room && left+int(tail) >= journal.WriterAdvanceTail {
			writerPad(t, repo, left)
			return
		}
		writerPad(t, repo, max(0, min(room, left-100)))
		check(writerMutate(t, repo, fmt.Sprintf("seal-pad-%d", headSeqOf(t, repo)), nil))
	}
}

// sealValidationStore is a store sealed through 1,024 whose writer
// checkpoint has FullSeq 1,100 from one complete-route write, with the
// head one write short of sealing receipts 1,025..1,536.
func sealValidationStore(t *testing.T) *intent.Repository {
	t.Helper()
	repo := historyStore(t, segmentHistory)
	if err := sealSegmentsLocked(context.Background(), repo); err != nil {
		t.Fatal(err)
	}
	lowerWriterCheckpoint(t, repo)
	// This fixture seals from a checkpoint inside the range, the state a
	// complete route left before JSG-V0-780 kept the sealed base.
	plainCheckpointStores.Store(repo.StateDir, true)
	if run := writerMutate(t, repo, "seal-seed", nil); !run.completed() || run.fast() {
		t.Fatalf("seed: %+v %v %v", run.rep, run.err, run.stages)
	}
	if wc := readWriterCheckpoint(repo); wc == nil || wc.FullSeq != segmentHistory {
		t.Fatalf("seed retained %+v, want FullSeq %d", wc, segmentHistory)
	}
	fastWrite := func(run writerRun) {
		if !run.completed() || !run.fast() {
			t.Fatalf("write: %+v %v %v", run.rep, run.err, run.stages)
		}
	}
	padToSeal(t, repo, fastWrite)
	// FullSeq lies past this seal's range, so it seals without validating.
	fastWrite(writerMutate(t, repo, "seal-plain", nil))
	if got := segmentThrough(t, repo); got != 2*snapshot.SegmentReceipts {
		t.Fatalf("sealed through %d, want %d", got, 2*snapshot.SegmentReceipts)
	}
	if wc := readWriterCheckpoint(repo); wc == nil || wc.FullSeq != segmentHistory {
		t.Fatalf("a plain seal left %+v, want FullSeq %d", wc, segmentHistory)
	}
	padToSeal(t, repo, fastWrite)
	return repo
}

// verifyAgainstCompleteAudit compares the retained writer checkpoint with a
// complete audit of the store, as the scheduled refresh does (JSG-V0-003).
func verifyAgainstCompleteAudit(t *testing.T, repo *intent.Repository) {
	t.Helper()
	head, err := readHead(repo)
	if err != nil {
		t.Fatal(err)
	}
	res, obs, err := journalReader(repo, head).AuditWriterRefresh()
	if err != nil {
		t.Fatal(err)
	}
	ok, err := res.VerifyWriterCheckpoint(readWriterCheckpoint(repo), obs.Files)
	if !ok || err != nil {
		t.Fatalf("complete audit verify: ok=%v %v", ok, err)
	}
}

// TestJSGV0009_SealValidatesAndAdvancesFullSeq covers JSG-V0-009 and
// JSG-V0-010: a write whose seal covers FullSeq validates the sealed
// receipts from the base and advances FullSeq to the seal's last receipt,
// with a rebased checkpoint a complete audit verifies. Every write takes the
// writer route, none schedules a refresh, and head − FullSeq stays at most
// 1,023 through two validated seals.
func TestJSGV0009_SealValidatesAndAdvancesFullSeq(t *testing.T) {
	t.Parallel()
	repo := sealValidationStore(t)
	check := func(run writerRun) {
		t.Helper()
		if !run.completed() || !run.fast() {
			t.Fatalf("write: %+v %v %v", run.rep, run.err, run.stages)
		}
		for _, s := range run.stages {
			if strings.HasPrefix(s, "refresh.") {
				t.Fatalf("a segmented store scheduled a refresh: %v", run.stages)
			}
		}
		wc := readWriterCheckpoint(repo)
		if wc == nil || headSeqOf(t, repo)-wc.FullSeq >= journal.WriterSegmentedBound {
			t.Fatalf("head %d, checkpoint %+v: head − FullSeq exceeds 1,023", headSeqOf(t, repo), wc)
		}
	}
	for _, through := range []uint64{3 * snapshot.SegmentReceipts, 4 * snapshot.SegmentReceipts} {
		before := readWriterCheckpoint(repo)
		check(writerMutate(t, repo, fmt.Sprintf("seal-validated-%d", through), nil))
		if got := segmentThrough(t, repo); got != through {
			t.Fatalf("sealed through %d, want %d", got, through)
		}
		wc := readWriterCheckpoint(repo)
		if wc == nil || wc.FullSeq != through || wc.Seq != before.Seq || wc.Requests() != before.Requests() {
			t.Fatalf("validated seal retained %+v, want FullSeq %d at seq %s", wc, through, before.Seq)
		}
		verifyAgainstCompleteAudit(t, repo)
		if through == 3*snapshot.SegmentReceipts {
			padToSeal(t, repo, check)
		}
	}
	mustSegmentAudit(t, repo, headSeqOf(t, repo))
}

// TestJSGV0009_SealDeclinesACarriedValueThatDiffers forges, under a valid
// trailer, the sequence of one carried ticket entry posted after the seal's
// range so that it claims the range's last receipt. The seal's validators
// derive that ticket's afterimage at that receipt, which differs, so the
// seal declines and invalidates the checkpoint; the write itself completes.
// The next write takes the complete route and seals plainly.
func TestJSGV0009_SealDeclinesACarriedValueThatDiffers(t *testing.T) {
	t.Parallel()
	repo := sealValidationStore(t)
	last := uint64(3 * snapshot.SegmentReceipts)
	forged := ""
	writerCheckpointRewrite(t, repo, func(wc *journal.WriterCheckpoint) {
		for i, e := range wc.Entries {
			if strings.HasPrefix(e.Path, "intent/tickets/") && e.Seq.Uint64() > last {
				wc.Entries[i].Seq = wire.SizeOf(last)
				forged = e.Path
				return
			}
		}
	})
	if forged == "" {
		t.Fatal("no ticket posted after the sealed range to forge")
	}
	token := writerInvalidation(repo)
	run := writerMutate(t, repo, "seal-forged", nil)
	if !run.completed() || !run.fast() {
		t.Fatalf("write: %+v %v %v", run.rep, run.err, run.stages)
	}
	if got := segmentThrough(t, repo); got != 2*snapshot.SegmentReceipts {
		t.Fatalf("a seal over a forged %s sealed through %d", forged, got)
	}
	for _, p := range []string{journal.WriterCheckpointPath(repo.StateDir), journal.WriterBasePath(repo.StateDir)} {
		if _, err := os.Lstat(p); !os.IsNotExist(err) {
			t.Fatalf("a declined seal left %s: %v", p, err)
		}
	}
	if writerInvalidation(repo) == token {
		t.Fatal("a declined seal kept the invalidation token")
	}
	next := writerMutate(t, repo, "seal-after-decline", nil)
	if !next.completed() || next.fast() {
		t.Fatalf("next write: %+v %v %v", next.rep, next.err, next.stages)
	}
	if got := segmentThrough(t, repo); got != last {
		t.Fatalf("the complete route sealed through %d, want %d", got, last)
	}
	mustSegmentAudit(t, repo, headSeqOf(t, repo))
}

// TestJSGV0009_SealWithoutCheckpointSealsPlainly removes the writer
// checkpoint before the sealing write: the write takes the complete route,
// retains FullSeq at its observed head and the seal does not validate.
func TestJSGV0009_SealWithoutCheckpointSealsPlainly(t *testing.T) {
	t.Parallel()
	repo := sealValidationStore(t)
	observed := headSeqOf(t, repo)
	os.Remove(journal.WriterCheckpointPath(repo.StateDir))
	os.Remove(journal.WriterBasePath(repo.StateDir))
	run := writerMutate(t, repo, "seal-no-checkpoint", nil)
	if !run.completed() || run.fast() {
		t.Fatalf("write: %+v %v %v", run.rep, run.err, run.stages)
	}
	if got := segmentThrough(t, repo); got != 3*snapshot.SegmentReceipts {
		t.Fatalf("sealed through %d, want %d", got, 3*snapshot.SegmentReceipts)
	}
	if wc := readWriterCheckpoint(repo); wc == nil || wc.FullSeq != observed {
		t.Fatalf("retained %+v, want FullSeq %d", wc, observed)
	}
	mustSegmentAudit(t, repo, observed+1)
}

// TestJSGV0010_SegmentedWriterBound checks the segmented bound: a writer
// whose head is 1,023 receipts past FullSeq takes the writer route, and one
// 1,024 past declines to the complete route.
func TestJSGV0010_SegmentedWriterBound(t *testing.T) {
	t.Parallel()
	repo := sealValidationStore(t)
	for _, tc := range []struct {
		past uint64
		fast bool
	}{{journal.WriterSegmentedBound - 1, true}, {journal.WriterSegmentedBound, false}} {
		head := headSeqOf(t, repo)
		writerCheckpointRewrite(t, repo, func(wc *journal.WriterCheckpoint) { wc.FullSeq = head - tc.past })
		run := writerMutate(t, repo, fmt.Sprintf("bound-%d", tc.past), nil)
		if !run.completed() || run.fast() != tc.fast {
			t.Fatalf("head − FullSeq %d: fast %v, want %v: %v %v", tc.past, run.fast(), tc.fast, run.err, run.stages)
		}
		if !tc.fast && !strings.Contains(run.declined(), "writer bound") {
			t.Fatalf("declined %q, want the writer bound", run.declined())
		}
	}
}
