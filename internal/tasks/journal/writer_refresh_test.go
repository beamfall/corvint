package journal

import (
	"bytes"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// refreshStore is a settled journal with tickets, request afterimages and
// an evidence blob, so a writer checkpoint carries every aggregate.
func refreshStore(t *testing.T) (*fixture.Repo, Reader) {
	t.Helper()
	repo, r := setup(t)
	appendReceipt(t, repo, "MUTATION", map[string][]byte{ticketPath("A"): fixture.Ticket("A").Encode()}, "", true, true, true)
	for i := 0; i < 6; i++ {
		id := fmt.Sprintf("refresh-%d", i)
		path, _ := snapshot.RequestPath(id)
		appendReceipt(t, repo, "MUTATION", map[string][]byte{path: requestBytes(id, uint64(i+3), wire.Sum([]byte(id)), false)}, id, true, true, false)
	}
	return repo, r
}

// appendRequest commits one more settled request receipt.
func appendRequest(t *testing.T, repo *fixture.Repo, id string) {
	t.Helper()
	h, err := snapshot.DecodeHead(read(t, filepath.Join(repo.StateDir, "head.json")))
	if err != nil {
		t.Fatal(err)
	}
	path, _ := snapshot.RequestPath(id)
	appendReceipt(t, repo, "MUTATION", map[string][]byte{path: requestBytes(id, h.LastSeq.Uint64()+1, wire.Sum([]byte(id)), false)}, id, true, true, false)
}

// TestCALV0265_RefreshPrefixWalk covers CAL-V0-265 (proposed). On a journal
// that holds still the refresh is the complete audit and derives exactly
// its checkpoint, and the prefix walk derives the same bytes from the same
// journal. When a writer appends during every attempt, the complete
// attempt moves and the prefix walk still derives a checkpoint bound to the
// head it captured, which a writer audit then resumes from, walking the
// appended receipts as its tail.
func TestCALV0265_RefreshPrefixWalk(t *testing.T) {
	repo, r := refreshStore(t)
	full, observed, err := r.AuditForWriteObserved()
	if err != nil || full.Mode != ModeFull {
		t.Fatalf("complete audit: %v", err)
	}
	want, err := full.WriterCheckpoint(observed.Files)
	if err != nil {
		t.Fatal(err)
	}

	quiet, quietObs, err := r.AuditWriterRefresh()
	if err != nil || quiet.Mode != ModeFull {
		t.Fatalf("quiescent refresh: %+v %v", quiet, err)
	}
	if got, err := quiet.WriterCheckpoint(quietObs.Files); err != nil || !bytes.Equal(got.Encode(), want.Encode()) {
		t.Fatalf("quiescent refresh checkpoint differs: %v", err)
	}

	prefix := r
	obs := PhysicalObservation{}
	prefix.physical, prefix.prefix = &obs, true
	walked, err := prefix.auditAttempt(nil, "", profileLimits, true, nil)
	if err != nil || walked.Mode != ModePrefix || walked.StructuralConsistency != "CONSISTENT" {
		t.Fatalf("prefix walk: %+v %v", walked, err)
	}
	if got, err := walked.WriterCheckpoint(obs.Files); err != nil || !bytes.Equal(got.Encode(), want.Encode()) {
		t.Fatalf("prefix checkpoint differs from the complete audit's: %v", err)
	}

	appends := 0
	r.afterCapture = func() {
		appends++
		appendRequest(t, repo, fmt.Sprintf("racing-%d", appends))
	}
	moving, movingObs, err := r.AuditWriterRefresh()
	if err != nil || moving.Mode != ModePrefix {
		t.Fatalf("refresh under appends: %+v %v", moving, err)
	}
	if appends != 2 {
		t.Fatalf("captures %d, want one complete attempt and one prefix walk", appends)
	}
	wc, err := moving.WriterCheckpoint(movingObs.Files)
	if err != nil || wc.FullSeq != moving.LastSeq.Uint64() || wc.FullSeq != want.FullSeq+1 {
		t.Fatalf("prefix checkpoint %+v %v, want FullSeq %d", wc, err, want.FullSeq+1)
	}
	r.afterCapture = nil
	tail, _, err := r.AuditForWriter(wc, "", false)
	if err != nil || tail.Mode != ModeWriter || tail.LastSeq.Uint64() != wc.FullSeq+1 {
		t.Fatalf("writer audit resumed from the prefix checkpoint: %+v %v", tail, err)
	}
}

// TestCALV0265_RefreshRefusesOnlyHeldJournal covers CAL-V0-265 (proposed):
// a prefix fault on a journal that keeps moving is SNAPSHOT_MOVED, never a
// refusal, and the same fault on a journal that holds still is the complete
// audit's own refusal.
func TestCALV0265_RefreshRefusesOnlyHeldJournal(t *testing.T) {
	repo, r := refreshStore(t)
	path, _ := snapshot.RequestPath("refresh-2")
	fixture.Write(t, filepath.Join(repo.StateDir, path), []byte("{}\n"))
	_, wantErr := r.AuditForWrite()
	if wantErr == nil || wire.CodeOf(wantErr) == wire.CodeSnapshotMoved {
		t.Fatalf("complete audit over the fault: %v", wantErr)
	}
	appends := 0
	moving := r
	moving.afterCapture = func() {
		appends++
		appendRequest(t, repo, fmt.Sprintf("racing-%d", appends))
	}
	if _, _, err := moving.AuditWriterRefresh(); wire.CodeOf(err) != wire.CodeSnapshotMoved {
		t.Fatalf("refresh over the fault under appends: %v", err)
	}
	if _, _, err := r.AuditWriterRefresh(); wire.CodeOf(err) != wire.CodeOf(wantErr) {
		t.Fatalf("held refresh: %v, want %v", err, wantErr)
	}
}
