//go:build darwin || linux

package store_test

import (
	"context"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/store"
)

// TestATRV0240_PhaseWriteKeepsControlRecordedMeanwhile records a DRAIN
// control after the workflow composed its next phase record and before the
// native transition. The phase write keeps the recorded control instead of
// overwriting it with the control its earlier read saw (V1-0831).
func TestATRV0240_PhaseWriteKeepsControlRecordedMeanwhile(t *testing.T) {
	f := newContinuationFixture(t, continuationOptions{})
	w := f.open(t)
	if p := f.program(t); p.Phase != "ADMITTED" || p.Control != "" {
		t.Fatalf("opened program phase %s control %q", p.Phase, p.Control)
	}
	recorded := false
	defer store.SetProgramWriteHookForTest(func() {
		if recorded {
			return
		}
		recorded = true
		p := f.program(t)
		p.Control = "DRAIN"
		r, e := store.ProgramTransition(context.Background(), f.s.repo, operator(), fixture.QueueID, "program-control-meanwhile", p)
		if e != nil || r.Outcome.Outcome != mutation.OutcomeCompleted {
			t.Fatalf("control write %+v %v", r, e)
		}
	})()
	if err := w.PersistForTest("WORKTREE_ADD"); err != nil {
		t.Fatalf("phase write: %v", err)
	}
	if !recorded {
		t.Fatal("hook did not run")
	}
	if p := f.program(t); p.Phase != "WORKTREE_ADD" || p.Control != "DRAIN" {
		t.Fatalf("program phase %s control %q, want WORKTREE_ADD with the recorded DRAIN kept", p.Phase, p.Control)
	}
}
