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
	if r.Operation == ImportMapAmend {
		return importMapAmendBound(state, posts)
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

// importMapAmendBound requires an amendment's two posts to agree: the queue
// post changes nothing but importMapSha256, which names the posted map. The
// model builds both, so a disagreement is a model defect, never an input.
func importMapAmendBound(state inputState, posts map[string][]byte) error {
	mapRaw, queueRaw := posts[importMapPath], posts["intent/queue.json"]
	if mapRaw == nil || queueRaw == nil || len(posts) != 2 {
		return malformed("an import map amendment posts exactly the map and the queue")
	}
	next, e := intent.DecodeQueue(queueRaw)
	if e != nil {
		return e
	}
	want := wire.Sum(mapRaw)
	bound := *state.queue
	bound.ImportMapSha256 = &want
	if next.ImportMapSha256 == nil || *next.ImportMapSha256 != want || !bytes.Equal(queueRaw, wire.EncodeFile(bound.Value())) {
		return malformed("the amended queue changes more than importMapSha256")
	}
	return nil
}

// maxAmendConflicts bounds the conflicts a refusal names; the count is exact.
const maxAmendConflicts = 20

// importMapAmendPosts plans an ImportMapAmend (CTS-V0-021..023): an OWNER
// binding, no write barrier, a queue still written by its foreign source
// (imports are closed after the authority switch), the caller's expected map
// digest equal to the current one, and an amended map that differs from it and
// keeps every identity the current map and the stored tickets hold. It
// returns the refusal, or nil with the map and queue posts added.
func importMapAmendPosts(r Request, state inputState, posts map[string][]byte) *Result {
	refuse := func(outcome, code, detail string) *Result {
		res := refused(r.RequestID, outcome, code, detail)
		return &res
	}
	if r.Actor.Role != "OWNER" {
		return refuse(mutation.OutcomeUnauthorized, "", "an import map amendment needs an OWNER binding")
	}
	q := state.queue
	if state.barrier != nil || q.WriteBarrier.Reason != "NONE" {
		return refuse(mutation.OutcomeBlocked, wire.CodePaused, "an import map amendment under a barrier")
	}
	if q.CanonicalWriter == "NATIVE" {
		return refuse(mutation.OutcomeBlocked, wire.CodeUnsupported, "canonicalWriter is NATIVE; imports are closed, so the import map no longer changes")
	}
	if q.ImportMapSha256 == nil || state.importMap == nil {
		failure := failed(r.RequestID, malformed("the queue names no import map to amend"))
		return &failure
	}
	if *q.ImportMapSha256 != r.ExpectedImportMapSha256 {
		return refuse(mutation.OutcomeRevisionConflict, "", "expectedImportMapSha256 "+string(r.ExpectedImportMapSha256)+" but the current import map is "+string(*q.ImportMapSha256))
	}
	amended := wire.Sum(r.ImportMap)
	if amended == *q.ImportMapSha256 {
		return refuse(mutation.OutcomeBlocked, "", "the amended import map equals the current one")
	}
	next, e := intent.DecodeImportMap(r.ImportMap)
	if e != nil {
		failure := failed(r.RequestID, e)
		return &failure
	}
	if e := amendConflicts(state.importMap, next, state.tickets); e != nil {
		failure := failed(r.RequestID, e)
		return &failure
	}
	bound := *q
	bound.ImportMapSha256 = &amended
	posts[importMapPath] = bytes.Clone(r.ImportMap)
	posts["intent/queue.json"] = wire.EncodeFile(bound.Value())
	return nil
}

// amendConflicts checks an amended map against the current one and the stored
// tickets (CTS-V0-022). Every current entry's ticket stays mapped to the same
// source item, revision and applied sequence; only its sourceQueueId may
// change (a source rename). A source item the current map assigns stays with
// its ticket, and an entry for a ticket the current map does not name must
// agree with that ticket's stored source. It refuses DUPLICATE_ID naming the
// conflicts.
func amendConflicts(current, next *intent.ImportMap, tickets *ticket.Inventory) error {
	nextByTicket := map[string]intent.ImportEntry{}
	for _, e := range next.Entries {
		nextByTicket[e.TicketID.Raw] = e
	}
	currentByTicket := map[string]bool{}
	assigned := map[string]string{}
	var conflicts []string
	for _, e := range current.Entries {
		currentByTicket[e.TicketID.Raw] = true
		assigned[e.SourceQueueID+"\x00"+e.SourceItemID] = e.TicketID.Raw
		n, ok := nextByTicket[e.TicketID.Raw]
		switch {
		case !ok:
			conflicts = append(conflicts, fmt.Sprintf("ticket %s is dropped from the map", e.TicketID.Raw))
		case n.SourceItemID != e.SourceItemID || n.AppliedSeq != e.AppliedSeq || (n.SourceRevisionSha256 == nil) != (e.SourceRevisionSha256 == nil) || (n.SourceRevisionSha256 != nil && *n.SourceRevisionSha256 != *e.SourceRevisionSha256):
			conflicts = append(conflicts, fmt.Sprintf("ticket %s is remapped to another source item, revision or applied sequence", e.TicketID.Raw))
		}
	}
	for _, n := range next.Entries {
		if held, ok := assigned[n.SourceQueueID+"\x00"+n.SourceItemID]; ok && held != n.TicketID.Raw {
			conflicts = append(conflicts, fmt.Sprintf("source item %s of %s is reassigned from %s to %s", n.SourceItemID, n.SourceQueueID, held, n.TicketID.Raw))
		}
		if currentByTicket[n.TicketID.Raw] {
			continue
		}
		if rec, ok := tickets.Get(n.TicketID.Raw); ok && (rec.Source.SourceItemID == nil || rec.Source.SourceQueueID != n.SourceQueueID || *rec.Source.SourceItemID != n.SourceItemID) {
			conflicts = append(conflicts, fmt.Sprintf("ticket %s exists with another source than source item %s of %s", n.TicketID.Raw, n.SourceItemID, n.SourceQueueID))
		}
	}
	if len(conflicts) == 0 {
		return nil
	}
	sort.Strings(conflicts)
	shown := conflicts
	if len(shown) > maxAmendConflicts {
		shown = shown[:maxAmendConflicts]
	}
	return wire.Errorf(wire.CodeDuplicateID, "/entries", "%d import-map amendment conflicts: %s", len(conflicts), strings.Join(shown, "; "))
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
