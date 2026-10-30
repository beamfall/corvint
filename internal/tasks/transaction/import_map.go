package transaction

import (
	"fmt"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// importMapPath is the journaled projection of a queue's import map.
const importMapPath = "intent/" + intent.ImportMapFile

// bindImportMap admits a non-fixture queue whose importMapSha256 names an
// import map (CTS-V0-014, CTS-V0-015). Genesis takes the map from the INIT
// request and journals it. A later operation decodes the map only when the
// caller supplied the bytes its journal audit selected; the journal audit
// binds those bytes to the canonical record and its projection, and the
// queue digest binds them here. A queue that names no map takes no map bytes.
func bindImportMap(r Request, in Input, q *intent.Queue) (*intent.ImportMap, error) {
	if q.ImportMapSha256 == nil {
		if r.ImportMap != nil || in.ImportMap != nil {
			return nil, malformed("import map without importMapSha256")
		}
		return nil, nil
	}
	raw := in.ImportMap
	if r.Operation == Init && len(in.Head) == 0 {
		if raw = r.ImportMap; raw == nil {
			return nil, malformed("importMapSha256 names an import map INIT did not bind")
		}
	} else if raw == nil {
		return nil, nil
	}
	if wire.Sum(raw) != *q.ImportMapSha256 {
		return nil, malformed("import map differs from importMapSha256")
	}
	m, e := intent.DecodeImportMap(raw)
	if e != nil {
		return nil, e
	}
	if m.QueueID.Raw != q.QueueID.Raw {
		return nil, malformed("import map queue identity")
	}
	return m, nil
}

// importMapUnchanged keeps the import map immutable after genesis
// (CTS-V0-016): no later transaction posts it, and a queue post keeps the
// queue's importMapSha256.
func importMapUnchanged(r Request, state inputState, posts map[string][]byte) error {
	// IMPORT_PLAN is the one amendment path; its model checks it (CTS-V0-020).
	if r.Operation == Init || r.Operation == ImportPlan {
		return nil
	}
	if _, ok := posts[importMapPath]; ok {
		return malformed("the import map is immutable after genesis")
	}
	raw, ok := posts["intent/queue.json"]
	if !ok {
		return nil
	}
	next, e := intent.DecodeQueue(raw)
	if e != nil {
		return e
	}
	before, after := state.queue.ImportMapSha256, next.ImportMapSha256
	if (before == nil) != (after == nil) || (before != nil && *before != *after) {
		return malformed("a queue post changed importMapSha256")
	}
	return nil
}

// importMapAdmits checks an imported record against the queue's import map
// (CTS-V0-017): a mapped source item imports only to its mapped ticket, and
// a mapped ticket takes no other source item. A mapped queue's import needs
// the map's bytes.
func importMapAdmits(state inputState, rec *ticket.Record) error {
	if state.queue.ImportMapSha256 == nil {
		return nil
	}
	if state.importMap == nil {
		return malformed("import into a mapped queue requires its import map")
	}
	if err := importMapConflict(state.importMap, rec); err != nil {
		return err
	}
	return nil
}

// importMapConflict is the CTS-V0-017 identity check of one record against
// the map, or nil when the map admits it.
func importMapConflict(m *intent.ImportMap, rec *ticket.Record) *wire.Error {
	if rec.Source.SourceItemID == nil {
		return nil
	}
	where := "/tickets/" + rec.TicketID.Raw
	for _, entry := range m.Entries {
		sameSource := entry.SourceQueueID == rec.Source.SourceQueueID && entry.SourceItemID == *rec.Source.SourceItemID
		if sameSource && entry.TicketID.Raw != rec.TicketID.Raw {
			return wire.Errorf(wire.CodeDuplicateID, where, "source item %s of %s is mapped to %s", entry.SourceItemID, entry.SourceQueueID, entry.TicketID.Raw)
		}
		if !sameSource && entry.TicketID.Raw == rec.TicketID.Raw {
			return wire.Errorf(wire.CodeDuplicateID, where, "ticket is mapped to source item %s of %s", entry.SourceItemID, entry.SourceQueueID)
		}
	}
	return nil
}

// ImportMapConflicts checks a whole import plan against the queue's journaled
// import map before the first batch is written (CTS-V0-018). It refuses
// DUPLICATE_ID naming every conflicting source item, so a conflict is never
// discovered only when the batch that holds it commits. A nil map admits the
// plan; the IMPORT_APPLY model still applies CTS-V0-017 to each batch.
func ImportMapConflicts(importMap []byte, records [][]byte) error {
	if importMap == nil {
		return nil
	}
	m, err := intent.DecodeImportMap(importMap)
	if err != nil {
		return err
	}
	var conflicts []string
	for _, raw := range records {
		rec, err := ticket.Decode(raw)
		if err != nil {
			return err
		}
		if e := importMapConflict(m, rec); e != nil {
			conflicts = append(conflicts, fmt.Sprintf("source item %s of %s -> %s: %s", *rec.Source.SourceItemID, rec.Source.SourceQueueID, rec.TicketID.Raw, e.Msg))
		}
	}
	if len(conflicts) == 0 {
		return nil
	}
	return wire.Errorf(wire.CodeDuplicateID, "/tickets", "%d import-map conflicts: %s", len(conflicts), strings.Join(conflicts, "; "))
}

// importMapAmendConflicts is the CTS-V0-021 identity check of an amended map
// against the journaled one and the store's tickets. Every ticket the old map
// names stays mapped, possibly under a renamed source item; a source item the
// old map names keeps its ticket; a source item an existing ticket imports
// stays on that ticket; no ticket is mapped twice; and an entry for a ticket
// the old map did not name may only name a store ticket that is an import of
// exactly that source item. It names every conflict, or nil.
func importMapAmendConflicts(old, next *intent.ImportMap, tickets *ticket.Inventory) error {
	key := func(e intent.ImportEntry) string { return e.SourceQueueID + "\x00" + e.SourceItemID }
	oldByKey, oldTickets := map[string]string{}, map[string]bool{}
	for _, e := range old.Entries {
		oldByKey[key(e)] = e.TicketID.Raw
		oldTickets[e.TicketID.Raw] = true
	}
	imported := map[string]string{}
	if tickets != nil {
		for _, id := range tickets.IDs() {
			if rec, ok := tickets.Get(id); ok && rec.Source.SourceItemID != nil {
				imported[rec.Source.SourceQueueID+"\x00"+*rec.Source.SourceItemID] = id
			}
		}
	}
	var conflicts []string
	nextTickets := map[string]bool{}
	for _, e := range next.Entries {
		item := fmt.Sprintf("source item %s of %s", e.SourceItemID, e.SourceQueueID)
		if nextTickets[e.TicketID.Raw] {
			conflicts = append(conflicts, fmt.Sprintf("%s: ticket %s is mapped twice", item, e.TicketID.Raw))
		}
		nextTickets[e.TicketID.Raw] = true
		if was, ok := oldByKey[key(e)]; ok && was != e.TicketID.Raw {
			conflicts = append(conflicts, fmt.Sprintf("%s: moves from ticket %s to %s", item, was, e.TicketID.Raw))
			continue
		}
		if held, ok := imported[key(e)]; ok && held != e.TicketID.Raw {
			conflicts = append(conflicts, fmt.Sprintf("%s: is imported as ticket %s, not %s", item, held, e.TicketID.Raw))
			continue
		}
		if oldTickets[e.TicketID.Raw] || tickets == nil {
			continue
		}
		if rec, ok := tickets.Get(e.TicketID.Raw); ok && (rec.Source.SourceItemID == nil || rec.Source.SourceQueueID != e.SourceQueueID || *rec.Source.SourceItemID != e.SourceItemID) {
			conflicts = append(conflicts, fmt.Sprintf("%s: ticket %s exists and is not that item's import", item, e.TicketID.Raw))
		}
	}
	for _, e := range old.Entries {
		if !nextTickets[e.TicketID.Raw] {
			conflicts = append(conflicts, fmt.Sprintf("ticket %s loses its mapping (source item %s of %s)", e.TicketID.Raw, e.SourceItemID, e.SourceQueueID))
		}
	}
	if len(conflicts) == 0 {
		return nil
	}
	return fmt.Errorf("%d import-map amendment conflicts: %s", len(conflicts), strings.Join(conflicts, "; "))
}
