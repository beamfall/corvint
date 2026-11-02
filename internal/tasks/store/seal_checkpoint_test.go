package store

import (
	"bytes"
	"context"
	"os"
	"reflect"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/journal"
)

// LGL-V0-809: `journal seal` and `journal unseal` each leave a read
// checkpoint at the head from one complete settled audit of the migrated
// journal, and a read resumed from it answers as the complete audit does.
func TestLGLV0809_SealAndUnsealLeaveAReadCheckpoint(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := historyStore(t, segmentHistory)
	check := func(label string) {
		t.Helper()
		raw, err := os.ReadFile(journal.CheckpointPath(repo.StateDir))
		if err != nil {
			t.Fatalf("%s: no read checkpoint retained: %v", label, err)
		}
		cp, err := journal.DecodeCheckpoint(raw)
		if err != nil {
			t.Fatal(err)
		}
		head, err := readHead(repo)
		if err != nil {
			t.Fatal(err)
		}
		r := journalReader(repo, head)
		r.SelectState = true
		complete, err := r.Audit()
		if err != nil || complete.Mode != journal.ModeFull || cp.Seq != head.LastSeq {
			t.Fatalf("%s: complete %v, checkpoint %s, head %s", label, err, cp.Seq, head.LastSeq)
		}
		r.Checkpoint = cp
		resumed, err := r.Audit()
		if err != nil || resumed.Mode != journal.ModeCheckpoint || resumed.LastSeq != complete.LastSeq || !reflect.DeepEqual(resumed.Records, complete.Records) {
			t.Fatalf("%s: resumed read %v %v", label, resumed.Mode, err)
		}
	}
	os.Remove(journal.CheckpointPath(repo.StateDir))
	if _, err := SealJournal(ctx, repo); err != nil {
		t.Fatal(err)
	}
	if segmentThrough(t, repo) == 0 {
		t.Fatal("nothing sealed")
	}
	check("seal")
	if _, err := UnsealJournal(ctx, repo); err != nil {
		t.Fatal(err)
	}
	check("unseal")
}

// LGL-V0-815: the read checkpoint a seal or unseal retains from its opening
// complete audit is byte-identical to the one a complete audit of the
// migrated journal derives.
func TestLGLV0815_MigrationReusesTheOpeningCheckpoint(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := historyStore(t, segmentHistory)
	fresh := func(label string) {
		t.Helper()
		retained, err := os.ReadFile(journal.CheckpointPath(repo.StateDir))
		if err != nil {
			t.Fatalf("%s: no checkpoint retained: %v", label, err)
		}
		head, err := readHead(repo)
		if err != nil {
			t.Fatal(err)
		}
		proof, err := lockedJournalReader(repo, head).Audit()
		if err != nil {
			t.Fatal(err)
		}
		cp := proof.Checkpoint()
		if cp == nil {
			t.Fatalf("%s: the migrated journal derives no checkpoint", label)
		}
		if !bytes.Equal(retained, cp.Encode()) {
			t.Fatalf("%s: the reused opening checkpoint differs from the migrated journal's", label)
		}
	}
	if _, err := SealJournal(ctx, repo); err != nil {
		t.Fatal(err)
	}
	fresh("seal")
	if _, err := UnsealJournal(ctx, repo); err != nil {
		t.Fatal(err)
	}
	fresh("unseal")
}
