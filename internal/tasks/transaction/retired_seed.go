package transaction

import (
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// Retired-prefix seeding of the material binding folds (JRT-V0-008).
//
// A sealed store retires receipts 1..S-1, so its folds start after the
// retained anchor S. Each fold is seeded with the anchor afterimages the seal
// carries, posts naming the latest bytes of every seeded path, exactly the
// state the fold holds after folding 1..S for every input a later receipt
// can name: ticket references, attempts, reservations and the policy. What a
// fold retains only from a receipt itself (review submissions, artifact
// links, claim admissions) is not seeded; a later receipt that names one
// refuses as the unsealed fold refuses an unknown one.

// seedPolicy decodes the seeded policy as the folds do on a policy post.
func seedPolicy(posts []snapshot.PostEntry, blob ExternalReviewBlob) (*intent.Policy, string) {
	for _, p := range posts {
		if p.Path != "intent/policy.json" {
			continue
		}
		raw, err := externalPostBytes(p, blob)
		if err != nil {
			return nil, "the retained policy is unreadable: " + err.Error()
		}
		policy, err := intent.DecodePolicy(raw)
		if err != nil {
			return nil, "the retained policy is unreadable: " + err.Error()
		}
		return policy, ""
	}
	return nil, "no policy was posted"
}

func seedFail(p string, err error) error {
	return wire.Errorf(wire.CodeJournalForked, p, "retired prefix seed: %v", err)
}

func seedTickets(posts []snapshot.PostEntry, blob ExternalReviewBlob, each func(snapshot.PostEntry, []byte, *ticket.Record) error) error {
	for _, p := range posts {
		if !strings.HasPrefix(p.Path, "intent/tickets/") {
			continue
		}
		raw, err := externalPostBytes(p, blob)
		if err != nil {
			return seedFail(p.Path, err)
		}
		rec, err := ticket.Decode(raw)
		if err != nil {
			return seedFail(p.Path, err)
		}
		if err := each(p, raw, rec); err != nil {
			return err
		}
	}
	return nil
}

// Seed starts the review fold at a retired prefix's anchor.
func (a *ExternalReviewReceiptAudit) Seed(posts []snapshot.PostEntry, blob ExternalReviewBlob) error {
	a.refs = map[string]map[string]ticket.ExternalReviewRef{}
	a.receipts = map[uint64]externalSubmission{}
	a.latest = map[string]map[string]uint64{}
	a.artifacts = map[ExternalArtifact]bool{}
	a.policy, a.policyErr = seedPolicy(posts, blob)
	return seedTickets(posts, blob, func(p snapshot.PostEntry, _ []byte, rec *ticket.Record) error {
		a.refs[p.Path] = rec.ExternalReviews
		return nil
	})
}

// Seed starts the escalation fold at a retired prefix's anchor. The anchor
// is a settled terminal store, so no admission it retired can be current.
func (a *EscalationReceiptAudit) Seed(posts []snapshot.PostEntry, blob ExternalReviewBlob) error {
	a.tickets = map[string]escalationTicket{}
	a.admits = map[uint64]escalationAdmit{}
	a.attempts = map[string]escalationAttempt{}
	a.policy, a.policyErr = seedPolicy(posts, blob)
	if err := seedTickets(posts, blob, func(p snapshot.PostEntry, _ []byte, rec *ticket.Record) error {
		var refs []byte
		if rec.Escalations != nil {
			var err error
			if refs, err = ticket.EncodeEscalationRefs(*rec.Escalations); err != nil {
				return seedFail(p.Path, err)
			}
		}
		a.tickets[p.Path] = escalationTicket{post: p, refs: refs}
		return nil
	}); err != nil {
		return err
	}
	// admit folds the attempt and reservation posts; a SEED receipt admits
	// nothing.
	return a.admit(&snapshot.Receipt{Kind: "SEED", Post: posts}, "", blob, nil)
}

// Seed starts the obligation fold at a retired prefix's anchor.
func (a *ObligationReceiptAudit) Seed(posts []snapshot.PostEntry, blob ExternalReviewBlob) error {
	a.tickets = map[string]obligationTicket{}
	a.attempts = map[string]*snapshot.Attempt{}
	a.policy, a.policyErr = seedPolicy(posts, blob)
	if err := seedTickets(posts, blob, func(p snapshot.PostEntry, _ []byte, rec *ticket.Record) error {
		cur := obligationTicket{post: p}
		if rec.ObligationsRef != nil {
			cur.ref = wire.EncodeFile(rec.ObligationsRef.Value())
			if rec.Status != ticket.StatusArchived && rec.Source.Kind == "NATIVE" {
				cur.prefix = rec.ObligationsRef.Prefix
			}
		}
		a.tickets[p.Path] = cur
		return nil
	}); err != nil {
		return err
	}
	for id, at := range seedAttempts(posts, blob) {
		a.attempts[id] = at
	}
	return nil
}

// Seed starts the gate reuse fold at a retired prefix's anchor. A reused
// result the anchor attempts name was accepted by the fold that retired it.
func (a *GateReuseReceiptAudit) Seed(posts []snapshot.PostEntry, blob ExternalReviewBlob) error {
	a.attempts = seedAttempts(posts, blob)
	a.accepted = map[string]bool{}
	a.policy, a.policyErr = seedPolicy(posts, blob)
	for _, p := range posts {
		if p.Path == "intent/queue.json" {
			a.branch = postedBranch(p, blob)
		}
	}
	for _, at := range a.attempts {
		for _, d := range at.GateResults {
			raw, ok := blob(wire.Digest(d))
			if !ok || wire.Sum(raw) != wire.Digest(d) {
				continue
			}
			if g, err := snapshot.DecodeGateResult(raw); err == nil && g.ReusedFrom != nil {
				a.accepted[d] = true
			}
		}
	}
	return nil
}

// seedAttempts decodes the seeded attempts as the folds do: an attempt post
// whose bytes are unreadable or do not decode is not folded.
func seedAttempts(posts []snapshot.PostEntry, blob ExternalReviewBlob) map[string]*snapshot.Attempt {
	out := map[string]*snapshot.Attempt{}
	for _, p := range posts {
		if !strings.HasPrefix(p.Path, "attempts/") {
			continue
		}
		raw, err := externalPostBytes(p, blob)
		if err != nil {
			continue
		}
		if at, err := snapshot.DecodeAttempt(raw); err == nil {
			out[strings.TrimSuffix(strings.TrimPrefix(p.Path, "attempts/"), ".json")] = at
		}
	}
	return out
}
