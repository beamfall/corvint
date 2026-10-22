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
	if r.Operation == Init {
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
