package journal

import (
	"bytes"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// noteAudit binds one receipt's operator-note effects (ON-V0-006). step
// feeds it every post with the canonical latest it replaces; bind then
// requires that a receipt carrying a note event is exactly the NOTE_SET or
// NOTE_CLEAR its retained request replays to from the audited historical
// pre-ticket and pre-policy, and that every other receipt leaves each ticket's
// note reference unchanged. A pending receipt is folded by the same step, so
// the PRE_OR_POST audit that authorizes supported redo binds it too.
type noteAudit struct {
	r       Reader
	st      *chain
	rc      *snapshot.Receipt
	tickets []notePost
	events  [][]byte
	policy  bool
}

// noteState is what the walk has observed so far: each walked ticket's note
// reference and whether the policy was posted by a walked receipt. A complete
// audit walks every path from genesis; a checkpoint-resumed read starts
// empty and never reads a receipt before its checkpoint (CAL-V0-061).
type noteState struct {
	refs         map[string]*ticket.OperatorNoteReference
	policyWalked bool
}

type notePost struct {
	path  string
	index int
	prior latest
	raw   []byte
}

func (r Reader) noteAudit(st *chain, rc *snapshot.Receipt) *noteAudit {
	return &noteAudit{r: r, st: st, rc: rc}
}

// isOperatorNoteEvent reports an evidence post that decodes as a note event.
// Its semantic coverage is earned only through bind, which fails the receipt
// otherwise; every other evidence profile stays UNKNOWN.
func isOperatorNoteEvent(p string, raw []byte) bool {
	if !strings.HasPrefix(p, "evidence/") {
		return false
	}
	_, err := ticket.DecodeOperatorNoteEvent(raw)
	return err == nil
}

func (n *noteAudit) observe(j int, p snapshot.PostEntry, prior latest, raw []byte) {
	switch {
	case strings.HasPrefix(p.Path, "intent/tickets/"):
		n.tickets = append(n.tickets, notePost{path: p.Path, index: j, prior: prior, raw: raw})
	case p.Path == "intent/policy.json":
		n.policy = true
	case raw != nil && isOperatorNoteEvent(p.Path, raw):
		n.events = append(n.events, raw)
	}
}

func noteForked(where, format string, args ...any) error {
	return wire.Errorf(wire.CodeJournalForked, where, "operator note: "+format, args...)
}

func sameReference(a, b *ticket.OperatorNoteReference) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return bytes.Equal(wire.Encode(a.Value()), wire.Encode(b.Value()))
}

// bind runs after the post loop. target is the decoded receipt-target ticket
// post, reused rather than decoded again.
func (n *noteAudit) bind(req *snapshot.Request, target *ticket.Record) error {
	if n.st.notes == nil {
		n.st.notes = &noteState{refs: map[string]*ticket.OperatorNoteReference{}}
	}
	if len(n.events) > 1 {
		return noteForked("receipt", "more than one note event in one receipt")
	}
	posts := make([]*ticket.OperatorNoteReference, len(n.tickets))
	for i, t := range n.tickets {
		if t.raw == nil {
			continue
		}
		rec := target
		if rec == nil || t.path != "intent/tickets/"+rec.TicketID.Local+".json" {
			decoded, err := ticket.Decode(t.raw)
			if err != nil {
				return err
			}
			rec = decoded
		}
		posts[i] = rec.OperatorNote
	}
	if len(n.events) == 1 {
		if err := n.bindEvent(req, target, posts); err != nil {
			return err
		}
	} else {
		for i, t := range n.tickets {
			pre, known := n.st.notes.refs[t.path]
			if !known && t.prior.seq != "" {
				// A checkpoint-resumed read cannot see a reference posted
				// before its checkpoint; the complete audit binds it.
				continue
			}
			if t.raw != nil && !sameReference(pre, posts[i]) {
				return noteForked(t.path, "reference changed without its note event")
			}
		}
	}
	for i, t := range n.tickets {
		if t.raw == nil {
			delete(n.st.notes.refs, t.path)
			continue
		}
		n.st.notes.refs[t.path] = posts[i]
	}
	n.st.notes.policyWalked = n.st.notes.policyWalked || n.policy
	return nil
}

// bindEvent proves the one note receipt: a completed MUTATION with one request
// afterimage, one ticket post naming the receipt target, and post/event bytes
// equal to the ordinary transition replayed from the exact retained
// pre-ticket, pre-policy and prior event.
func (n *noteAudit) bindEvent(req *snapshot.Request, target *ticket.Record, posts []*ticket.OperatorNoteReference) error {
	rc := n.rc
	if rc.Kind != "MUTATION" || rc.RequestID == nil || req == nil || req.Entry.Outcome.Outcome != mutation.OutcomeCompleted || rc.TicketID == nil || target == nil {
		return noteForked("receipt", "event requires one completed MUTATION request on a ticket")
	}
	if len(n.tickets) != 1 || n.tickets[0].raw == nil || n.tickets[0].path != "intent/tickets/"+rc.TicketID.Local+".json" || n.policy {
		return noteForked("receipt", "event requires exactly the target ticket post and no policy post")
	}
	t := n.tickets[0]
	if t.prior.seq == "" || !equalDigest(rc.Pre[t.index].Sha256, t.prior.digest) {
		return noteForked(t.path, "pre-ticket is not the audited canonical afterimage")
	}
	if _, walked := n.st.notes.refs[t.path]; !walked || !n.st.notes.policyWalked {
		return errCheckpoint(t.path, "note transition pre-state precedes the checkpoint")
	}
	event, err := ticket.DecodeOperatorNoteEvent(n.events[0])
	if err != nil {
		return err
	}
	if wire.Sum(event.Request) != req.Entry.MutationSha256 || event.RequestSha256 != req.Entry.MutationSha256 {
		return noteForked("receipt", "event request differs from the retained request digest")
	}
	preRaw, err := n.r.priorPost(t.prior, t.path)
	if err != nil {
		return err
	}
	pre, err := ticket.Decode(preRaw)
	if err != nil {
		return err
	}
	policyRaw, err := n.r.priorPost(n.st.canonical["intent/policy.json"], "intent/policy.json")
	if err != nil {
		return err
	}
	policy, err := intent.DecodePolicy(policyRaw)
	if err != nil {
		return err
	}
	var prior []byte
	if pre.OperatorNote != nil {
		path := "evidence/" + string(pre.OperatorNote.Head)
		if prior, err = requiredRead(n.r.Source, path, ticket.MaxOperatorNoteBytes); err != nil {
			return err
		}
		if err = snapshot.ContentPath(path, prior); err != nil {
			return err
		}
	}
	post, derived, err := mutation.ReplayOperatorNote(n.r.QueueID, policy, pre, prior, mutation.Binding{ID: rc.ActorID, Role: rc.ActorRole}, event.Request, rc.RecordedAt)
	if err != nil {
		return noteForked(t.path, "transition does not replay from audited pre-state: %v", err)
	}
	if !bytes.Equal(post, t.raw) || !bytes.Equal(derived, n.events[0]) || posts[0] == nil {
		return noteForked(t.path, "posted ticket or event differs from the replayed transition")
	}
	return nil
}

// priorPost re-reads the exact retained afterimage a canonical latest names
// from the receipt that posted it; its digest binds the bytes.
func (r Reader) priorPost(prior latest, path string) ([]byte, error) {
	if prior.seq == "" || prior.digest == nil {
		return nil, noteForked(path, "no audited prior afterimage")
	}
	name, err := snapshot.ReceiptName(prior.seq.Uint64())
	if err != nil {
		return nil, err
	}
	raw, err := requiredRead(r.Source, "receipts/"+name, wire.MaxReceiptFileBytes)
	if err != nil {
		return nil, err
	}
	rc, err := snapshot.DecodeReceipt(raw)
	if err != nil {
		return nil, err
	}
	for _, p := range rc.Post {
		if p.Path == path && equalDigest(p.Sha256, prior.digest) {
			return r.postBytes(p)
		}
	}
	return nil, noteForked(path, "prior receipt does not post the audited afterimage")
}
