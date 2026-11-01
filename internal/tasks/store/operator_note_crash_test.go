package store_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestONV0871_NoteSetCrashAtEveryArtifactRecovers (ON-V0-871, V1-0814 item
// 6): a NOTE_SET interrupted before each artifact it publishes, in turn,
// leaves a store whose retry of the same request completes exactly once with
// note revision 1 resolving to its event, and whose receipt audit passes.
func TestONV0871_NoteSetCrashAtEveryArtifactRecovers(t *testing.T) {
	var roles []string
	{
		repo, _ := initialized(t)
		created := mutate(t, repo, envelope("req-create", mutation.OpCreate, "", "", createPayload("Noted ticket")))
		restore := store.SetPublishFaultForTest(func(a transaction.Artifact) error {
			roles = append(roles, a.Role+" "+a.Target)
			return nil
		})
		mutate(t, repo, noteEnvelope("req-note-1", mutation.OpNoteSet, created.Ticket, "", "Crash witness.", "0"))
		restore()
	}
	if len(roles) < 3 {
		t.Fatalf("NOTE_SET published %d artifacts: %v", len(roles), roles)
	}
	fault := errors.New("injected crash")
	for k := range roles {
		repo, _ := initialized(t)
		created := mutate(t, repo, envelope("req-create", mutation.OpCreate, "", "", createPayload("Noted ticket")))
		set := noteEnvelope("req-note-1", mutation.OpNoteSet, created.Ticket, "", "Crash witness.", "0")
		seen := 0
		restore := store.SetPublishFaultForTest(func(transaction.Artifact) error {
			seen++
			if seen == k+1 {
				return fault
			}
			return nil
		})
		_, err := store.Mutate(context.Background(), repo, operator(), set, now(t))
		restore()
		if !errors.Is(err, fault) {
			t.Fatalf("crash before %s: err = %v", roles[k], err)
		}
		retry, err := store.Mutate(context.Background(), repo, operator(), set, now(t))
		if err != nil || retry.Outcome.Outcome != mutation.OutcomeCompleted {
			t.Fatalf("retry after a crash before %s = %+v, %v", roles[k], retry, err)
		}
		rec := loadRecord(t, repo, created.Ticket)
		if rec.OperatorNote == nil || rec.OperatorNote.Revision != "1" || rec.OperatorNote.Current == nil {
			t.Fatalf("crash before %s: reference = %+v", roles[k], rec.OperatorNote)
		}
		raw, err := os.ReadFile(filepath.Join(repo.StateDir, "evidence", string(rec.OperatorNote.Head)))
		if err != nil {
			t.Fatalf("crash before %s: event: %v", roles[k], err)
		}
		if ev, err := ticket.ResolveOperatorNote(rec.TicketID, *rec.OperatorNote, raw); err != nil || ev.NoteRevision != "1" || !bytes.Contains(ev.Request, []byte("Crash witness.")) {
			t.Fatalf("crash before %s: event = %+v, %v", roles[k], ev, err)
		}
		if _, err := noteAudit(t, repo); err != nil {
			t.Fatalf("crash before %s: audit: %v", roles[k], err)
		}
		t.Logf("crash before %s: retry replayed=%t redone=%t", roles[k], retry.Outcome.Replayed, retry.Redone)
	}
}

// TestONV0874_ClaimResponseFailureAfterCommitIsRetriedByReplay (ON-V0-874,
// V1-0814 item 6): the note event vanishing after a claim's receipt and before
// its response materialization keeps the committed lease, surfaces
// MISSING_EVIDENCE on the delivery rather than NONE, refuses a second claim
// instead of re-leasing, and, once the event is restored, the exact claim
// replay delivers the admitted note under the same attempt.
func TestONV0874_ClaimResponseFailureAfterCommitIsRetriedByReplay(t *testing.T) {
	s := newLeaseStore(t)
	noted := s.ticket(t, "noted")
	s.note(t, noteEnvelope("note-1", mutation.OpNoteSet, noted, "", "Pinned.", "0"), 1)
	ref := loadRecord(t, s.repo, noted).OperatorNote
	event := filepath.Join(s.repo.StateDir, "evidence", string(ref.Head))
	raw := mustRead(t, event)
	restore := store.SetPublishFaultForTest(func(a transaction.Artifact) error {
		if a.Role == "HEAD" {
			return os.Remove(event)
		}
		return nil
	})
	first := s.lease(t, "claim-1", claimOf(noted), 2, nil)
	restore()
	if first.Outcome.Outcome != mutation.OutcomeCompleted || first.AttemptID == "" || first.Delivery == nil ||
		first.Delivery.OperatorNote.Reference == nil || wire.CodeOf(first.Delivery.OperatorNote.Err) != wire.CodeMissingEvidence {
		t.Fatalf("claim with a vanished event = %+v", first)
	}
	if a := s.attempt(t, first.AttemptID); a.OperatorNote == nil || a.OperatorNote.Head != ref.Head {
		t.Fatalf("committed attempt pin = %+v", a.OperatorNote)
	}
	fixture.Write(t, event, raw)
	choice := store.LeaseChoice{QueueID: fixture.QueueID, RequestID: "claim-2", Root: s.root, Lease: claimOf(noted)}
	if again, err := store.Lease(context.Background(), s.repo, operator(), choice, s.at(t, 3)); err == nil && again.Outcome.Outcome == mutation.OutcomeCompleted {
		t.Fatalf("a second claim re-leased the committed ticket: %+v", again)
	}
	replay := s.lease(t, "claim-1", claimOf(noted), 4, nil)
	if !replay.Outcome.Replayed || replay.AttemptID != first.AttemptID || deliveredNote(t, replay) != "Pinned." {
		t.Fatalf("exact replay = %+v", replay)
	}
}
