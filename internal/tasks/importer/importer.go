// Package importer maps a foreign roadmap export onto shadow IMPORT ticket
// records (CTS-V0-003). It is pure: it reads the export and the current
// canonical inventory and returns the post records to write, and never
// performs I/O.
package importer

import (
	"bytes"
	"sort"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// Profile names the export format: a header line, then one item per line.
const Profile = "corvint-tasks-import/0"

// MaxExportBytes bounds the whole export read by the CLI.
const MaxExportBytes = wire.MaxIntentTreeBytes

// TicketKeys is the closed key set of an export item's "ticket" object: every
// taskman-ticket/0 field the exporter maps. The importer owns the rest
// (profile, ticketId, revision, acceptanceRevision, previousRecordSha256,
// source, approvals, shadowOverlay, createdAt, updatedAt, updatedBy) and
// stamps holds[].placedAt and completion kind, evidence, manifest and
// recordedAt.
var TicketKeys = []string{
	"acceptanceCriteria", "archivedFrom", "body", "capabilities", "completion", "dependencies",
	"dueDate", "effects", "estimateMinutes", "executionClass", "holds", "kind", "labels",
	"milestone", "order", "owner", "priority", "requiredGates", "requirementRefs", "status",
	"supersededBy", "supersedes", "title",
}

// Item is one foreign ticket: its primary ID, verbatim block and mapping.
type Item struct {
	SourceItemID string
	Block        string
	Ticket       wire.Value
}

// Export is a decoded export.
type Export struct {
	SourceQueueID string
	Items         []Item
}

// Decode reads the export: line 1 is {"profile","sourceQueueId"}, each later
// line is {"sourceItemId","block","ticket"}. Every line ends in LF and no
// sourceItemId repeats.
func Decode(raw []byte) (*Export, error) {
	if len(raw) > MaxExportBytes {
		return nil, wire.Errorf(wire.CodeLimitExceeded, "export", "export larger than %d bytes", MaxExportBytes)
	}
	if len(raw) == 0 || raw[len(raw)-1] != '\n' {
		return nil, wire.Errorf(wire.CodeMalformed, "export", "export must end in LF")
	}
	lines := bytes.Split(raw[:len(raw)-1], []byte("\n"))
	header, err := line(lines[0], 1)
	if err != nil {
		return nil, err
	}
	h := wire.NewReader(header, "export/header")
	h.Closed("profile", "sourceQueueId")
	profile := h.Field("profile").String()
	exp := &Export{SourceQueueID: h.Field("sourceQueueId").Identifier()}
	if err = h.Err(); err != nil {
		return nil, err
	}
	if err = wire.CheckProfile("export/header/profile", profile, Profile); err != nil {
		return nil, err
	}
	if len(lines)-1 > wire.MaxTicketsPerQueue {
		return nil, wire.Errorf(wire.CodeLimitExceeded, "export", "more than %d items", wire.MaxTicketsPerQueue)
	}
	seen := map[string]bool{}
	for i, body := range lines[1:] {
		v, err := line(body, i+2)
		if err != nil {
			return nil, err
		}
		r := wire.NewReader(v, "export/"+string(wire.CountOf(int64(i+2))))
		r.Closed("sourceItemId", "block", "ticket")
		item := Item{SourceItemID: r.Field("sourceItemId").Identifier(), Block: r.Field("block").Prose(1, wire.MaxBodyBytes)}
		t := r.Field("ticket")
		t.Closed(TicketKeys...)
		item.Ticket = t.Value()
		if err = r.Err(); err != nil {
			return nil, err
		}
		if seen[item.SourceItemID] {
			return nil, wire.Errorf(wire.CodeDuplicateID, r.Where(), "sourceItemId %s repeats", item.SourceItemID)
		}
		seen[item.SourceItemID] = true
		exp.Items = append(exp.Items, item)
	}
	return exp, nil
}

func line(body []byte, n int) (wire.Value, error) {
	if len(body) == 0 {
		return wire.Value{}, wire.Errorf(wire.CodeMalformed, "export/"+string(wire.CountOf(int64(n))), "blank line")
	}
	return wire.Parse(append(bytes.Clone(body), '\n'))
}

// Store is the canonical state an import plan reads.
type Store struct {
	Queue   *intent.Queue
	Policy  *intent.Policy
	Tickets *ticket.Inventory
}

// Plan returns the canonical post records for every new or changed item, in
// ascending ticketId order. An item whose ticket already holds its block's
// digest is unchanged and yields nothing. Plan validates the whole export
// against the prospective inventory before returning any record, so a refusal
// means nothing may be written.
func Plan(exp *Export, st Store, actor string, now wire.Timestamp) ([][]byte, error) {
	if st.Queue.CanonicalWriter == "NATIVE" {
		return nil, wire.Errorf(wire.CodeUnsupported, "canonicalWriter", "import into a NATIVE-written queue would make imported records eligible without a cutover")
	}
	prospective := map[string]*ticket.Record{}
	for _, id := range st.Tickets.IDs() {
		prospective[id], _ = st.Tickets.Get(id)
	}
	posted := []*ticket.Record{}
	for _, item := range exp.Items {
		rec, err := record(exp.SourceQueueID, item, st, prospective, actor, now)
		if err != nil {
			return nil, err
		}
		if rec == nil {
			continue
		}
		prospective[rec.TicketID.Raw] = rec
		posted = append(posted, rec)
	}
	all := make([]*ticket.Record, 0, len(prospective))
	for _, rec := range prospective {
		all = append(all, rec)
	}
	inv, err := ticket.NewInventory(st.Queue.QueueID, all)
	if err != nil {
		return nil, err
	}
	gates := st.Policy.GateIDs()
	for _, rec := range posted {
		if err = references(rec, inv, gates); err != nil {
			return nil, err
		}
	}
	sort.Slice(posted, func(i, j int) bool { return posted[i].TicketID.Raw < posted[j].TicketID.Raw })
	out := make([][]byte, len(posted))
	for i, rec := range posted {
		out[i] = rec.Encode()
	}
	return out, nil
}

// record maps one item, or returns nil when its ticket already holds the
// same source revision.
func record(sourceQueue string, item Item, st Store, current map[string]*ticket.Record, actor string, now wire.Timestamp) (*ticket.Record, error) {
	id, err := wire.ParseTicketID("sourceItemId", "ticket:"+st.Queue.QueueID.Authority+":"+st.Queue.QueueID.Queue+":"+item.SourceItemID)
	if err != nil {
		return nil, err
	}
	revision := wire.Sum([]byte(item.Block))
	pre := current[id.Raw]
	if pre != nil {
		if pre.Source.Kind != "IMPORT" {
			return nil, wire.Errorf(wire.CodeDuplicateID, id.Raw, "ticket is held by a %s record", pre.Source.Kind)
		}
		if pre.Source.SourceQueueID != sourceQueue || *pre.Source.SourceItemID != item.SourceItemID {
			return nil, wire.Errorf(wire.CodeDuplicateID, id.Raw, "ticket is held by source item %s of %s", *pre.Source.SourceItemID, pre.Source.SourceQueueID)
		}
		if *pre.Source.SourceRevisionSha256 == revision {
			return nil, nil
		}
	}
	o := wire.NewObject()
	for _, k := range TicketKeys {
		v, _ := item.Ticket.Obj.Get(k)
		o.Set(k, v)
	}
	o.Set("profile", wire.String(ticket.Profile))
	o.Set("ticketId", wire.String(id.Raw))
	o.Set("source", wire.ObjectValue(source(sourceQueue, item.SourceItemID, revision)))
	o.Set("approvals", wire.Array())
	o.Set("shadowOverlay", wire.Bool(true))
	o.Set("updatedAt", wire.String(string(now)))
	o.Set("updatedBy", wire.String(actor))
	holds, err := stampHolds(o, pre, now)
	if err != nil {
		return nil, err
	}
	o.Set("holds", holds)
	completion, err := stampCompletion(o, pre, now)
	if err != nil {
		return nil, err
	}
	o.Set("completion", completion)
	chain(o, pre, now)
	rec, err := ticket.FromValue(wire.ObjectValue(o))
	if err != nil {
		return nil, wire.Errorf(wire.CodeOf(err), id.Raw, "%v", err)
	}
	return rec, nil
}

func source(queue, item string, revision wire.Digest) *wire.Object {
	s := wire.NewObject()
	s.Set("kind", wire.String("IMPORT"))
	s.Set("sourceQueueId", wire.String(queue))
	s.Set("sourceItemId", wire.String(item))
	s.Set("sourceRevisionSha256", wire.String(string(revision)))
	return s
}

// chain sets the revision fields: revision 1 for a new ticket, otherwise the
// next revision of pre, bumping acceptanceRevision only when an
// acceptance-relevant field changed (§3.1).
func chain(o *wire.Object, pre *ticket.Record, now wire.Timestamp) {
	if pre == nil {
		o.Set("revision", wire.String("1"))
		o.Set("acceptanceRevision", wire.String("1"))
		o.Set("previousRecordSha256", wire.Null())
		o.Set("createdAt", wire.String(string(now)))
		return
	}
	acceptance := pre.AcceptanceRevision.Int()
	old := pre.Value().Obj
	for _, f := range ticket.AcceptanceRelevantFields {
		a, _ := old.Get(f)
		b, _ := o.Get(f)
		if !wire.Equal(a, b) {
			acceptance++
			break
		}
	}
	o.Set("revision", wire.String(string(wire.CountOf(pre.Revision.Int()+1))))
	o.Set("acceptanceRevision", wire.String(string(wire.CountOf(acceptance))))
	o.Set("previousRecordSha256", wire.String(string(pre.FileDigest())))
	o.Set("createdAt", wire.String(string(pre.CreatedAt)))
}

// stampHolds adds placedAt to each exported {holdId, actor, reason}, keeping
// the time a hold of the same holdId was first placed.
func stampHolds(o *wire.Object, pre *ticket.Record, now wire.Timestamp) (wire.Value, error) {
	placed := map[string]wire.Timestamp{}
	if pre != nil {
		for _, h := range pre.Holds {
			placed[h.HoldID] = h.PlacedAt
		}
	}
	v, _ := o.Get("holds")
	r := wire.NewReader(v, "/holds")
	out := []wire.Value{}
	for _, h := range r.Array(wire.MaxHolds, true) {
		h.Closed("holdId", "actor", "reason")
		id := h.Field("holdId").Label()
		at, ok := placed[id]
		if !ok {
			at = now
		}
		ho := wire.NewObject()
		ho.Set("holdId", wire.String(id))
		ho.Set("actor", h.Field("actor").Value())
		ho.Set("reason", h.Field("reason").Value())
		ho.Set("placedAt", wire.String(string(at)))
		out = append(out, wire.ObjectValue(ho))
	}
	return wire.Array(out...), r.Err()
}

// stampCompletion turns an exported {actor, reason} into a MANUAL completion
// (the foreign checkoff carries no manifest), keeping an earlier recordedAt.
func stampCompletion(o *wire.Object, pre *ticket.Record, now wire.Timestamp) (wire.Value, error) {
	v, _ := o.Get("completion")
	r := wire.NewReader(v, "/completion")
	if r.IsNull() {
		return v, nil
	}
	r.Closed("actor", "reason")
	at := now
	if pre != nil && pre.Completion != nil {
		at = pre.Completion.RecordedAt
	}
	co := wire.NewObject()
	co.Set("kind", wire.String("MANUAL"))
	co.Set("actor", r.Field("actor").Value())
	co.Set("reason", r.Field("reason").Value())
	co.Set("evidence", wire.Array())
	co.Set("manifestSha256", wire.Null())
	co.Set("recordedAt", wire.String(string(at)))
	return wire.ObjectValue(co), r.Err()
}

// references requires every dependency and supersession target to exist in
// the prospective inventory and every gate to be declared by policy. A
// dependency cycle is not refused here: the inventory records it as a read
// problem, as it does for any store.
func references(rec *ticket.Record, inv *ticket.Inventory, gates map[string]bool) error {
	for _, d := range rec.Dependencies {
		if _, ok := inv.Get(d.TicketID.Raw); !ok {
			return wire.Errorf(wire.CodeDependencyMissing, rec.TicketID.Raw, "dependency %s is in neither the export nor the store", d.TicketID.Raw)
		}
		if d.GateID != nil && !gates[*d.GateID] {
			return wire.Errorf(wire.CodeGateUnknown, rec.TicketID.Raw, "dependency gate %q is not declared by policy", *d.GateID)
		}
	}
	for _, g := range rec.RequiredGates {
		if !gates[g] {
			return wire.Errorf(wire.CodeGateUnknown, rec.TicketID.Raw, "required gate %q is not declared by policy", g)
		}
	}
	for _, ref := range []*wire.TicketID{rec.Supersedes, rec.SupersededBy} {
		if ref == nil {
			continue
		}
		if _, ok := inv.Get(ref.Raw); !ok {
			return wire.Errorf(wire.CodeDependencyMissing, rec.TicketID.Raw, "supersession target %s is in neither the export nor the store", ref.Raw)
		}
	}
	return nil
}
