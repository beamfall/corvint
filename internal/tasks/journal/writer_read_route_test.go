package journal

import (
	"bytes"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// writerReadStore is a history of n ticket receipts with a writer
// checkpoint derived at its head, then tail more ticket receipts; it returns
// the read Reader (SelectState, as a CLI read) and the checkpoint.
func writerReadStore(t *testing.T, n, tail int) (*fixture.Repo, Reader, *WriterCheckpoint, *Checkpoint) {
	t.Helper()
	repo, r := prefetchHistory(t, n)
	full, observed, err := r.AuditForWriteObserved()
	if err != nil || full.Mode != ModeFull {
		t.Fatalf("complete audit: %v", err)
	}
	wc, err := full.WriterCheckpoint(observed.Files)
	if err != nil || wc == nil {
		t.Fatalf("writer checkpoint: %v", err)
	}
	read := r
	read.SelectState = true
	plain, err := read.Audit()
	if err != nil {
		t.Fatal(err)
	}
	cp := plain.Checkpoint()
	for i := 0; i < tail; i++ {
		tk := fixture.Ticket("A")
		body := fmt.Sprintf("tail %d", i)
		tk.Body = &body
		appendReceipt(t, repo, "MUTATION", map[string][]byte{"intent/tickets/A.json": tk.Encode()}, "", true, true, false)
	}
	return repo, read, wc, cp
}

func withWriter(r Reader, wc *WriterCheckpoint, calls *int) Reader {
	r.WriterCheckpoint = func() *WriterCheckpoint {
		*calls++
		return wc
	}
	return r
}

// sameReadAudit compares what a read binds: the chain position, the head and
// intent tree identity (the inventory digest is the capture's own and
// differs between a complete and a resumed capture for the read checkpoint
// too), the agreement and every retained record.
func sameReadAudit(t *testing.T, name string, a, b *Result) {
	t.Helper()
	if a.LastSeq != b.LastSeq || a.LastReceiptSha256 != b.LastReceiptSha256 || a.Identity.HeadSha256 != b.Identity.HeadSha256 || a.Identity.IntentTreeSha256 != b.Identity.IntentTreeSha256 || a.ProjectionAgreement != b.ProjectionAgreement {
		t.Fatalf("%s: verdicts differ: seq %s/%s last %s/%s identity %+v / %+v agreement %s/%s", name, a.LastSeq, b.LastSeq, a.LastReceiptSha256, b.LastReceiptSha256, a.Identity, b.Identity, a.ProjectionAgreement, b.ProjectionAgreement)
	}
	for p, ra := range a.Records {
		if rb, ok := b.Records[p]; !ok || !reflect.DeepEqual(ra, rb) {
			t.Fatalf("%s: record %s differs: %s %v / %s %v (present %v)", name, p, a.Mode, ra.Seq, b.Mode, rb.Seq, ok)
		}
	}
	for p := range b.Records {
		if _, ok := a.Records[p]; !ok {
			t.Fatalf("%s: %s retained only by %s", name, p, b.Mode)
		}
	}
}

// LGL-V0-807: a read with no read checkpoint resumes from the bound writer
// checkpoint, returns what the complete audit returns for every selected and
// state record, and reads only the checkpoint's receipt and the tail,
// however long the history.
func TestLGLV0807_ReadResumesFromWriterCheckpoint(t *testing.T) {
	paths := []string{"intent/tickets/A.json", "intent/tickets/B.json", "intent/queue.json"}
	var reads [2]int
	for i, n := range []int{40, 400} {
		_, r, wc, _ := writerReadStore(t, n, 5)
		complete, err := r.Audit(paths...)
		if err != nil || complete.Mode != ModeFull {
			t.Fatalf("n=%d complete: %v", n, err)
		}
		calls := 0
		old := afterNativeRead
		afterNativeRead = func(p string) {
			if strings.HasPrefix(p, "receipts/") {
				reads[i]++
			}
		}
		got, err := withWriter(r, wc, &calls).Audit(paths...)
		afterNativeRead = old
		if err != nil || got.Mode != ModeWriter || got.StructuralConsistency != ModeWriter || got.ProjectionAgreement != "AGREES" {
			t.Fatalf("n=%d writer read: %v %+v", n, err, got)
		}
		if calls != 1 {
			t.Fatalf("n=%d: loader called %d times", n, calls)
		}
		sameReadAudit(t, fmt.Sprintf("n=%d", n), complete, got)
	}
	if reads[0] != 6 || reads[1] != 6 {
		t.Fatalf("receipt reads %v, want the checkpoint receipt and the 5-receipt tail at both history sizes", reads)
	}
}

// LGL-V0-807: a read checkpoint that serves wins, and the writer loader is
// never called.
func TestLGLV0807_ReadCheckpointWinsAndLoaderIsLazy(t *testing.T) {
	_, r, wc, cp := writerReadStore(t, 40, 3)
	if cp == nil {
		t.Fatal("no read checkpoint derived")
	}
	calls := 0
	rr := withWriter(r, wc, &calls)
	rr.Checkpoint = cp
	got, err := rr.Audit("intent/tickets/A.json")
	if err != nil || got.Mode != ModeCheckpoint || calls != 0 {
		t.Fatalf("mode %s, loader calls %d, err %v", got.Mode, calls, err)
	}
	none := withWriter(r, nil, &calls)
	if got, err := none.Audit("intent/tickets/A.json"); err != nil || got.Mode != ModeFull {
		t.Fatalf("nil writer checkpoint: %v %v", got.Mode, err)
	}
}

// LGL-V0-807: tampering the route would trust is refused exactly as the
// complete audit refuses it -- a tail receipt, the receipt the checkpoint
// names, or a projection -- and a forged or foreign checkpoint only falls
// back to the complete audit.
func TestLGLV0807_TamperedReceiptsStillRefused(t *testing.T) {
	receipt := func(repo *fixture.Repo, seq uint64) string {
		name, _ := snapshot.ReceiptName(seq)
		return filepath.Join(repo.StateDir, "receipts", name)
	}
	edit := func(t *testing.T, path, from, to string) {
		t.Helper()
		raw := read(t, path)
		next := bytes.Replace(raw, []byte(from), []byte(to), 1)
		if bytes.Equal(raw, next) {
			t.Fatalf("%s: %q not found", path, from)
		}
		fixture.Write(t, path, next)
	}
	both := func(t *testing.T, name string, r Reader, wc *WriterCheckpoint) {
		t.Helper()
		complete, e1 := r.Audit("intent/tickets/A.json")
		calls := 0
		got, e2 := withWriter(r, wc, &calls).Audit("intent/tickets/A.json")
		if fmt.Sprint(e1) != fmt.Sprint(e2) {
			t.Fatalf("%s: complete %v, with writer checkpoint %v", name, e1, e2)
		}
		if e1 == nil {
			if got.Mode != ModeFull {
				t.Fatalf("%s: served from the writer checkpoint (%s) though tampered", name, got.Mode)
			}
			sameReadAudit(t, name, complete, got)
		}
	}
	t.Run("tail receipt", func(t *testing.T) {
		repo, r, wc, _ := writerReadStore(t, 40, 3)
		edit(t, receipt(repo, wc.Seq.Uint64()+2), `"tail 1"`, `"tail 9"`)
		both(t, "tail receipt", r, wc)
		if _, err := withWriter(r, wc, new(int)).Audit("intent/tickets/A.json"); err == nil {
			t.Fatal("a tampered tail receipt was accepted")
		}
	})
	t.Run("checkpoint receipt", func(t *testing.T) {
		repo, r, wc, _ := writerReadStore(t, 40, 3)
		edit(t, receipt(repo, wc.Seq.Uint64()), `"receipt 39"`, `"receipt 93"`)
		both(t, "checkpoint receipt", r, wc)
		if _, err := withWriter(r, wc, new(int)).Audit("intent/tickets/A.json"); err == nil {
			t.Fatal("a tampered checkpoint receipt was accepted")
		}
	})
	t.Run("projection", func(t *testing.T) {
		repo, r, wc, _ := writerReadStore(t, 40, 3)
		fixture.Write(t, filepath.Join(repo.IntentDir, "tickets", "B.json"), fixture.Ticket("B").Encode())
		both(t, "projection", r, wc)
	})
	t.Run("forged checkpoint", func(t *testing.T) {
		_, r, wc, _ := writerReadStore(t, 40, 3)
		forged := *wc
		forged.ReceiptSha256 = wire.Sum([]byte("forged"))
		both(t, "forged checkpoint", r, &forged)
		foreign := *wc
		foreign.QueueID, _ = wire.ParseQueueID("", "queue:other:main")
		both(t, "foreign checkpoint", r, &foreign)
	})
}

// LGL-V0-807: the boundary is the read checkpoint's. A receipt before the
// checkpoint is trusted as the writer audit that derived the checkpoint saw
// it, exactly as a read resumed from the read checkpoint trusts it
// (CAL-V0-059); the explicit complete audit (`receipt audit`, no checkpoint)
// still refuses it.
func TestLGLV0807_PrefixTamperIsTheReadCheckpointBoundary(t *testing.T) {
	repo, r, wc, cp := writerReadStore(t, 40, 3)
	name, _ := snapshot.ReceiptName(10)
	path := filepath.Join(repo.StateDir, "receipts", name)
	raw := read(t, path)
	tampered := bytes.Replace(raw, []byte(`"receipt 8"`), []byte(`"receipt 0"`), 1)
	if bytes.Equal(raw, tampered) {
		t.Fatal("fixture body not found")
	}
	fixture.Write(t, path, tampered)
	if _, err := r.Audit("intent/tickets/A.json"); err == nil {
		t.Fatal("the complete audit accepted a tampered prefix receipt")
	}
	viaRead := r
	viaRead.Checkpoint = cp
	fromRead, e1 := viaRead.Audit("intent/tickets/A.json")
	fromWriter, e2 := withWriter(r, wc, new(int)).Audit("intent/tickets/A.json")
	if e1 != nil || e2 != nil || fromRead.Mode != ModeCheckpoint || fromWriter.Mode != ModeWriter {
		t.Fatalf("resumed reads: read checkpoint %v %v, writer checkpoint %v %v", fromRead.Mode, e1, fromWriter.Mode, e2)
	}
	if fromRead.LastReceiptSha256 != fromWriter.LastReceiptSha256 || !reflect.DeepEqual(fromRead.Records["intent/tickets/A.json"], fromWriter.Records["intent/tickets/A.json"]) {
		t.Fatal("the two resumed reads disagree")
	}

}
