package transaction

import (
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// ExternalReviewReceiptAudit binds every review event to the receipt that
// records it (ERG-V0-009, the operator-notes slot-reuse rule). Receipts are
// folded in sequence order; a ticket post that adds or changes a gate
// reference must be a MUTATION receipt that changes exactly one gate, posts
// exactly that head event, and whose event names the receipt's sequence,
// request, actor, time, ticket, gate, counters, predecessor and the post's
// acceptance revision. A dropped reference is refused. It is pure: blob
// returns retained evidence bytes by digest.
type ExternalReviewReceiptAudit struct {
	refs map[string]map[string]ticket.ExternalReviewRef
}

func (a *ExternalReviewReceiptAudit) fail(rc *snapshot.Receipt, f string, args ...any) error {
	return wire.Errorf(wire.CodeJournalForked, "receipts/"+string(rc.Seq), "external review binding: "+f, args...)
}

// Step folds one validated receipt.
func (a *ExternalReviewReceiptAudit) Step(rc *snapshot.Receipt, blob ExternalReviewBlob) error {
	if a.refs == nil {
		a.refs = map[string]map[string]ticket.ExternalReviewRef{}
	}
	events := map[wire.Digest]bool{}
	for _, p := range rc.Post {
		if strings.HasPrefix(p.Path, "evidence/") && p.Sha256 != nil {
			events[*p.Sha256] = true
		}
	}
	for _, p := range rc.Post {
		if !strings.HasPrefix(p.Path, "intent/tickets/") {
			continue
		}
		path := p.Path
		if p.Sha256 == nil {
			if len(a.refs[path]) != 0 {
				return a.fail(rc, "%s dropped its external review references", path)
			}
			continue
		}
		var raw []byte
		if p.Record != nil {
			raw = wire.EncodeFile(*p.Record)
		} else if p.BlobSha256 != nil {
			var ok bool
			if raw, ok = blob(*p.BlobSha256); !ok {
				return a.fail(rc, "%s post bytes are absent", path)
			}
		}
		rec, err := ticket.Decode(raw)
		if err != nil {
			return a.fail(rc, "%s post is not a ticket record: %v", path, err)
		}
		old := a.refs[path]
		changed := []string{}
		for gate, ref := range rec.ExternalReviews {
			if prev, ok := old[gate]; !ok || prev != ref {
				changed = append(changed, gate)
			}
		}
		for gate := range old {
			if _, ok := rec.ExternalReviews[gate]; !ok {
				return a.fail(rc, "%s dropped gate %s", path, gate)
			}
		}
		if len(changed) == 0 {
			a.refs[path] = rec.ExternalReviews
			continue
		}
		if rc.Kind != "MUTATION" || len(changed) != 1 || rc.RequestID == nil {
			return a.fail(rc, "%s changed review references outside one review mutation", path)
		}
		gate := changed[0]
		ref := rec.ExternalReviews[gate]
		if !events[ref.Head] {
			return a.fail(rc, "gate %s head event is not posted by its receipt", gate)
		}
		ev, ok := blob(ref.Head)
		if !ok {
			return a.fail(rc, "gate %s head event is absent", gate)
		}
		e, err := externalRefEvent(externalRefOf(ref), ev)
		if err != nil {
			return a.fail(rc, "gate %s head event: %v", gate, err)
		}
		q := e.Request
		var previous *wire.Digest
		if prev, ok := old[gate]; ok {
			h := prev.Head
			previous = &h
		}
		if q.TicketID != rec.TicketID.Raw || q.GateID != gate || !externalDigestEqual(e.Previous, previous) || e.ReceiptSeq != rc.Seq || q.RequestID != *rc.RequestID || e.ActorID != rc.ActorID || e.ActorRole != rc.ActorRole || e.RecordedAt != rc.RecordedAt || q.AcceptanceRevision != rec.AcceptanceRevision {
			return a.fail(rc, "gate %s head event does not record this receipt's transition", gate)
		}
		a.refs[path] = rec.ExternalReviews
	}
	return nil
}

func externalDigestEqual(a, b *wire.Digest) bool {
	return (a == nil && b == nil) || (a != nil && b != nil && *a == *b)
}
