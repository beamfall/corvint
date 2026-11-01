package store

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/journal"
)

// TestCALV0060_AdministrativeWriterLeavesCheckpoints (V1-0673): a policy
// update through the administrative writer neither retains nor removes the
// read or writer checkpoint. A retained one stays a valid resume point whose
// tail replays the policy receipt; with none, reads stay complete until the
// next ordinary writer retains one.
func TestCALV0060_AdministrativeWriterLeavesCheckpoints(t *testing.T) {
	t.Parallel()
	read := func(t *testing.T, repo *intent.Repository, cp *journal.Checkpoint) string {
		t.Helper()
		head, err := writerGuards(repo, "MUTATE")
		if err != nil {
			t.Fatal(err)
		}
		reader := journalReader(repo, head)
		reader.Checkpoint = cp
		res, err := reader.Audit()
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		return res.Mode
	}
	t.Run("retained", func(t *testing.T) {
		t.Parallel()
		repo := writerStore(t, 70)
		cpRaw, err := os.ReadFile(journal.CheckpointPath(repo.StateDir))
		if err != nil {
			t.Fatal(err)
		}
		wcRaw, err := os.ReadFile(journal.WriterCheckpointPath(repo.StateDir))
		if err != nil {
			t.Fatal(err)
		}
		before := readCheckpointSeq(t, repo)
		holdLeasePolicy(t, repo)
		for path, want := range map[string][]byte{journal.CheckpointPath(repo.StateDir): cpRaw, journal.WriterCheckpointPath(repo.StateDir): wcRaw} {
			if got, err := os.ReadFile(path); err != nil || !bytes.Equal(got, want) {
				t.Fatalf("the policy update changed %s: %v", path, err)
			}
		}
		cp, err := journal.DecodeCheckpoint(cpRaw)
		if err != nil {
			t.Fatal(err)
		}
		if before >= headSeqOf(t, repo) {
			t.Fatalf("checkpoint %d is not behind the policy receipt at %d", before, headSeqOf(t, repo))
		}
		if mode := read(t, repo, cp); mode != journal.ModeCheckpoint {
			t.Fatalf("read after the policy update: %s, want %s", mode, journal.ModeCheckpoint)
		}
		if run := writerMutate(t, repo, "after-policy", nil); !run.completed() || !run.fast() {
			t.Fatalf("write after the policy update: %+v %v %v", run.rep, run.err, run.stages)
		}
		if after := readCheckpointSeq(t, repo); after <= before {
			t.Fatalf("the next ordinary writer left the checkpoint at %d", after)
		}
	})
	t.Run("absent", func(t *testing.T) {
		t.Parallel()
		repo := writerStore(t, 70)
		if err := os.Remove(journal.CheckpointPath(repo.StateDir)); err != nil {
			t.Fatal(err)
		}
		holdLeasePolicy(t, repo)
		if _, err := os.Stat(journal.CheckpointPath(repo.StateDir)); !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("the policy update retained a read checkpoint: %v", err)
		}
		if mode := read(t, repo, nil); mode != journal.ModeFull {
			t.Fatalf("read without a checkpoint: %s, want %s", mode, journal.ModeFull)
		}
		if run := writerMutate(t, repo, "after-policy", nil); !run.completed() {
			t.Fatalf("write after the policy update: %+v %v %v", run.rep, run.err, run.stages)
		}
		if readCheckpointSeq(t, repo) != headSeqOf(t, repo)-1 {
			t.Fatalf("the next ordinary writer retained %d, want the head it audited (%d)", readCheckpointSeq(t, repo), headSeqOf(t, repo)-1)
		}
	})
}
