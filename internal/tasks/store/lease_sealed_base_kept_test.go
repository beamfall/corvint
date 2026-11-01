//go:build darwin || linux

package store

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/journal"
)

// LGL-V0-891 (V1-2621): on a segmented store whose bound writer checkpoint
// holds the sealed base, a Mutate that takes the complete route while that
// checkpoint is still valid keeps FullSeq at the sealed sequence S, its
// complete audit re-based on the sealed base. The next pool release then
// resumes from the sealed base (Resumed 1, no complete audit) with the
// report and the store bytes the complete route gives on the same pre-state.
func TestLGLV0891_CompleteRouteWriteKeepsTheSealedBase(t *testing.T) {
	t.Parallel()
	if !journal.ResumableAudits() {
		t.Skip("platform reports no change time")
	}
	f := newSealedBaseFixture(t)
	through := segmentThrough(t, f.repo)
	acquired := sealedRun(t, f, f.acquire("kept-acquire"), nil)
	if !acquired.completed() || acquired.rep.PoolAllocation == nil || acquired.timing.Resumed != 1 {
		t.Fatalf("acquire: %+v %v resumed %d", acquired.rep, acquired.err, acquired.timing.Resumed)
	}
	if wc, _ := boundWriterCheckpoint(f.repo); wc == nil || wc.FullSeq != through {
		t.Fatalf("checkpoint before the complete-route write: %+v, want FullSeq %d", wc, through)
	}
	mutateWriterOff.Store(f.repo.StateDir, struct{}{})
	complete := writerMutate(t, f.repo, "kept-complete", nil)
	mutateWriterOff.Delete(f.repo.StateDir)
	if !complete.completed() || complete.fast() {
		t.Fatalf("complete-route mutate: %+v %v %v", complete.rep, complete.err, complete.stages)
	}
	// The complete-route write recorded its receipt at the wall clock.
	f.now = WallClock()
	wc, _ := boundWriterCheckpoint(f.repo)
	if wc == nil || wc.FullSeq != through {
		t.Fatalf("the complete-route write retained %+v, want FullSeq %d", wc, through)
	}
	parent := filepath.Dir(f.repo.PrimaryWorktree)
	pre := filepath.Join(t.TempDir(), "pre")
	copyTree(t, parent, pre)
	allocation := string(acquired.rep.PoolAllocation.AllocationID)
	released := sealedRun(t, f, f.release("kept-release", allocation), nil)
	if !released.completed() || released.timing.Resumed != 1 || released.audits != 0 || !released.fast() {
		t.Fatalf("release after the complete-route write: %+v %v resumed %d audits %d stages %v", released.rep, released.err, released.timing.Resumed, released.audits, released.stages)
	}
	wcPath := journal.WriterCheckpointPath(f.repo.StateDir)
	resumed := storeFiles(t, parent, wcPath)
	if err := os.RemoveAll(parent); err != nil {
		t.Fatal(err)
	}
	copyTree(t, pre, parent)
	for _, p := range []string{wcPath, wcPath + ".base"} {
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
	}
	oracle := sealedRun(t, f, f.release("kept-release", allocation), nil)
	if !oracle.completed() || oracle.fast() || oracle.audits == 0 {
		t.Fatalf("complete route: %+v %v audits %d", oracle.rep, oracle.err, oracle.audits)
	}
	completeFiles := storeFiles(t, parent, wcPath)
	for p, d := range completeFiles {
		if resumed[p] != d {
			t.Errorf("%s: resumed %q, complete %q", p, resumed[p], d)
		}
	}
	for p := range resumed {
		if _, ok := completeFiles[p]; !ok {
			t.Errorf("%s: only the resumed route wrote it", p)
		}
	}
	if !reflect.DeepEqual(*released.rep, *oracle.rep) {
		t.Fatalf("reports differ:\nresumed  %+v\ncomplete %+v", *released.rep, *oracle.rep)
	}
	mustSegmentAudit(t, f.repo, headSeqOf(t, f.repo))
}
