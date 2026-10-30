package store_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/authority"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// CTS-V0-008 (V1-0467, V1-0949): the unlocked lease read behind
// store.AttemptRecord and GateRun's pre-lock attempt read (readLeaseProof)
// is an unlocked journal audit, so a descriptor-less staging slot is waited
// for before it is refused. Both tickets' failures were this reader refusing
// a live writer's slot at once. The pause sequence itself is pinned without
// timing by the journal package's stage-sleep seam; these tests pin the
// store's lease read to that audit, which the seam cannot reach from here.
//
// A slot that goes away inside the budget, as a live writer's does, is
// waited out and the read succeeds. The slot is removed only after half the
// budget, so a read that refuses at once fails here.
func TestCTSV0008_UnlockedLeaseReadWaitsOutWriterHeldSlot(t *testing.T) {
	t.Parallel()
	s := newGateStore(t)
	a := s.claim(t, "claim-a", s.ticket(t, "a"), 1, "src/")
	slot := filepath.Join(s.repo.StateDir, "staging", "a00")
	fixture.Write(t, slot, []byte("x"))
	removed := make(chan error, 1)
	timer := time.AfterFunc(snapshot.StageSlotPatience/2, func() { removed <- os.Remove(slot) })
	defer timer.Stop()
	rec, err := store.AttemptRecord(context.Background(), s.repo, a.AttemptID)
	if err != nil {
		t.Fatalf("lease read beside a writer-held slot: %v", err)
	}
	if rec.AttemptID != a.AttemptID {
		t.Fatalf("attempt %q; want %q", rec.AttemptID, a.AttemptID)
	}
	if e := <-removed; e != nil {
		t.Fatal(e)
	}
}

// A slot that outlasts the budget while the writer lock stays held is
// still refused MALFORMED at its path, but only after the full budget; the
// bound is one-sided because every pause sleeps at least its requested
// length. A killed writer's orphan beside a free lock reads instead
// (CTS-V0-026, TestCTSV0026_KilledGateRunLeavesAReadableStore).
func TestCTSV0008_UnlockedLeaseReadWaitsForStageSlot(t *testing.T) {
	t.Parallel()
	s := newGateStore(t)
	a := s.claim(t, "claim-a", s.ticket(t, "a"), 1, "src/")
	fixture.Write(t, filepath.Join(s.repo.StateDir, "staging", "a00"), []byte("x"))
	held, err := authority.AcquireLock(context.Background(), s.repo, authority.LockOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer held.Close()
	started := time.Now()
	_, err = store.AttemptRecord(context.Background(), s.repo, a.AttemptID)
	elapsed := time.Since(started)
	if e, ok := err.(*wire.Error); !ok || e.Code != wire.CodeMalformed || e.Where != "staging/a00" {
		t.Fatalf("lease read beside a held slot: %v", err)
	}
	if elapsed < snapshot.StageSlotPatience {
		t.Fatalf("lease read refused a descriptor-less slot after %v, before the %v stage-slot wait", elapsed, snapshot.StageSlotPatience)
	}
}
