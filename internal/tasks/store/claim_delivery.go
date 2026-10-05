package store

import (
	"errors"
	"io/fs"
	"path/filepath"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// ClaimDelivery is the context a successful claim or claim-next delivers from
// its own original admission (ON-V0-007). claimedTicket resolves it from the
// claim receipt's POST attempt for fresh and replayed claims alike, so a later
// note change moves neither the response nor its replay. Further admitted
// context, such as escalation answers (ESC-V0-005), is one more field beside
// OperatorNote, pinned on the same attempt and resolved here the same way.
type ClaimDelivery struct {
	// TicketRecordSha256 is the admitted ticket record the snapshot came from.
	TicketRecordSha256 wire.Digest
	OperatorNote       ClaimedNote
}

// ClaimedNote is the admitted note reference (nil means NONE) and, when it
// resolves, its event and original request. Err keeps a missing or mismatched
// event visible; the committed claim stands and exact replay retries delivery.
type ClaimedNote struct {
	Reference *ticket.OperatorNoteReference
	Event     *ticket.OperatorNoteEvent
	Request   *ticket.OperatorNoteRequest
	Err       error
}

func claimDelivery(repo *intent.Repository, a *snapshot.Attempt) *ClaimDelivery {
	d := &ClaimDelivery{TicketRecordSha256: a.TicketRecordSha256, OperatorNote: ClaimedNote{Reference: a.OperatorNote}}
	if a.OperatorNote != nil {
		d.OperatorNote.Event, d.OperatorNote.Request, d.OperatorNote.Err = ReadOperatorNote(repo, a.TicketID, *a.OperatorNote)
	}
	return d
}

// ReadOperatorNote resolves one note reference against the content-addressed
// evidence store: a missing event is MISSING_EVIDENCE and a mismatched one
// fails its closed decoders, never NONE. It writes nothing.
func ReadOperatorNote(repo *intent.Repository, id wire.TicketID, ref ticket.OperatorNoteReference) (*ticket.OperatorNoteEvent, *ticket.OperatorNoteRequest, error) {
	raw, err := intent.ReadFile(filepath.Join(repo.StateDir, "evidence", string(ref.Head)), ticket.MaxOperatorNoteBytes)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil, wire.Errorf(wire.CodeMissingEvidence, "/operatorNote/head", "note event %s is not in the evidence store", ref.Head)
	}
	if err != nil {
		return nil, nil, err
	}
	event, err := ticket.ResolveOperatorNote(id, ref, raw)
	if err != nil {
		return nil, nil, err
	}
	request, err := ticket.DecodeOperatorNoteRequest(event.Request)
	if err != nil {
		return nil, nil, err
	}
	return event, request, nil
}
