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
	// WriterCheckpoint, when set, lets a lookup be served by the writer
	// route before it pays the complete audit (LGL-V0-811): it returns the
	// retained writer checkpoint bound to its CAL-V0-117 token, or nil, and
	// is called at most once per lookup. The route is the one a writer's
	// own replay uses (CAL-V0-116, CAL-V0-190); any decline falls back to
	// the complete audit, which derives the entry, its absence or the
	// refusal itself.
	WriterCheckpoint func() *WriterCheckpoint
}

func (i *RequestIndex) Lookup(id string) (mutation.IndexEntry, bool, error) {
	i.Identity = Identity{}
	i.TicketID = ""
	i.IntentProjectionAgreement = "NOT_OBSERVED"
	if _, err := snapshot.RequestPath(id); err != nil {
		return mutation.IndexEntry{}, false, err
	}
	if entry, found, served, err := i.writerLookup(id); served {
		return entry, found, err
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

// writerLookup serves Lookup from the writer checkpoint when the writer
// route can (LGL-V0-811). served is false whenever the route declines; only
// a failed close of its reads is terminal, as it is for the writer's replay.
func (i *RequestIndex) writerLookup(id string) (entry mutation.IndexEntry, found, served bool, err error) {
	if i.WriterCheckpoint == nil || !requestIndexWriterRoute {
		return entry, false, false, nil
	}
	wc := i.WriterCheckpoint()
	if wc == nil {
		return entry, false, false, nil
	}
	res, _, err := i.Reader.AuditForWriter(wc, id, false)
	if err != nil {
		return entry, false, false, nil
	}
	found, entry, ticketID, cleanup, err := i.Reader.WriterReplay(res, id)
	if cleanup != nil {
		i.Identity = res.Identity
		return mutation.IndexEntry{}, false, true, err
	}
	if err != nil {
		return mutation.IndexEntry{}, false, false, nil
	}
	i.Identity = res.Identity
	if found {
		i.TicketID = ticketID
	}
	return entry, found, true, nil
}

// requestIndexWriterRoute lets a test compare the writer route with the
// complete audit it falls back to.
var requestIndexWriterRoute = true
