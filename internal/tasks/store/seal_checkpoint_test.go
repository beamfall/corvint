package store

import (
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
