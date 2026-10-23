package transaction

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
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
		// An amendment's own map is its request, not the store's map; the
		// model refuses it for a queue that names none (CTS-V0-020).
		if (r.ImportMap != nil && r.Operation != ImportMapUpdate) || in.ImportMap != nil {
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
// queue's importMapSha256. The one exception is the journaled amendment,
// whose posts importMapUpdatePosts builds and checks (CTS-V0-020).
func importMapUnchanged(r Request, state inputState, posts map[string][]byte) error {
	if r.Operation == Init || r.Operation == ImportMapUpdate {
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

// importMapUpdatePosts records an amended import map (CTS-V0-020..022). The
// queue must already name a journaled map, the caller must have read the
// current one, and the amendment must keep every existing ticket identity:
// each ticket the current map names stays mapped, a source item both maps
// name keeps its ticket, no ticket is mapped twice, and no ticket record's
// own source item is mapped to another ticket. A source item may move to a
// new (sourceQueueId, sourceItemId) under its ticket, which is how a renamed
// or added source queue is recorded. Any conflict refuses the whole
// amendment DUPLICATE_ID, naming each; nothing is written. It returns nil
// once posts holds the map and the queue that names it.
func importMapUpdatePosts(r Request, state inputState, posts map[string][]byte) *Result {
	q := state.queue
	if r.Actor.Role != "OWNER" {
		res := refused(r.RequestID, mutation.OutcomeUnauthorized, "", "an import map amendment needs an OWNER binding")
		return &res
	}
	if q.ImportMapSha256 == nil || state.importMap == nil {
		res := refused(r.RequestID, mutation.OutcomeBlocked, wire.CodeUnsupported, "the queue names no journaled import map to amend")
		return &res
	}
	if *q.ImportMapSha256 != r.ExpectedImportMapSha256 {
		res := refused(r.RequestID, mutation.OutcomeRevisionConflict, "", "expectedImportMapSha256 "+string(r.ExpectedImportMapSha256)+" but the current import map is "+string(*q.ImportMapSha256))
		return &res
	}
	next, e := intent.DecodeImportMap(r.ImportMap)
	if e != nil {
		res := failed(r.RequestID, e)
		return &res
	}
	if next.QueueID.Raw != q.QueueID.Raw {
		res := failed(r.RequestID, malformed("import map queue identity"))
		return &res
	}
	if wire.Sum(r.ImportMap) == *q.ImportMapSha256 {
		res := noChange(r.RequestID)
		return &res
	}
	if conflicts := importMapAmendConflicts(state.importMap, next, state.tickets); len(conflicts) > 0 {
		res := failed(r.RequestID, wire.Errorf(wire.CodeDuplicateID, "/entries", "%d import-map amendment conflicts: %s", len(conflicts), strings.Join(conflicts, "; ")))
		return &res
	}
	amended := *q
	d := wire.Sum(r.ImportMap)
	amended.ImportMapSha256 = &d
	posts["intent/queue.json"] = wire.EncodeFile(amended.Value())
	posts[importMapPath] = bytes.Clone(r.ImportMap)
	return nil
}

// importMapAmendConflicts names every way next would change an existing
// ticket identity that current or a ticket record fixes (CTS-V0-021).
func importMapAmendConflicts(current, next *intent.ImportMap, tickets *ticket.Inventory) []string {
	key := func(e intent.ImportEntry) string { return e.SourceQueueID + "\x00" + e.SourceItemID }
	name := func(e intent.ImportEntry) string { return "source item " + e.SourceItemID + " of " + e.SourceQueueID }
	nextByKey := map[string]intent.ImportEntry{}
	nextByTicket := map[string]intent.ImportEntry{}
	var conflicts []string
	for _, e := range next.Entries {
		nextByKey[key(e)] = e
		if prior, ok := nextByTicket[e.TicketID.Raw]; ok {
			conflicts = append(conflicts, fmt.Sprintf("ticket %s is mapped to both %s and %s", e.TicketID.Raw, name(prior), name(e)))
			continue
		}
		nextByTicket[e.TicketID.Raw] = e
	}
	for _, e := range current.Entries {
		if n, ok := nextByKey[key(e)]; ok && n.TicketID.Raw != e.TicketID.Raw {
			conflicts = append(conflicts, fmt.Sprintf("%s moves from %s to %s", name(e), e.TicketID.Raw, n.TicketID.Raw))
		}
		if _, ok := nextByTicket[e.TicketID.Raw]; !ok {
			conflicts = append(conflicts, fmt.Sprintf("ticket %s (%s) is dropped from the map", e.TicketID.Raw, name(e)))
		}
	}
	for _, id := range tickets.IDs() {
		rec, _ := tickets.Get(id)
		if rec == nil || rec.Source.SourceItemID == nil {
			continue
		}
		e := intent.ImportEntry{SourceQueueID: rec.Source.SourceQueueID, SourceItemID: *rec.Source.SourceItemID}
		if n, ok := nextByKey[key(e)]; ok && n.TicketID.Raw != rec.TicketID.Raw {
			conflicts = append(conflicts, fmt.Sprintf("%s is ticket %s's own source but is mapped to %s", name(e), rec.TicketID.Raw, n.TicketID.Raw))
		}
	}
	sort.Strings(conflicts)
	return conflicts
}
