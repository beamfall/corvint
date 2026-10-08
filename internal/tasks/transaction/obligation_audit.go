package transaction

import (
	"bytes"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/obligation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// ObligationReceiptAudit binds every obligation-ledger reference change to
// the receipt that records it (TOL-V0-013). Receipts are folded in sequence
// order; a ticket post whose `obligations` reference differs from the folded
// predecessor must be the one ticket of a completed MUTATION receipt whose
// request entry binds the posted event's retained request, and that receipt
// posts nothing but its request entry, that ticket and the event. The event
// and the ticket must then be byte for byte what the writer's own reducer
// computes from the audited predecessor, its retained chain, the policy the
// history had posted, the attempt ledger as folded at this receipt's time
// and this receipt's actor. A first seed's prefix must not be held by
// another folded non-archived native ticket. A Playwright report is never
// retained, so the audit replays the retained witness claim without it and
// its result does not depend on the report still existing; the writer's
// report recomputation (TOL-V0-010..011) is checked at write time only. The
// TOL-V0-012 source presence of every retained match is re-checked through
// Source at the event's commit when Source is set. A dropped reference is
// refused. Apart from Source it is pure: blob returns retained evidence
// bytes by digest, and its memory is the latest ticket, attempt and policy
// posts.
type ObligationReceiptAudit struct {
	// Source reads paths at commit: commit is "" when the repository holds
	// no such commit, present names every blob and content its bytes.
	Source func(commit string, paths []string) (resolved string, present map[string]bool, content map[string][]byte, err error)

	tickets   map[string]obligationTicket
	attempts  map[string]*snapshot.Attempt
	policy    *intent.Policy
	policyErr string
}

type obligationTicket struct {
	post   snapshot.PostEntry
	ref    []byte // canonical reference, nil when the record has none
	prefix string // the ledger prefix of a non-archived native record
}

func (a *ObligationReceiptAudit) fail(rc *snapshot.Receipt, f string, args ...any) error {
	return wire.Errorf(wire.CodeJournalForked, "receipts/"+string(rc.Seq), "obligation binding: "+f, args...)
}

// Step folds one validated receipt; sum is unused but keeps the audit
// signature of the other material binding folds.
func (a *ObligationReceiptAudit) Step(rc *snapshot.Receipt, _ wire.Digest, blob ExternalReviewBlob) error {
	if a.tickets == nil {
		a.tickets = map[string]obligationTicket{}
		a.attempts = map[string]*snapshot.Attempt{}
		a.policyErr = "no policy was posted"
	}
	tickets := 0
	for _, p := range rc.Post {
		if strings.HasPrefix(p.Path, "intent/tickets/") {
			tickets++
		}
	}
	for _, p := range rc.Post {
		if !strings.HasPrefix(p.Path, "intent/tickets/") {
			continue
		}
		prior, known := a.tickets[p.Path]
		if p.Sha256 == nil {
			if prior.ref != nil {
				return a.fail(rc, "%s dropped its obligation ledger reference", p.Path)
			}
			delete(a.tickets, p.Path)
			continue
		}
		raw, err := externalPostBytes(p, blob)
		if err != nil {
			return a.fail(rc, "%s post: %v", p.Path, err)
		}
		rec, err := ticket.Decode(raw)
		if err != nil {
			return a.fail(rc, "%s post is not a ticket record: %v", p.Path, err)
		}
		cur := obligationTicket{post: p}
		if rec.ObligationsRef != nil {
			cur.ref = wire.EncodeFile(rec.ObligationsRef.Value())
			if rec.Status != ticket.StatusArchived && rec.Source.Kind == "NATIVE" {
				cur.prefix = rec.ObligationsRef.Prefix
			}
		}
		if !bytes.Equal(cur.ref, prior.ref) {
			if cur.ref == nil {
				return a.fail(rc, "%s dropped its obligation ledger reference", p.Path)
			}
			if !known || rc.Kind != "MUTATION" || rc.Outcome != mutation.OutcomeCompleted || rc.RequestID == nil || tickets != 1 {
				return a.fail(rc, "%s changed its obligation ledger outside one OBLIGATIONS_* mutation", p.Path)
			}
			if err := a.bind(rc, prior, p.Path, rec, raw, blob); err != nil {
				return err
			}
		}
		a.tickets[p.Path] = cur
	}
	for _, p := range rc.Post {
		switch {
		case strings.HasPrefix(p.Path, "attempts/"):
			id := strings.TrimSuffix(strings.TrimPrefix(p.Path, "attempts/"), ".json")
			delete(a.attempts, id)
			if p.Sha256 == nil {
				continue
			}
			raw, err := externalPostBytes(p, blob)
			if err != nil {
				return a.fail(rc, "attempt post %s: %v", p.Path, err)
			}
			if at, err := snapshot.DecodeAttempt(raw); err == nil {
				a.attempts[id] = at
			}
		case p.Path == "intent/policy.json":
			a.policy, a.policyErr = nil, "the retained policy was removed"
			if p.Sha256 == nil {
				continue
			}
			raw, err := externalPostBytes(p, blob)
			if err == nil {
				a.policy, err = intent.DecodePolicy(raw)
			}
			if err != nil {
				a.policy, a.policyErr = nil, "the retained policy is unreadable: "+err.Error()
			}
		}
	}
	return nil
}

// bind replays one obligation write: prior is the folded predecessor of
// rec's path and raw is rec's posted bytes.
func (a *ObligationReceiptAudit) bind(rc *snapshot.Receipt, prior obligationTicket, path string, rec *ticket.Record, raw []byte, blob ExternalReviewBlob) error {
	preRaw, err := externalPostBytes(prior.post, blob)
	if err != nil {
		return a.fail(rc, "preceding ticket post: %v", err)
	}
	pre, err := ticket.Decode(preRaw)
	if err != nil {
		return a.fail(rc, "preceding ticket post is not a ticket record: %v", err)
	}
	head := rec.ObligationsRef.Head
	eventPath := "evidence/" + string(head)
	var event []byte
	for _, p := range rc.Post {
		if p.Path != eventPath || p.Sha256 == nil || *p.Sha256 != head {
			continue
		}
		b, ok := blob(head)
		if !ok || wire.Sum(b) != head {
			return a.fail(rc, "event %s is absent or differs from its digest", head)
		}
		event = b
	}
	if event == nil {
		return a.fail(rc, "the write posts no ledger event at its new head")
	}
	ev, err := ticket.DecodeObligationEvent(event)
	if err != nil {
		return a.fail(rc, "posted event: %v", err)
	}
	q := ev.Decoded
	queue := rec.TicketID.QueueID()
	if q.RequestID != *rc.RequestID || q.ActorID != rc.ActorID || q.ActorRole != rc.ActorRole || q.QueueID.Raw != queue || q.TicketID.Raw != rec.TicketID.Raw {
		return a.fail(rc, "the event's request does not name this receipt's request, actor or ticket")
	}
	if err := a.requestEntry(rc, q.RequestID, ev.RequestSha256, blob); err != nil {
		return err
	}
	requestPath, _ := snapshot.RequestPath(q.RequestID)
	for _, p := range rc.Post {
		if p.Path != path && p.Path != requestPath && p.Path != eventPath {
			return a.fail(rc, "the write posts %s", p.Path)
		}
	}
	if a.policy == nil {
		return a.fail(rc, "the grant cannot be recovered: %s", a.policyErr)
	}
	if pre.ObligationsRef == nil {
		for other, t := range a.tickets {
			if other != path && t.prefix != "" && t.prefix == rec.ObligationsRef.Prefix {
				return a.fail(rc, "the seeded prefix %s is already declared by %s", t.prefix, other)
			}
		}
	}
	events, err := ticket.ObligationChainEvents(func(d wire.Digest) ([]byte, error) {
		b, _ := blob(d)
		return b, nil
	}, pre.ObligationsRef)
	if err != nil {
		return a.fail(rc, "preceding ledger chain: %v", err)
	}
	post, replayed, err := mutation.ReplayObligation(mustQueue(queue), a.policy, []*ticket.Record{pre}, events,
		attemptLedgerAt(a.attempts, rc.RecordedAt), mutation.Binding{ID: rc.ActorID, Role: rc.ActorRole}, ev.Request, rc.RecordedAt)
	if err != nil {
		return a.fail(rc, "the write does not replay: %v", err)
	}
	if !bytes.Equal(replayed, event) {
		return a.fail(rc, "the posted event is not the replayed event")
	}
	if !bytes.Equal(post, raw) {
		return a.fail(rc, "the posted ticket is not the replayed record")
	}
	return a.sourcePresence(rc, q)
}

// sourcePresence re-checks TOL-V0-012 for a report witness: every retained
// match's path is a blob at the event's commit that holds the id as a whole
// token. A DECLARED witness checks shape, commit and grant only.
func (a *ObligationReceiptAudit) sourcePresence(rc *snapshot.Receipt, q *ticket.ObligationRequest) error {
	if a.Source == nil || q.Operation != ticket.OpObligationsWitness {
		return nil
	}
	w := q.Witness
	if w == nil || w.Source != ticket.ObligationSourceReport {
		return nil
	}
	seen := map[string]bool{}
	var paths []string
	for _, c := range w.Credits {
		if len(c.Matches) == 0 {
			return a.fail(rc, "credited id %s has no match", c.ID)
		}
		for _, m := range c.Matches {
			if !seen[m.Path] {
				seen[m.Path] = true
				paths = append(paths, m.Path)
			}
		}
	}
	commit, present, content, err := a.Source(w.Commit, paths)
	if err != nil {
		return a.fail(rc, "source presence at %s is unobservable: %v", w.Commit, err)
	}
	if commit == "" {
		return a.fail(rc, "the repository holds no commit %s", w.Commit)
	}
	for _, c := range w.Credits {
		for _, m := range c.Matches {
			if !present[m.Path] || !obligation.ContainsID(content[m.Path], c.ID) {
				return a.fail(rc, "credited id %s is not in %s at %s", c.ID, m.Path, w.Commit)
			}
		}
	}
	return nil
}

// requestEntry requires the receipt's request entry to bind d at its own
// sequence.
func (a *ObligationReceiptAudit) requestEntry(rc *snapshot.Receipt, id string, d wire.Digest, blob ExternalReviewBlob) error {
	path, err := snapshot.RequestPath(id)
	if err != nil {
		return a.fail(rc, "request path: %v", err)
	}
	for _, p := range rc.Post {
		if p.Path != path || p.Sha256 == nil {
			continue
		}
		raw, err := externalPostBytes(p, blob)
		if err != nil {
			return a.fail(rc, "request entry: %v", err)
		}
		entry, err := snapshot.DecodeRequest(raw)
		if err != nil || entry.Seq != rc.Seq || entry.Entry.MutationSha256 != d {
			return a.fail(rc, "the request entry does not bind the event's request")
		}
		return nil
	}
	return a.fail(rc, "the write posts no request entry")
}
