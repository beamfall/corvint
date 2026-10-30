package transaction

import (
	"fmt"
	"sort"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// ERG-V0-009 read bounds: at most 16 gates per ticket and an anchored history
// page of at most 50 events (default 20) and 1 MiB of event bytes.
const (
	MaxExternalReviewGates       = 16
	ExternalReviewHistoryDefault = 20
	ExternalReviewHistoryMax     = 50
	ExternalReviewHistoryBytes   = 1 << 20
)

// ExternalReviewBlob returns retained event bytes by content digest; false
// means the bytes are absent. The caller supplies audited journal evidence.
type ExternalReviewBlob func(wire.Digest) ([]byte, bool)

// ExternalReviewGates is the pure workState adapter over a ticket's gate map
// (ERG-V0-009): each view comes from the bound head event and the explicit
// current binding of the same gate. A missing head, a missing or mismatched
// binding, or a head event for another ticket or gate gives UNKNOWN, never an
// actionable verdict.
func ExternalReviewGates(ticketID string, refs map[string]snapshot.ExternalReviewRef, blob ExternalReviewBlob, current map[string]*ExternalReviewBinding) (map[string]ExternalReviewView, error) {
	if len(refs) > MaxExternalReviewGates {
		return nil, fmt.Errorf("external review: more than %d gates", MaxExternalReviewGates)
	}
	out := make(map[string]ExternalReviewView, len(refs))
	for gate, ref := range refs {
		if _, err := wire.ParseLabel("/externalReviews", gate); err != nil {
			return nil, err
		}
		raw, _ := blob(ref.Head)
		if e, err := snapshot.CanonicalExternalReviewEvent(raw); err != nil || e.Request.TicketID != ticketID || e.Request.GateID != gate {
			raw = nil
		}
		b := current[gate]
		if b != nil && (b.GateID != gate || b.TicketID != ticketID) {
			b = nil
		}
		r := ref
		out[gate] = ExternalReviewCurrent(&r, raw, b)
	}
	return out, nil
}

// ExternalReviewCompletionOffer is the ERG-V0-011 read-only completion offer
// predicate over one ticket's gate views (ExternalReviewGates). The required
// set is every gate the policy declares plus every gate the ticket references;
// each must read CURRENT with verdict PASS. It returns their head digests,
// sorted, as suggested complete-manual evidence, or nil when the policy
// declares no gate or any required gate is missing, STALE, UNKNOWN, RETURN or
// a resubmission awaiting review. It writes nothing and satisfies nothing: an
// external PASS stays routing evidence only (ERG-V0-007).
func ExternalReviewCompletionOffer(policy *intent.Policy, views map[string]ExternalReviewView) []wire.Digest {
	if policy == nil || len(policy.ExternalReviews) == 0 {
		return nil
	}
	for _, d := range policy.ExternalReviews {
		if _, ok := views[d.GateID]; !ok {
			return nil
		}
	}
	heads := make([]wire.Digest, 0, len(views))
	for _, v := range views {
		if v.Status != "CURRENT" || v.Verdict == nil || *v.Verdict != "PASS" || v.Resubmitted || v.EvidenceSha256 == nil {
			return nil
		}
		heads = append(heads, *v.EvidenceSha256)
	}
	sort.Slice(heads, func(i, j int) bool { return heads[i] < heads[j] })
	return heads
}

// ExternalReviewPage is one newest-first history page. Next names the event
// that starts the following page, or is nil after the first event.
type ExternalReviewPage struct {
	Events [][]byte
	Next   *wire.Digest
}

// ExternalReviewHistory reads one gate's events newest first, anchored at the
// ticket's head reference. Every link must be canonical, name the same ticket
// and gate, follow the retained request's counters and, for a resubmission,
// link its prior RETURN. A cursor must be an event on the anchored chain; it
// is found by walking from the head, so a page reads at most 4096 events.
func ExternalReviewHistory(ref snapshot.ExternalReviewRef, blob ExternalReviewBlob, ticketID, gateID string, cursor *wire.Digest, limit int) (ExternalReviewPage, error) {
	fail := func(f string, a ...any) (ExternalReviewPage, error) {
		return ExternalReviewPage{}, fmt.Errorf("external review history: "+f, a...)
	}
	if limit == 0 {
		limit = ExternalReviewHistoryDefault
	}
	if limit < 0 || limit > ExternalReviewHistoryMax {
		return fail("page size must be 1..%d", ExternalReviewHistoryMax)
	}
	if _, err := ref.Encode(); err != nil {
		return fail("head reference: %v", err)
	}
	page := ExternalReviewPage{Events: [][]byte{}}
	at, revision, started, size := ref.Head, ref.Revision.Int(), cursor == nil, 0
	var newer *snapshot.ExternalReviewEvent
	for {
		raw, ok := blob(at)
		if !ok {
			return fail("event %s is absent", at)
		}
		e, err := snapshot.CanonicalExternalReviewEvent(raw)
		if err != nil || wire.Sum(raw) != at {
			return fail("event %s is not canonical or differs from its digest", at)
		}
		q := e.Request
		if q.TicketID != ticketID || q.GateID != gateID || e.EventRevision.Int() != revision {
			return fail("event %s is not revision %d of %s %s", at, revision, ticketID, gateID)
		}
		if newer == nil && e.ReviewGeneration != ref.Generation {
			return fail("head generation differs from its reference")
		}
		if newer != nil && (e.ReviewGeneration != newer.Request.ExpectedGeneration || (newer.Request.Action == "RESUBMIT" && (e.Request.Verdict == nil || *e.Request.Verdict != "RETURN" || *newer.Request.PriorReturn != at))) {
			return fail("event %s does not precede its successor", at)
		}
		if !started && at == *cursor {
			started = true
		}
		if started {
			if len(page.Events) == limit || size+len(raw) > ExternalReviewHistoryBytes {
				next := at
				page.Next = &next
				return page, nil
			}
			page.Events = append(page.Events, raw)
			size += len(raw)
		}
		if e.Previous == nil {
			if !started {
				return fail("cursor is not on the anchored chain")
			}
			return page, nil
		}
		newer, at, revision = e, *e.Previous, revision-1
	}
}
