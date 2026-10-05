package journal

import (
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// MutationAudit is one complete writer audit serving a mutation's request
// lookup, its inventory digests and its canonical intent records
// (CAL-V0-070). It lives for one writer-locked mutation; it is not a cache and
// grants nothing the three separate audits it replaces did not.
type MutationAudit struct {
	Found    bool
	Entry    mutation.IndexEntry
	TicketID string // original receipt target of a found request
	Identity Identity
	// Physical.Files is published only for a complete, settled audit on which
	// every projection agrees; otherwise it is nil and the inventory reads fresh.
	Physical PhysicalObservation
	result   *Result
	lim      limits
}

// AuditForMutation is RequestIndex.Lookup's audit, made once, that also
// retains the canonical queue, policy and every observed ticket and release,
// and the physical digests it read. Its error is exactly Lookup's. Refusals
// only a later Audit of that selection would raise (the selection budget and
// intent divergence) are deferred to Canonical, so a found request replays as
// before and the inventory still refuses first.
func (r Reader) AuditForMutation(requestID string) (*MutationAudit, error) {
	return r.auditForMutation(requestID, profileLimits)
}

func (r Reader) auditForMutation(requestID string, lim limits) (*MutationAudit, error) {
	m := &MutationAudit{lim: lim}
	if _, err := snapshot.RequestPath(requestID); err != nil {
		return m, err
	}
	r.observedIntent, r.physical = true, &m.Physical
	result, err := r.audit(nil, requestID, lim, false)
	if result != nil {
		m.Identity = result.Identity
	}
	if err != nil {
		return m, err
	}
	m.result = result
	if result.request != nil {
		m.Found, m.Entry, m.TicketID = true, result.request.Entry, result.requestTicket
	}
	return m, nil
}

// observedSelection is the path set store.Mutate derives from its inventory:
// queue, policy and every regular ticket and release file. The capture has
// already refused every other child of those directories.
func observedSelection(o *observation) map[string]bool {
	selected := map[string]bool{"intent/queue.json": true, "intent/policy.json": true}
	for p, info := range o.files {
		if !info.IsDir() && (strings.HasPrefix(p, "intent/tickets/") || strings.HasPrefix(p, "intent/releases/")) {
			selected[p] = true
		}
	}
	return selected
}

// Canonical returns what Audit(paths...) returns for the same observation:
// its first refusal, or its Result. ok is false unless paths name exactly the
// retained selection and pass Audit's own path checks; the caller then runs
// Audit itself, which raises any path refusal first as before.
func (m *MutationAudit) Canonical(paths ...string) (*Result, bool, error) {
	if m == nil || m.result == nil || m.Found {
		return nil, false, nil
	}
	named := make(map[string]bool, len(paths))
	for _, p := range paths {
		if _, err := snapshot.PostBound(p); err != nil || !m.result.selection[p] {
			return nil, false, nil
		}
		if len(named) >= m.lim.scan+intent.MaxIntentRootEntries+wire.MaxTicketsPerQueue+wire.MaxReleasesPerQueue {
			return nil, false, nil
		}
		named[p] = true
	}
	if len(named) != len(m.result.selection) {
		return nil, false, nil
	}
	if m.result.selectErr != nil {
		return nil, true, m.result.selectErr
	}
	if m.result.intentErr != nil {
		return nil, true, m.result.intentErr
	}
	out := *m.result
	out.ProjectionAgreement = "AGREES"
	out.request, out.requestTicket, out.selection = nil, "", nil
	return &out, true, nil
}
