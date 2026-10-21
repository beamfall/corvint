package journal

import (
	"bytes"
	"fmt"
	"os"
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
	if got, err := quiet.WriterCheckpoint(quietObs.Files); err != nil || !bytes.Equal(encodeWriter(t, got), encodeWriter(t, want)) {
		t.Fatalf("quiescent refresh checkpoint differs: %v", err)
	}

	prefix := r
	obs := PhysicalObservation{}
	prefix.physical, prefix.prefix = &obs, true
	walked, err := prefix.auditAttempt(nil, "", profileLimits, true, nil)
	if err != nil || walked.Mode != ModePrefix || walked.StructuralConsistency != "CONSISTENT" {
		t.Fatalf("prefix walk: %+v %v", walked, err)
	}
	if got, err := walked.WriterCheckpoint(obs.Files); err != nil || !bytes.Equal(encodeWriter(t, got), encodeWriter(t, want)) {
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

// TestCALV0265_PrefixRecheckAfterWalk covers CAL-V0-265 (proposed): a
// prefix result never yields a read checkpoint, and a walked receipt or a
// compared request afterimage that changes between the walk and the
// recheck fails the walk SNAPSHOT_MOVED instead of binding a checkpoint.
func TestCALV0265_PrefixRecheckAfterWalk(t *testing.T) {
	request, _ := snapshot.RequestPath("refresh-2")
	second, _ := snapshot.ReceiptName(2)
	for _, tc := range []struct {
		name string
		path string
	}{
		{"none", ""},
		{"rewritten-request", request},
		{"rewritten-receipt", "receipts/" + second},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo, prefix := refreshStore(t)
			obs := PhysicalObservation{}
			prefix.physical, prefix.prefix = &obs, true
			if tc.path != "" {
				p := filepath.Join(repo.StateDir, tc.path)
				prefix.afterPrefixWalk = func() { fixture.Write(t, p, append(read(t, p), '\n')) }
			}
			walked, err := prefix.auditAttempt(nil, "", profileLimits, true, nil)
			if tc.path == "" {
				if err != nil || walked.Mode != ModePrefix || walked.Checkpoint() != nil {
					t.Fatalf("prefix walk: %v, read checkpoint %v", err, walked.Checkpoint() != nil)
				}
				return
			}
			if wire.CodeOf(err) != wire.CodeSnapshotMoved {
				t.Fatalf("change after the walk: %+v %v", walked, err)
			}
		})
	}
}

// TestCALV0265_RefreshUnderAppendsNeverAdvancesOverUnsettledHistory covers
// decision 0549's fail-closed refresh, which a liveness write's probe
// (CAL-V0-224) relies on to classify history it does not list. Under
// appends the refresh's complete attempt moves and the prefix walk runs; a
// stray temporary receipt name, or a receipt missing from history more than
// MaxWriterTail receipts before head, then yields SNAPSHOT_MOVED and no
// checkpoint, so FullSeq never advances past it and WriterFullBound keeps
// its bound. A held journal is the complete audit's own classification:
// staging present, or the missing receipt's refusal. Mutation: dropping the
// staging check in walkPrefix returns a prefix result that binds a
// checkpoint with an advanced FullSeq.
func TestCALV0265_RefreshUnderAppendsNeverAdvancesOverUnsettledHistory(t *testing.T) {
	for _, tc := range []struct {
		name    string
		history int
	}{
		{"stray-staging", 0},
		{"missing-beyond-tail", MaxWriterTail + 8},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo, r := refreshStore(t)
			for i := 0; i < tc.history; i++ {
				appendRequest(t, repo, fmt.Sprintf("history-%d", i))
			}
			receipts := filepath.Join(repo.StateDir, "receipts")
			if tc.history == 0 {
				name, _ := snapshot.ReceiptName(2)
				fixture.Write(t, filepath.Join(receipts, name+".tmp-1-x"), []byte("x"))
			} else {
				name, _ := snapshot.ReceiptName(2)
				if err := os.Remove(filepath.Join(receipts, name)); err != nil {
					t.Fatal(err)
				}
			}
			held, heldObs, heldErr := r.AuditWriterRefresh()
			if tc.history == 0 {
				if heldErr != nil || held == nil || !held.StagingPresent {
					t.Fatalf("held refresh with a stray temporary name: %+v %v, want staging present", held, heldErr)
				}
				if _, err := held.WriterCheckpoint(heldObs.Files); err == nil {
					t.Fatal("held refresh with a stray temporary name bound a checkpoint")
				}
			} else if heldErr == nil || wire.CodeOf(heldErr) == wire.CodeSnapshotMoved {
				t.Fatalf("held refresh with receipt 2 missing: %v, want a refusal", heldErr)
			}
			appends := 0
			r.afterCapture = func() {
				appends++
				appendRequest(t, repo, fmt.Sprintf("racing-%d", appends))
			}
			moving, _, err := r.AuditWriterRefresh()
			if wire.CodeOf(err) != wire.CodeSnapshotMoved || appends < 2 {
				t.Fatalf("refresh under appends: %+v %v after %d appends, want SNAPSHOT_MOVED past the prefix walk", moving, err, appends)
			}
		})
	}
}
