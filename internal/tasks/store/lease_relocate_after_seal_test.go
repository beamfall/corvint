//go:build darwin || linux

package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
)

// JSG-V0-780, V1-3665: the primary-worktree guard. A RELOCATE after the
// sealed sequence S leaves a head whose primary worktree differs from the
// chain's at S. A complete-route write then keeps the base-free checkpoint at
// head: a base derived at S would name the new primary, which the chain did
// not hold at S, and the next seal or resume would refuse the RELOCATE it
// re-steps. Without the guard the retained checkpoint has FullSeq at S.
func TestJSGV0780_RelocateAfterSealKeepsPlainCheckpoint(t *testing.T) {
	t.Parallel()
	if !journal.ResumableAudits() {
		t.Skip("platform reports no change time")
	}
	c := sealedWidenReapCases[1] // reap: the fixture's clock is past the claim's lease
	f := c.fixture(t)
	// Relocation refuses while an attempt holds a reservation, so reap it.
	if run := sealedRun(t, f, f.lease("relocate-reap", c.lease(f)), nil); !run.completed() {
		t.Fatalf("reap: %+v %v %v", run.rep, run.err, run.stages)
	}
	through := segmentThrough(t, f.repo)
	// Move the repository and resolve it at its new path, as an operator
	// relocating a store does (TRL-V0-001).
	from := f.repo.PrimaryWorktree
	to := filepath.Join(filepath.Dir(from), "moved")
	if err := os.Rename(from, to); err != nil {
		t.Fatal(err)
	}
	resolved, err := intent.Resolve(to)
	if err != nil {
		t.Fatal(err)
	}
	moved := *resolved
	lowerWriterCheckpoint(t, &moved)
	rep, err := Relocate(context.Background(), &moved, historyActor, RelocateRequest{QueueID: fixture.QueueID, RequestID: "relocate-after-seal", From: from}, f.now)
	if err != nil || rep.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("relocate: %+v %v", rep, err)
	}
	plainCheckpointStores.Delete(moved.StateDir)
	wcPath := journal.WriterCheckpointPath(moved.StateDir)
	for _, p := range []string{wcPath, wcPath + ".base"} {
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
	}
	write := stagedWrite(writerCreate(&moved, "after-relocate", f.now), nil) // the fixture clock is past wall time
	if !write.completed() || write.fast() {
		t.Fatalf("write after the relocate did not take the complete route: %+v %v %v", write.rep, write.err, write.stages)
	}
	// The complete route retains the checkpoint its audit derived, at the
	// RELOCATE receipt before this write; with the guard it is base-free
	// (FullSeq equals Seq), without it FullSeq is the sealed sequence.
	head := headSeqOf(t, &moved)
	wc := readWriterCheckpoint(&moved)
	if wc == nil || wc.Seq.Uint64() != head-1 || wc.FullSeq != wc.Seq.Uint64() || wc.FullSeq <= through {
		t.Fatalf("complete route after a RELOCATE past S=%d retained %+v, want the base-free checkpoint at the RELOCATE, %d", through, wc, head-1)
	}
	mustSegmentAudit(t, &moved, head)
}
