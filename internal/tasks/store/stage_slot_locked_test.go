package store_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// CTS-V0-008: reconcile holds the writer lock and does not clear orphan
// slots before its request lookup, so a killed writer's slot reaches its
// journal audit. Under the lock no writer can be publishing, so the audit
// refuses the slot at once. Without the writer-locked reader it pauses for
// exactly the two-second budget while every other writer waits on the lock;
// the bound below is one-sided against that.
func TestCTSV0008_LockedReconcileRefusesOrphanSlotWithoutWaiting(t *testing.T) {
	repo, request, _, _ := reconcileFixture(t)
	fixture.Write(t, filepath.Join(repo.StateDir, "staging", "a00"), []byte("x"))
	started := time.Now()
	_, err := store.Reconcile(context.Background(), repo, operator(), request, now(t))
	elapsed := time.Since(started)
	if e, ok := err.(*wire.Error); !ok || e.Code != wire.CodeMalformed || e.Where != "staging/a00" {
		t.Fatalf("reconcile beside an orphan slot: %v", err)
	}
	if elapsed >= 2*time.Second {
		t.Fatalf("reconcile held the writer lock %v waiting for an orphan slot", elapsed)
	}
}
