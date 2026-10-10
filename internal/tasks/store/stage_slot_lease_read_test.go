package store_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// CTS-V0-008 (V1-0467, V1-0949): the unlocked lease read behind
// store.AttemptRecord and GateRun's pre-lock attempt read (readLeaseProof)
// is an unlocked journal audit, so a descriptor-less staging slot is waited
// for before it is refused. Both tickets' failures were this reader refusing
// a live writer's slot at once. A slot that never goes away is still refused
// MALFORMED, but only after the full two-second budget; the bound below is
// one-sided because every pause sleeps at least its requested length.
func TestCTSV0008_UnlockedLeaseReadWaitsForStageSlot(t *testing.T) {
	t.Parallel()
	s := newGateStore(t)
	a := s.claim(t, "claim-a", s.ticket(t, "a"), 1, "src/")
	fixture.Write(t, filepath.Join(s.repo.StateDir, "staging", "a00"), []byte("x"))
	started := time.Now()
	_, err := store.AttemptRecord(context.Background(), s.repo, a.AttemptID)
	elapsed := time.Since(started)
	if e, ok := err.(*wire.Error); !ok || e.Code != wire.CodeMalformed || e.Where != "staging/a00" {
		t.Fatalf("lease read beside a held slot: %v", err)
	}
	if elapsed < snapshot.StageSlotPatience {
		t.Fatalf("lease read refused a descriptor-less slot after %v, before the %v stage-slot wait", elapsed, snapshot.StageSlotPatience)
	}
}
