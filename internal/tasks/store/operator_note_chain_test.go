package store_test

import (
	"path/filepath"
	"strconv"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestONV0872_FullChainHistoryPagesEveryEvent (ON-V0-872, V1-0814 item 3):
// a ticket's note chain at the 4096-event cap, extended from one native
// NOTE_SET by events that pass the closed event decoder, pages newest first
// through every event with exact revisions and links, and revision 4097 is
// refused LIMIT_EXCEEDED by both the reference and the event encoder. The
// writer itself is not driven 4096 times (about 170 ms per CLI write).
func TestONV0872_FullChainHistoryPagesEveryEvent(t *testing.T) {
	t.Parallel()
	repo, _ := initialized(t)
	created := mutate(t, repo, envelope("req-create", mutation.OpCreate, "", "", createPayload("Long chain")))
	if r := mutate(t, repo, noteEnvelope("req-note-1", mutation.OpNoteSet, created.Ticket, "", "Revision 1.", "0")); r.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("set = %s %s", r.Outcome.Outcome, r.Detail)
	}
	rec := loadRecord(t, repo, created.Ticket)
	head := rec.OperatorNote.Head
	first, err := ticket.DecodeOperatorNoteEvent(mustRead(t, filepath.Join(repo.StateDir, "evidence", string(head))))
	if err != nil {
		t.Fatal(err)
	}
	extend := func(revision int, previous wire.Digest) (ticket.OperatorNoteEvent, []byte) {
		q, err := wire.Parse(first.Request)
		if err != nil {
			t.Fatal(err)
		}
		n := strconv.Itoa(revision)
		q.Obj.Set("requestId", wire.String("req-note-"+n))
		payload, _ := q.Obj.Get("payload")
		payload.Obj.Set("supersedes", wire.String(strconv.Itoa(revision-1)))
		payload.Obj.Set("text", wire.String("Revision "+n+"."))
		ev := *first
		ev.NoteRevision, ev.Previous = wire.CountOf(int64(revision)), &previous
		ev.Request = wire.EncodeFile(q)
		ev.RequestSha256 = wire.Sum(ev.Request)
		raw, err := ev.Encode()
		if err != nil {
			t.Fatalf("event %d: %v", revision, err)
		}
		return ev, raw
	}
	for revision := 2; revision <= ticket.MaxOperatorNoteEvents; revision++ {
		_, raw := extend(revision, head)
		head = wire.Sum(raw)
		fixture.Write(t, filepath.Join(repo.StateDir, "evidence", string(head)), raw)
	}
	ref := ticket.OperatorNoteReference{Revision: wire.CountOf(ticket.MaxOperatorNoteEvents), Current: &head, Head: head}
	size := func(e store.OperatorNoteHistoryEntry) int { return len(e.Event.Request) }
	var cursor *store.OperatorNoteCursor
	want, pages := ticket.MaxOperatorNoteEvents, 0
	var last wire.Digest
	for {
		page, err := store.OperatorNoteHistory(repo, rec.TicketID, ref, cursor, store.OperatorNoteHistoryMax, size)
		if err != nil {
			t.Fatalf("page %d: %v", pages, err)
		}
		pages++
		for _, e := range page.Entries {
			if int(e.Event.NoteRevision.Int()) != want || (last != "" && e.Sha256 != last) {
				t.Fatalf("entry %s revision %s, want %d linked from the newer event", e.Sha256, e.Event.NoteRevision, want)
			}
			if e.Event.Previous != nil {
				last = *e.Event.Previous
			}
			want--
		}
		if page.Next == nil {
			break
		}
		cursor = page.Next
	}
	if want != 0 || pages != (ticket.MaxOperatorNoteEvents+store.OperatorNoteHistoryMax-1)/store.OperatorNoteHistoryMax {
		t.Fatalf("walked to revision %d in %d pages", want+1, pages)
	}
	over := ref
	over.Revision = wire.CountOf(ticket.MaxOperatorNoteEvents + 1)
	if _, err := ticket.OperatorNoteReferenceFromValue(over.Value()); wire.CodeOf(err) != wire.CodeLimitExceeded {
		t.Fatalf("revision 4097 reference: %v", err)
	}
	ev, _ := extend(ticket.MaxOperatorNoteEvents, head)
	ev.NoteRevision = wire.CountOf(ticket.MaxOperatorNoteEvents + 1)
	if _, err := ev.Encode(); wire.CodeOf(err) != wire.CodeLimitExceeded {
		t.Fatalf("revision 4097 event: %v", err)
	}
}
