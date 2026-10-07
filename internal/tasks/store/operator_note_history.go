package store

import (
	"bytes"
	"encoding/base64"
	"errors"
	"io/fs"
	"path/filepath"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// Operator-note history page bounds (ON-V0-008).
const (
	OperatorNoteHistoryDefault = 20
	OperatorNoteHistoryMax     = 50
	OperatorNoteHistoryBytes   = 1 << 20
	operatorNoteCursorProfile  = "taskman-operator-note-cursor/0"
	maxOperatorNoteCursorBytes = 1024
)

// OperatorNoteCursor is the opaque continuation of one anchored history read.
// It binds the queue, ticket, anchor head and revision, and the next event's
// digest and revision; it never names a path.
type OperatorNoteCursor struct {
	QueueID, TicketID string
	Anchor            wire.Digest
	AnchorRevision    wire.Count
	Next              wire.Digest
	NextRevision      wire.Count
}

func (c OperatorNoteCursor) canonical() []byte {
	o := wire.NewObject()
	o.Set("profile", wire.String(operatorNoteCursorProfile)).Set("queueId", wire.String(c.QueueID)).Set("ticketId", wire.String(c.TicketID))
	o.Set("anchor", wire.String(string(c.Anchor))).Set("anchorRevision", wire.String(string(c.AnchorRevision)))
	o.Set("next", wire.String(string(c.Next))).Set("nextRevision", wire.String(string(c.NextRevision)))
	return wire.EncodeFile(wire.ObjectValue(o))
}

// Encode renders the cursor as unpadded base64url of its canonical object.
func (c OperatorNoteCursor) Encode() string {
	return base64.RawURLEncoding.EncodeToString(c.canonical())
}

// DecodeOperatorNoteCursor admits only a cursor this reader encoded: strict
// base64url of the closed canonical object, re-encoded byte for byte.
func DecodeOperatorNoteCursor(s string) (*OperatorNoteCursor, error) {
	bad := func(msg string) error { return wire.Errorf(wire.CodeMalformed, "--cursor", "%s", msg) }
	if s == "" || len(s) > maxOperatorNoteCursorBytes {
		return nil, bad("cursor must be 1..1024 bytes")
	}
	raw, err := base64.RawURLEncoding.Strict().DecodeString(s)
	if err != nil {
		return nil, bad("cursor is not a history cursor")
	}
	v, err := wire.Parse(raw)
	if err != nil {
		return nil, bad("cursor is not a history cursor")
	}
	r := wire.NewReader(v, "--cursor")
	if err := r.Profile(operatorNoteCursorProfile); err != nil {
		return nil, err
	}
	r.Closed("profile", "queueId", "ticketId", "anchor", "anchorRevision", "next", "nextRevision")
	c := &OperatorNoteCursor{QueueID: r.Field("queueId").QueueID().Raw, TicketID: r.Field("ticketId").TicketID().Raw,
		Anchor: r.Field("anchor").Digest(), AnchorRevision: r.Field("anchorRevision").Count(),
		Next: r.Field("next").Digest(), NextRevision: r.Field("nextRevision").Count()}
	profile := r.Field("profile").String()
	if err := r.Err(); err != nil {
		return nil, err
	}
	if profile != operatorNoteCursorProfile || !bytes.Equal(c.canonical(), raw) {
		return nil, bad("cursor is not a canonical history cursor")
	}
	if c.NextRevision.Int() < 1 || c.NextRevision.Int() >= c.AnchorRevision.Int() || c.AnchorRevision.Int() > ticket.MaxOperatorNoteEvents {
		return nil, bad("cursor revisions must satisfy 1 <= next < anchor <= 4096")
	}
	return c, nil
}

// OperatorNoteHistoryEntry is one verified event of a history page.
type OperatorNoteHistoryEntry struct {
	Sha256  wire.Digest
	Event   *ticket.OperatorNoteEvent
	Request *ticket.OperatorNoteRequest
}

// OperatorNoteHistoryPage is one newest-first page. Next is nil once the
// page reaches the first event.
type OperatorNoteHistoryPage struct {
	Anchor         wire.Digest
	AnchorRevision wire.Count
	Entries        []OperatorNoteHistoryEntry
	Next           *OperatorNoteCursor
}

// OperatorNoteHistory reads one ticket's note events newest first (ON-V0-008).
// Every read walks from the ticket's committed head reference: each step
// checks the digest, closed profile and ticket, an exact revision decrement
// and the previous link, so a missing, cyclic or mismatched event refuses
// instead of shortening the history. Without a cursor the page is anchored at
// the head; a cursor's anchor and next event must both lie on that walk at
// their revisions, so a concurrent replacement never moves later pages off
// their anchor. A page holds at most limit entries and its entries' rendered
// bytes stay within budget (one entry always fits). It writes nothing.
func OperatorNoteHistory(repo *intent.Repository, id wire.TicketID, ref ticket.OperatorNoteReference, cursor *OperatorNoteCursor, limit int, size func(OperatorNoteHistoryEntry) int) (*OperatorNoteHistoryPage, error) {
	if limit < 1 || limit > OperatorNoteHistoryMax {
		return nil, wire.Errorf(wire.CodeMalformed, "--limit", "limit must be 1..%d", OperatorNoteHistoryMax)
	}
	forked := func(format string, a ...any) error {
		return wire.Errorf(wire.CodeJournalForked, "/operatorNote", "note history of %s "+format, append([]any{id.Raw}, a...)...)
	}
	page := &OperatorNoteHistoryPage{Anchor: ref.Head, AnchorRevision: ref.Revision, Entries: []OperatorNoteHistoryEntry{}}
	startRevision := ref.Revision.Int()
	if cursor != nil {
		if cursor.QueueID != id.QueueID() || cursor.TicketID != id.Raw {
			return nil, wire.Errorf(wire.CodeMalformed, "--cursor", "cursor belongs to another ticket")
		}
		if cursor.AnchorRevision.Int() > ref.Revision.Int() {
			return nil, wire.Errorf(wire.CodeMalformed, "--cursor", "cursor anchor is newer than the committed note head")
		}
		page.Anchor, page.AnchorRevision, startRevision = cursor.Anchor, cursor.AnchorRevision, cursor.NextRevision.Int()
	}
	at, used := ref.Head, 0
	for revision := ref.Revision.Int(); ; revision-- {
		raw, err := intent.ReadFile(filepath.Join(repo.StateDir, "evidence", string(at)), ticket.MaxOperatorNoteBytes)
		if errors.Is(err, fs.ErrNotExist) {
			return nil, wire.Errorf(wire.CodeMissingEvidence, "/operatorNote", "note event %s (revision %d) is not in the evidence store", at, revision)
		}
		if err != nil {
			return nil, err
		}
		if wire.Sum(raw) != at {
			return nil, forked("event %s differs from its digest", at)
		}
		var event *ticket.OperatorNoteEvent
		if revision == ref.Revision.Int() {
			event, err = ticket.ResolveOperatorNote(id, ref, raw)
		} else {
			event, err = ticket.DecodeOperatorNoteEvent(raw)
		}
		if err != nil {
			return nil, err
		}
		if event.TicketID.Raw != id.Raw || event.NoteRevision.Int() != revision {
			return nil, forked("event %s is not revision %d of the ticket", at, revision)
		}
		if cursor != nil && revision == cursor.AnchorRevision.Int() && at != cursor.Anchor {
			return nil, wire.Errorf(wire.CodeMalformed, "--cursor", "cursor anchor is not on the committed note chain")
		}
		if cursor != nil && revision == cursor.NextRevision.Int() && at != cursor.Next {
			return nil, wire.Errorf(wire.CodeMalformed, "--cursor", "cursor next event is not on the committed note chain")
		}
		if revision <= startRevision {
			request, err := ticket.DecodeOperatorNoteRequest(event.Request)
			if err != nil {
				return nil, err
			}
			entry := OperatorNoteHistoryEntry{Sha256: at, Event: event, Request: request}
			n := size(entry)
			if len(page.Entries) == limit || (len(page.Entries) > 0 && used+n > OperatorNoteHistoryBytes) {
				page.Next = &OperatorNoteCursor{QueueID: id.QueueID(), TicketID: id.Raw, Anchor: page.Anchor, AnchorRevision: page.AnchorRevision, Next: at, NextRevision: wire.CountOf(int64(revision))}
				return page, nil
			}
			page.Entries, used = append(page.Entries, entry), used+n
		}
		if revision == 1 {
			if event.Previous != nil {
				return nil, forked("first event names a previous event")
			}
			return page, nil
		}
		if event.Previous == nil {
			return nil, forked("event %s ends the chain at revision %d", at, revision)
		}
		at = *event.Previous
	}
}
