package store_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// noteEnvelope builds one NOTE_SET/NOTE_CLEAR taskman-mutation/0; an empty
// expected or supersedes is null.
func noteEnvelope(requestID, operation, target, expected, text, supersedes string) []byte {
	rev, sup := wire.Null(), wire.Null()
	if expected != "" {
		rev = str(expected)
	}
	if supersedes != "" {
		sup = str(supersedes)
	}
	payload := obj("supersedes", sup)
	if operation == mutation.OpNoteSet {
		payload = obj("supersedes", sup, "text", str(text))
	}
	return wire.EncodeFile(obj(
		"profile", str(mutation.Profile),
		"requestId", str(requestID),
		"actor", obj("id", str("tester"), "role", str("OWNER")),
		"queueId", str(fixture.QueueID),
		"targetId", str(target),
		"expectedRevision", rev,
		"operation", str(operation),
		"payload", payload,
		"issuedAt", str(issued),
	))
}

func loadRecord(t *testing.T, repo *intent.Repository, id string) *ticket.Record {
	t.Helper()
	loaded, err := intent.Load(repo.PrimaryWorktree)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	rec, ok := loaded.Inventory.Get(id)
	if !ok {
		t.Fatalf("%s missing", id)
	}
	return rec
}

// TestONV0006_NativeNoteSetClearReplayAndAudit is the first native fixture:
// SET posts the ticket and its event in one MUTATE stage, the reference and
// event agree, identical replay returns the original outcome, a stale
// supersedes conflicts, CLEAR keeps a tombstone, and an ordinary REFINE then
// preserves the reference through a full audit.
func TestONV0006_NativeNoteSetClearReplayAndAudit(t *testing.T) {
	repo, _ := initialized(t)
	created := mutate(t, repo, envelope("req-create", mutation.OpCreate, "", "", createPayload("Noted ticket")))
	set := noteEnvelope("req-note-1", mutation.OpNoteSet, created.Ticket, "", "Use the staging database.", "0")
	first := mutate(t, repo, set)
	if first.Outcome.Outcome != mutation.OutcomeCompleted || first.Receipt == "" {
		t.Fatalf("set outcome = %s (%v) %s", first.Outcome.Outcome, first.Outcome.Codes, first.Detail)
	}
	rec := loadRecord(t, repo, created.Ticket)
	if rec.OperatorNote == nil || rec.OperatorNote.Revision != "1" || rec.OperatorNote.Current == nil || *rec.OperatorNote.Current != rec.OperatorNote.Head {
		t.Fatalf("reference after SET = %+v", rec.OperatorNote)
	}
	if rec.Revision != "2" || rec.AcceptanceRevision != "1" {
		t.Errorf("revisions = %s/%s, want 2/1", rec.Revision, rec.AcceptanceRevision)
	}
	raw, err := os.ReadFile(filepath.Join(repo.StateDir, "evidence", string(rec.OperatorNote.Head)))
	if err != nil {
		t.Fatalf("note event not stored: %v", err)
	}
	ev, err := ticket.ResolveOperatorNote(rec.TicketID, *rec.OperatorNote, raw)
	if err != nil || ev.Operation != "SET" || ev.TicketRevision != "2" {
		t.Fatalf("event = %+v, %v", ev, err)
	}

	before := storeDigest(t, repo)
	again := mutate(t, repo, set)
	if !again.Outcome.Replayed || again.Receipt != "" {
		t.Fatalf("identical NOTE_SET did not replay: %+v", again.Outcome)
	}
	stale := mutate(t, repo, noteEnvelope("req-note-stale", mutation.OpNoteSet, created.Ticket, "", "Other", "0"))
	if stale.Outcome.Outcome != mutation.OutcomeRevisionConflict || stale.Receipt != "" {
		t.Fatalf("stale supersedes = %s %s", stale.Outcome.Outcome, stale.Detail)
	}
	if storeDigest(t, repo) != before {
		t.Fatal("replay or conflict changed the store")
	}

	refined := mutate(t, repo, envelope("req-refine", mutation.OpRefine, created.Ticket, "2", obj("title", str("Refined"))))
	if refined.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("refine = %s %s", refined.Outcome.Outcome, refined.Detail)
	}
	after := loadRecord(t, repo, created.Ticket)
	if after.OperatorNote == nil || after.OperatorNote.Head != rec.OperatorNote.Head {
		t.Fatalf("REFINE dropped the reference: %+v", after.OperatorNote)
	}

	cleared := mutate(t, repo, noteEnvelope("req-note-2", mutation.OpNoteClear, created.Ticket, "3", "", "1"))
	if cleared.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("clear = %s (%v) %s", cleared.Outcome.Outcome, cleared.Outcome.Codes, cleared.Detail)
	}
	tomb := loadRecord(t, repo, created.Ticket)
	if tomb.OperatorNote == nil || tomb.OperatorNote.Revision != "2" || tomb.OperatorNote.Current != nil {
		t.Fatalf("reference after CLEAR = %+v", tomb.OperatorNote)
	}
}
