package journal

import (
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
)

// RequestIndex is a streamed adapter. Construction grants no cached validity:
// each lookup re-audits the bounded native snapshot and all private projections.
// Stable intent divergence is permitted for request-history lookup only;
// intent bytes and inventory are still bounded and rechecked for movement.
// Reader is an explicit read input; there is no trusted actor binding here.
// Use serially: Identity is the observation from the most recent lookup.
type RequestIndex struct {
	Reader                    Reader
	Identity                  Identity
	TicketID                  string // original receipt target from the last successful lookup
	IntentProjectionAgreement string
}

func (i *RequestIndex) Lookup(id string) (mutation.IndexEntry, bool, error) {
	i.Identity = Identity{}
	i.TicketID = ""
	i.IntentProjectionAgreement = "NOT_OBSERVED"
	if _, err := snapshot.RequestPath(id); err != nil {
		return mutation.IndexEntry{}, false, err
	}
	result, err := i.Reader.audit(nil, id, profileLimits, false)
	if result != nil {
		i.Identity = result.Identity
	}
	if err != nil {
		return mutation.IndexEntry{}, false, err
	}
	if result.request == nil {
		return mutation.IndexEntry{}, false, nil
	}
	i.TicketID = result.requestTicket
	return result.request.Entry, true, nil
}
