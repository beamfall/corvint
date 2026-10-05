package store_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// deliveredNote returns the claim's delivered note text, "" for NONE and
// "<cleared>" for a tombstone, failing on an unresolved event.
func deliveredNote(t *testing.T, r *store.Report) string {
	t.Helper()
	if r.Delivery == nil {
		t.Fatalf("claim %s delivered nothing", r.AttemptID)
	}
	n := r.Delivery.OperatorNote
	if n.Err != nil {
		t.Fatalf("claim %s note unresolved: %v", r.AttemptID, n.Err)
	}
	switch {
	case n.Reference == nil:
		return ""
	case n.Reference.Current == nil:
		return "<cleared>"
	}
	return n.Request.Text
}

// note commits one NOTE_SET/NOTE_CLEAR on the lease clock, t0 plus minutes.
func (s *leaseStore) note(t *testing.T, env []byte, minutes int) {
	t.Helper()
	r, err := store.Mutate(context.Background(), s.repo, operator(), env, s.at(t, minutes))
	if err != nil || r.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("note = %+v, %v", r, err)
	}
}

// TestONV0007_ClaimDeliversTheNotePinnedByItsAdmission: a claim delivers the
// note of its own admitted ticket snapshot. A later note change moves neither
// the live response nor its exact replay; a new generation and claim-next
// take their own snapshot; a never-noted claim keeps legacy attempt bytes.
func TestONV0007_ClaimDeliversTheNotePinnedByItsAdmission(t *testing.T) {
	s := newLeaseStore(t)
	plain := s.planned(t, "plain", "P2", "docs/")
	noted := s.planned(t, "noted", "P1", "src/")

	none := s.claim(t, "claim-plain", plain, 0)
	if got := deliveredNote(t, none); got != "" {
		t.Fatalf("never-noted claim delivered %q", got)
	}
	raw, err := os.ReadFile(filepath.Join(s.repo.StateDir, "attempts", none.AttemptID+".json"))
	if err != nil || strings.Contains(string(raw), "operatorNote") {
		t.Fatalf("never-noted attempt carries a note pin (%v)", err)
	}

	s.note(t, noteEnvelope("note-1", mutation.OpNoteSet, noted, "", "First.", "0"), 1)
	first := s.claim(t, "claim-1", noted, 2)
	if got := deliveredNote(t, first); got != "First." {
		t.Fatalf("claim delivered %q, want the admitted note", got)
	}
	a := s.attempt(t, first.AttemptID)
	if a.OperatorNote == nil || a.OperatorNote.Revision != "1" || first.Delivery.TicketRecordSha256 != a.TicketRecordSha256 {
		t.Fatalf("attempt pin = %+v, delivery digest %s, attempt digest %s", a.OperatorNote, first.Delivery.TicketRecordSha256, a.TicketRecordSha256)
	}
	if ev := first.Delivery.OperatorNote.Event; ev.Operation != "SET" || ev.ActorID != "tester" {
		t.Fatalf("delivered provenance = %+v", ev)
	}

	// A post-commit replacement changes neither the live claim nor its replay.
	s.note(t, noteEnvelope("note-2", mutation.OpNoteSet, noted, "", "Second.", "1"), 3)
	replay := s.lease(t, "claim-1", claimOf(noted), 4, nil)
	if !replay.Outcome.Replayed || replay.AttemptID != first.AttemptID || deliveredNote(t, replay) != "First." {
		t.Fatalf("replay = %+v delivered %q", replay.Outcome, deliveredNote(t, replay))
	}

	// A new generation re-snapshots from its own admission.
	if r := s.lease(t, "release-1", releaseOf(first), 5, nil); r.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("release = %+v %s", r.Outcome, r.Detail)
	}
	second := s.claim(t, "claim-2", noted, 6)
	if second.Generation == first.Generation || deliveredNote(t, second) != "Second." {
		t.Fatalf("new generation %s delivered %q", second.Generation, deliveredNote(t, second))
	}

	// claim-next delivers a CLEARED tombstone, not NONE.
	if r := s.lease(t, "release-2", releaseOf(second), 7, nil); r.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("release = %+v %s", r.Outcome, r.Detail)
	}
	s.note(t, noteEnvelope("note-3", mutation.OpNoteClear, noted, "", "", "2"), 8)
	next := s.lease(t, "next-1", claimNext, 9, nil)
	if next.Outcome.Outcome != mutation.OutcomeCompleted || next.Ticket != noted || deliveredNote(t, next) != "<cleared>" {
		t.Fatalf("claim-next = %+v ticket %s", next.Outcome, next.Ticket)
	}
}

// TestONV0007_UnresolvableNoteFailsClosed: once a claim pinned a note, a
// missing event is never delivered as NONE. The store read reports
// MISSING_EVIDENCE, and the exact claim replay refuses before delivery
// because the full receipt audit cannot bind the absent note history.
func TestONV0007_UnresolvableNoteFailsClosed(t *testing.T) {
	s := newLeaseStore(t)
	noted := s.ticket(t, "noted")
	s.note(t, noteEnvelope("note-1", mutation.OpNoteSet, noted, "", "First.", "0"), 1)
	first := s.claim(t, "claim-1", noted, 2)
	a := s.attempt(t, first.AttemptID)
	if err := os.Remove(filepath.Join(s.repo.StateDir, "evidence", string(a.OperatorNote.Head))); err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.ReadOperatorNote(s.repo, a.TicketID, *a.OperatorNote); wire.CodeOf(err) != wire.CodeMissingEvidence {
		t.Fatalf("read of a missing event = %v, want MISSING_EVIDENCE", err)
	}
	choice := store.LeaseChoice{QueueID: fixture.QueueID, RequestID: "claim-1", Root: s.root, Lease: claimOf(noted)}
	if replay, err := store.Lease(context.Background(), s.repo, operator(), choice, s.at(t, 3)); err == nil || wire.CodeOf(err) != wire.CodeJournalForked {
		t.Fatalf("replay over a missing event = %+v, %v; want JOURNAL_FORKED", replay, err)
	}
}
