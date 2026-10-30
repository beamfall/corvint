package workqueue

import (
	"slices"
	"sort"
)

// ValidateDetailRequests checks the policy-dependent request derivation in §4.2.
// Snapshot parsing and ValidateSnapshot establish ticket order and identities;
// this helper neither normalizes supplied requests nor validates returned details.
func ValidateDetailRequests(snapshot *Snapshot, policy *Policy) ValidationResult {
	result := validation{unknowns: map[string]struct{}{}}
	if snapshot == nil || policy == nil {
		result.unknown(UnknownAdapterInvalid)
		return result.finish()
	}
	limit, err := ParseCount(policy.DetailLimit)
	if err != nil || limit > 512 {
		result.unknown(UnknownAdapterInvalid)
		return result.finish()
	}
	requested := make([]string, 0, int(limit))
	for _, ticket := range snapshot.Tickets {
		if Count(len(requested)) == limit {
			break
		}
		if ticket.Lifecycle != "READY" {
			continue
		}
		if ticket.DetailPayloadSHA256 == nil {
			continue
		}
		requested = append(requested, ticket.TicketVersionID)
	}
	// IDs have one fixed ASCII prefix followed by lowercase hex, so lexical order
	// is also their canonical JSON byte order. Select before sorting.
	sort.Strings(requested)
	result.conflicted = !slices.Equal(requested, snapshot.DetailRequestTicketVersionIDs)
	return result.finish()
}
