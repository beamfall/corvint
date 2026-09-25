package intent

import (
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// ProfileImportMap is the import identity map profile (§3.1).
const ProfileImportMap = "taskman-import-map/0"

// ImportEntry is one import-map entry.
type ImportEntry struct {
	SourceQueueID        string
	SourceItemID         string
	TicketID             wire.TicketID
	SourceRevisionSha256 *wire.Digest
	AppliedSeq           wire.Size
}

// ImportMap is a validated taskman-import-map/0.
type ImportMap struct {
	QueueID wire.QueueID
	Entries []ImportEntry
}

// DecodeImportMap parses and validates import-map.json (≤8 MiB, ≤10,000
// entries, sorted, no duplicate (sourceQueueId, sourceItemId)).
func DecodeImportMap(data []byte) (*ImportMap, error) {
	if len(data) > wire.MaxImportMapBytes {
		return nil, wire.Errorf(wire.CodeLimitExceeded, "/", "import-map.json larger than %d bytes", wire.MaxImportMapBytes)
	}
	v, err := wire.Parse(data)
	if err != nil {
		return nil, err
	}
	r := wire.NewReader(v, "/")
	r.Closed("profile", "queueId", "entries")
	if err := r.Err(); err != nil {
		return nil, err
	}
	if err := wire.CheckProfile("/profile", r.Field("profile").String(), ProfileImportMap); err != nil {
		return nil, err
	}
	m := &ImportMap{}
	m.QueueID = r.Field("queueId").QueueID()
	seen := map[string]bool{}
	for _, e := range r.Field("entries").Array(wire.MaxImportMapEntries, false) {
		e.Closed("sourceQueueId", "sourceItemId", "ticketId", "sourceRevisionSha256", "appliedSeq")
		ie := ImportEntry{}
		ie.SourceQueueID = e.Field("sourceQueueId").Identifier()
		ie.SourceItemID = e.Field("sourceItemId").Identifier()
		ie.TicketID = e.Field("ticketId").TicketID()
		ie.SourceRevisionSha256 = e.Field("sourceRevisionSha256").DigestOrNull()
		ie.AppliedSeq = e.Field("appliedSeq").Size()
		key := ie.SourceQueueID + "\x00" + ie.SourceItemID
		if e.Err() == nil && seen[key] {
			e.Fail(wire.CodeDuplicateID, "duplicate (sourceQueueId, sourceItemId)")
		}
		seen[key] = true
		if e.Err() == nil && ie.TicketID.QueueID() != m.QueueID.Raw {
			e.Field("ticketId").Fail(wire.CodeMalformed, "ticket %s is outside queue %s", ie.TicketID.Raw, m.QueueID.Raw)
		}
		m.Entries = append(m.Entries, ie)
	}
	if err := r.Err(); err != nil {
		return nil, err
	}
	return m, nil
}
