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
	// others counts posts that are neither a ticket, the policy, a note
	// event nor a request-index afterimage.
	others int
}

// noteState is what the walk has observed so far: each walked ticket's note
// reference and whether the policy was posted by a walked receipt. A complete
// audit walks every path from genesis; a checkpoint-resumed walk starts
// empty and reads a receipt before its checkpoint only for a note
// transition's pre-state, bound by the checkpoint entry digest (ON-V0-013).
type noteState struct {
	refs         map[string]*ticket.OperatorNoteReference
	policyWalked bool
}

type notePost struct {
	path  string
	index int
	prior latest
	raw   []byte
	// rec is raw as validateRecord decoded it, when it did (V1-1701).
	rec *ticket.Record
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
	case strings.HasPrefix(p.Path, "requests/"):
	default:
		n.others++
	}
}

// decoded records validateRecord's decoding of post j's ticket bytes.
func (n *noteAudit) decoded(j int, rec *ticket.Record) {
	if rec == nil {
		return
	}
	for i := range n.tickets {
		if n.tickets[i].index == j {
			n.tickets[i].rec = rec
		}
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
	for _, t := range n.tickets {
		if err := n.st.sealedNote(t.path, t.prior); err != nil {
			return err
		}
	}
	posts := make([]*ticket.OperatorNoteReference, len(n.tickets))
	for i, t := range n.tickets {
		if t.raw == nil {
			continue
		}
		rec := t.rec
		if rec == nil {
			rec = target
		}
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
				if t.raw != nil {
					if err := n.prefixReference(t, posts[i]); err != nil {
						return err
					}
				}
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
	// A note receipt posts exactly its event, the target ticket and its one
	// bound request-index afterimage (step already requires that one), and
	// nothing else: no other ticket, no policy and no other state path.
	if len(n.tickets) != 1 || n.tickets[0].raw == nil || n.tickets[0].path != "intent/tickets/"+rc.TicketID.Local+".json" || n.policy || n.others != 0 || len(rc.Post) != 3 {
		return noteForked("receipt", "event requires exactly the event, target ticket and request posts")
	}
	t := n.tickets[0]
	if t.prior.seq == "" || !equalDigest(rc.Pre[t.index].Sha256, t.prior.digest) {
		return noteForked(t.path, "pre-ticket is not the audited canonical afterimage")
	}
	_, walked := n.st.notes.refs[t.path]
	if n.st.resumed == 0 && (!walked || !n.st.notes.policyWalked) {
		return errCheckpoint(t.path, "note transition pre-state precedes the checkpoint")
	}
	event, err := ticket.DecodeOperatorNoteEvent(n.events[0])
	if err != nil {
		return err
	}
	if wire.Sum(event.Request) != req.Entry.MutationSha256 || event.RequestSha256 != req.Entry.MutationSha256 {
		return noteForked("receipt", "event request differs from the retained request digest")
	}
	env, err := mutation.Decode(event.Request)
	if err != nil {
		return noteForked("receipt", "retained request does not decode: %v", err)
	}
	if env.RequestID != *rc.RequestID || env.RequestID != req.Entry.RequestID || env.QueueID != n.r.QueueID {
		return noteForked("receipt", "retained request identity differs from the receipt and request index")
	}
	preRaw, err := n.prePost(t.prior, t.path)
	if err != nil {
		return err
	}
	pre, err := ticket.Decode(preRaw)
	if err != nil {
		return err
	}
	// A writer resume binds a pre-ticket last posted before its checkpoint
	// to the note reference the checkpoint carries too (ON-V0-012).
	if w := n.st.elided; w != nil && !walked && !w.sameNote(t.path, pre.OperatorNote) {
		return errCheckpoint(t.path, "note reference differs from the writer checkpoint")
	}
	policyLatest, _, err := n.st.get("intent/policy.json")
	if err != nil {
		return err
	}
	policyRaw, err := n.prePost(policyLatest, "intent/policy.json")
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

// prefixReference binds the note reference of a ticket post whose prior
// afterimage a resumed walk did not walk: it was last posted at or before the
// checkpoint. A writer resume compares it with the reference its checkpoint
// carries (CAL-V0-116); a read resume reads that prior afterimage from the
// one receipt that posted it, bound to the checkpoint entry digest
// (ON-V0-013). Any difference declines the checkpoint, so the complete audit
// classifies the change; it never passes unverified. A complete walk, or a
// prior that is a deletion, has no earlier reference to bind.
func (n *noteAudit) prefixReference(t notePost, post *ticket.OperatorNoteReference) error {
	if n.st.resumed == 0 || t.prior.digest == nil || t.prior.seq.Uint64() > n.st.resumed {
		return nil
	}
	if w := n.st.elided; w != nil && t.prior.seq.Uint64() <= w.Seq.Uint64() {
		if !w.sameNote(t.path, post) {
			return errCheckpoint(t.path, "note reference differs from the writer checkpoint")
		}
		return nil
	}
	raw, err := n.prePost(t.prior, t.path)
	if err != nil {
		return err
	}
	pre, err := ticket.Decode(raw)
	if err != nil {
		return errCheckpoint(t.path, "checkpoint pre-state does not bind: "+err.Error())
	}
	if !sameReference(pre.OperatorNote, post) {
		return errCheckpoint(t.path, "note reference differs from the checkpoint pre-state")
	}
	return nil
}

// prePost is priorPost for a note transition's pre-state. A resumed walk
// reads a pre-state last posted at or before its checkpoint from that one
// prefix receipt, bound by the checkpoint entry's digest rather than by the
// chain it does not walk (ON-V0-013); any failure there declines the
// checkpoint, so the complete audit classifies the prefix.
func (n *noteAudit) prePost(prior latest, path string) ([]byte, error) {
	raw, err := n.st.priorPost(n.r, prior, path)
	if err != nil && n.st.resumed != 0 && prior.seq != "" && prior.seq.Uint64() <= n.st.resumed {
		return nil, errCheckpoint(path, "checkpoint pre-state does not bind: "+err.Error())
	}
	return raw, err
}

// priorPost re-reads the exact retained afterimage a canonical latest names
// from the receipt that posted it; its digest binds the bytes. The receipt
// must also hold the chain position the complete audit checks for it: its
// own sequence, a previous digest exactly when it is not receipt 1, and a
// head generation no later than the walk's (ON-V0-014). A walk resumed after
// checkpoint receipt resumed (digest tip), which did not walk the prefix,
// also binds the receipt's chain position by value: the checkpoint receipt
// itself by tip, and an earlier one by its previous digest naming the
// receipt before it (ON-V0-015).
func (r Reader) priorPost(prior latest, path string, generation, resumed uint64, tip wire.Digest) ([]byte, error) {
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
	if rc.Seq != prior.seq || (rc.Prev == nil) != (prior.seq.Uint64() == 1) || rc.HeadGeneration.Uint64() > generation {
		return nil, noteForked(path, "prior receipt sequence/prev/generation differs from its chain position")
	}
	switch seq := prior.seq.Uint64(); {
	case resumed == 0 || seq > resumed:
	case seq == resumed:
		if wire.Sum(raw) != tip {
			return nil, noteForked(path, "prior receipt is not the checkpoint receipt")
		}
	case rc.Prev != nil:
		if err := r.priorLink(path, seq, *rc.Prev); err != nil {
			return nil, err
		}
	}
	for _, p := range rc.Post {
		if p.Path == path && equalDigest(p.Sha256, prior.digest) {
			return r.postBytes(p)
		}
	}
	return nil, noteForked(path, "prior receipt does not post the audited afterimage")
}

// priorLink binds a prefix receipt's previous digest to the receipt before
// it: that receipt must hold sequence seq-1 and its chain digest (its bytes,
// or the digest an elided liveness receipt retains) must equal prev. One more
// bounded read; the complete audit walks this link itself (ON-V0-015).
func (r Reader) priorLink(path string, seq uint64, prev wire.Digest) error {
	name, err := snapshot.ReceiptName(seq - 1)
	if err != nil {
		return err
	}
	raw, err := requiredRead(r.Source, "receipts/"+name, wire.MaxReceiptFileBytes)
	if err != nil {
		return err
	}
	rc, digest, _, err := snapshot.DecodeRetainedReceipt(raw)
	if err != nil {
		return err
	}
	if rc.Seq.Uint64() != seq-1 || digest != prev {
		return noteForked(path, "prior receipt's previous digest does not name the receipt before it")
	}
	return nil
}
