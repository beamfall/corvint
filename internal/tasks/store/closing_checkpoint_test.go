package store_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/store"
)

// closingRead checks, after an administrative writer, that the store holds a
// read checkpoint at its head and that a read resumed from it answers as the
// complete audit does (LGL-V0-809). It returns the checkpoint.
func closingRead(t *testing.T, s *leaseStore, label string) *journal.Checkpoint {
	t.Helper()
	raw, err := os.ReadFile(journal.CheckpointPath(s.repo.StateDir))
	if err != nil {
		t.Fatalf("%s: no read checkpoint retained: %v", label, err)
	}
	cp, err := journal.DecodeCheckpoint(raw)
	if err != nil {
		t.Fatalf("%s: %v", label, err)
	}
	r := retireReader(t, s)
	r.SelectState = true
	complete, err := r.Audit()
	if err != nil || complete.Mode != journal.ModeFull {
		t.Fatalf("%s: complete read %v", label, err)
	}
	if cp.Seq != complete.LastSeq || cp.ReceiptSha256 != complete.LastReceiptSha256 {
		t.Fatalf("%s: checkpoint at %s, head %s", label, cp.Seq, complete.LastSeq)
	}
	r.Checkpoint = cp
	resumed, err := r.Audit()
	if err != nil || resumed.Mode != journal.ModeCheckpoint {
		t.Fatalf("%s: resumed read %v %v", label, resumed.Mode, err)
	}
	if resumed.LastSeq != complete.LastSeq || resumed.Identity.HeadSha256 != complete.Identity.HeadSha256 || resumed.ProjectionAgreement != complete.ProjectionAgreement || !reflect.DeepEqual(resumed.Records, complete.Records) {
		t.Fatalf("%s: resumed read differs from the complete audit", label)
	}
	return cp
}

// tamperedHeadRefused flips the head receipt and requires the read resumed
// from the retained checkpoint to refuse exactly as the complete audit does.
func tamperedHeadRefused(t *testing.T, s *leaseStore, cp *journal.Checkpoint, label string) {
	t.Helper()
	name, _ := snapshot.ReceiptName(cp.Seq.Uint64())
	path := filepath.Join(s.repo.StateDir, "receipts", name)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	flipped := bytes.Replace(raw, []byte(`"recordedAt":"`), []byte(`"recordedAt":"1`), 1)
	if bytes.Equal(raw, flipped) {
		t.Fatalf("%s: no recordedAt in the head receipt", label)
	}
	if err := os.WriteFile(path, flipped, 0o600); err != nil {
		t.Fatal(err)
	}
	r := retireReader(t, s)
	r.SelectState = true
	_, e1 := r.Audit()
	r.Checkpoint = cp
	_, e2 := r.Audit()
	if e1 == nil || fmt.Sprint(e1) != fmt.Sprint(e2) {
		t.Fatalf("%s: tampered head: complete %v, resumed %v", label, e1, e2)
	}
}

// LGL-V0-809: compaction leaves a read checkpoint from its closing complete
// audit (and from its opening one when it elides nothing), and a tampered
// receipt is still refused.
func TestLGLV0809_CompactionLeavesAReadCheckpoint(t *testing.T) {
	t.Parallel()
	s, _ := retirable(t, 12)
	if r := compact(t, s, false); r.Elided == 0 {
		t.Fatalf("nothing elided: %+v", r)
	}
	closingRead(t, s, "compaction")
	again := compact(t, s, false)
	if again.Elided != 0 {
		t.Fatalf("second compaction elided %d", again.Elided)
	}
	cp := closingRead(t, s, "compaction with nothing to elide")
	if dry := compact(t, s, true); dry.Elided != 0 {
		t.Fatalf("dry run: %+v", dry)
	}
	tamperedHeadRefused(t, s, cp, "compaction")
}

// LGL-V0-809: a retire leaves no read checkpoint, because a store with a
// retired prefix supports none (reads walk from the anchor); the unretire
// that restores the prefix leaves one.
func TestLGLV0809_RetireLeavesNoneUnretireLeavesOne(t *testing.T) {
	t.Parallel()
	s, _ := retirable(t, 3)
	path := retireArchive(t)
	if _, err := store.RetirePrefix(context.Background(), s.repo, path, false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(journal.CheckpointPath(s.repo.StateDir)); !os.IsNotExist(err) {
		t.Fatalf("a retire left a read checkpoint: %v", err)
	}
	if _, err := store.UnretirePrefix(context.Background(), s.repo, path); err != nil {
		t.Fatal(err)
	}
	cp := closingRead(t, s, "unretire")
	tamperedHeadRefused(t, s, cp, "unretire")
}
