package store

import (
	"errors"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// ClaimDelivery is the context a successful claim or claim-next delivers from
// its own original admission (ON-V0-007). claimedTicket resolves it from the
// claim receipt's POST attempt for fresh and replayed claims alike, so a later
// note change moves neither the response nor its replay. Escalation answers
// (ESC-V0-005) are pinned on the same attempt and resolved the same way.
type ClaimDelivery struct {
	// TicketRecordSha256 is the admitted ticket record the snapshot came from.
	TicketRecordSha256 wire.Digest
	OperatorNote       ClaimedNote
	EscalationAnswers  ClaimedAnswers
	// KnowHow is computed when the response is built, not pinned at
	// admission (KHN-V0-006), because freshness is a read-time state.
	KnowHow ClaimedKnowHow
}

// ClaimedAnswers is the admitted answer references (empty when none) and,
// when every event resolves, the delivered guidance array. Err keeps missing
// or mismatched material visible; exact replay retries delivery. Delivery
// never consumes or acknowledges an answer.
type ClaimedAnswers struct {
	Refs  []snapshot.EscalationAnswerRef
	Value wire.Value
	Err   error
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
	d.EscalationAnswers = readClaimedAnswers(repo, a)
	return d
}

// readClaimedAnswers resolves the attempt's pinned answers from the evidence
// store, never from the current ticket. A missing blob is left absent so the
// resolver reports MISSING_EVIDENCE. It writes nothing.
func readClaimedAnswers(repo *intent.Repository, a *snapshot.Attempt) ClaimedAnswers {
	c := ClaimedAnswers{Refs: a.EscalationAnswers}
	blobs := map[wire.Digest][]byte{}
	for _, ref := range a.EscalationAnswers {
		for _, d := range []wire.Digest{ref.OriginSha256, ref.HeadSha256} {
			if c.Err = readEvidence(repo, d, ticket.EscalationMaxEventBytes, blobs); c.Err != nil {
				return c
			}
		}
	}
	_, c.Value, c.Err = transaction.ResolveEscalationAnswers(a.TicketID.QueueID(), a.TicketID.Raw, a.EscalationAnswers, blobs)
	return c
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

// claimGrantWarnings names the grants an APPROVAL_REQUIRED ticket still
// lacks through completion when a fresh claim succeeds (ATR-V0-274), so the
// holder learns of a missing COMPLETE grant before complete refuses it. It
// reads the intent record the claim bound and is silent when that record has
// moved since or cannot be read: the warning is advisory, never admission.
func claimGrantWarnings(repo *intent.Repository, a *snapshot.Attempt) []string {
	raw, err := intent.ReadFile(filepath.Join(repo.IntentRoot(), intent.Dir, "tickets", a.TicketID.Local+".json"), wire.MaxTicketFileBytes)
	if err != nil || wire.Sum(raw) != a.TicketRecordSha256 {
		return nil
	}
	rec, err := ticket.Decode(raw)
	if err != nil {
		return nil
	}
	missing := ticket.MissingGrants(rec)
	if len(missing) == 0 {
		return nil
	}
	return []string{"ticket " + a.TicketID.Raw + " is APPROVAL_REQUIRED and holds no " + strings.Join(missing, ",") + " grant at acceptanceRevision " + string(rec.AcceptanceRevision) + "; complete refuses APPROVAL_MISSING until an owner grants it"}
}
