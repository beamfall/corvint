//go:build darwin || linux

package store_test

import (
	"context"
	"os"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestCALV0078_SupervisedGateRecordFailureIsNotRetryable: a supervised
// reviewer run whose required gate ran but could not be recorded returns the
// LOCK_TIMEOUT marked not retryable, because a repeated `run --role reviewer`
// skips the CHECKING attempt instead of finishing its gates (CAL-V0-078).
func TestCALV0078_SupervisedGateRecordFailureIsNotRetryable(t *testing.T) {
	f := buildProgramFixture(t, true, false)
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	w, err := store.OpenWorkflow(ctx, f.s.repo, operator(), "program", self, f.config, f.ticketID)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if a, err := w.RunRole(ctx, "implementer", ""); err != nil || a.Phase != "BUILT" {
		t.Fatalf("implement: %+v %v", a, err)
	}
	ran := 0
	restore := store.SetGateRecordFaultForTest(func() error {
		ran++
		return wire.Errorf(wire.CodeLockTimeout, "lock", "injected contention after the gate ran")
	})
	a, err := w.RunRole(ctx, "reviewer", "")
	restore()
	if ran != 1 {
		t.Fatalf("gate ran %d times before recording", ran)
	}
	if wire.CodeOf(err) != wire.CodeLockTimeout || !wire.RetryForbidden(err) {
		t.Fatalf("unrecorded gate reported retryable: %v (forbidden=%v)", err, wire.RetryForbidden(err))
	}
	if a == nil || a.Phase != "CHECKING" || len(a.GateResults) != 0 {
		t.Fatalf("attempt after the unrecorded gate: %+v", a)
	}
}
