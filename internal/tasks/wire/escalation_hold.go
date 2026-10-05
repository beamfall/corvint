package wire

import "sort"

// EscalationHoldEntry is the projection of one `escalations` reference entry
// that the ESCALATION_PENDING derived hold reads (ESC-V0-006). The native
// ticket codec and Core's read-only planner each convert their own decoded
// entry into it, so both readers apply exactly one predicate (decision 0397).
type EscalationHoldEntry struct {
	RequestID          string
	AcceptanceRevision string
	Kind               string
	State              string
}

// escalationHoldKinds are the kinds that hold admission. Infrastructure is a
// typed observation of a failed session and never holds; any other kind is
// not a hold either, since the codecs refuse it before this runs.
var escalationHoldKinds = map[string]bool{"decision": true, "scope": true, "blocked": true}

// EscalationPending returns the request IDs that hold a ticket whose current
// acceptance revision is acceptanceRevision: OPEN decision, scope or blocked
// entries whose source acceptance revision equals it. Stale, answered,
// superseded and infrastructure entries never hold. Revisions are canonical
// decimal counts, so string equality is numeric equality. The IDs are sorted,
// deduplicated and bounded to EscalationMaxCurrentOpen; total is the number
// of distinct holding IDs before the bound. A nil result means no hold. The
// hold is derived on every read and is never written.
func EscalationPending(acceptanceRevision string, entries []EscalationHoldEntry) (ids []string, total int) {
	seen := map[string]bool{}
	for _, e := range entries {
		if e.State != "OPEN" || !escalationHoldKinds[e.Kind] || e.AcceptanceRevision != acceptanceRevision || seen[e.RequestID] {
			continue
		}
		seen[e.RequestID] = true
		ids = append(ids, e.RequestID)
	}
	sort.Strings(ids)
	total = len(ids)
	if total > EscalationMaxCurrentOpen {
		ids = ids[:EscalationMaxCurrentOpen]
	}
	return ids, total
}
