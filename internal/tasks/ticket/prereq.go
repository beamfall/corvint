package ticket

import "github.com/Beamfall/corvint/internal/tasks/wire"

// Prerequisite is one optional `executionPrerequisites` entry (CAL-V0-099):
// an obligation on another ticket that gates claims and plans for the listed
// stages only. Unlike a Dependency it takes no part in cycle detection,
// completion or requiredGates, and completing the prerequisite changes no
// other record.
type Prerequisite struct {
	TicketID   wire.TicketID
	Obligation string
	GateID     *string
	Stages     []string
}

// ReadPrerequisites decodes the `executionPrerequisites` array: a non-empty
// canonical-byte-sorted set (omission is the canonical empty) of closed
// {ticketId, obligation, gateId, stages} objects, bounded like
// `dependencies`. stages is a non-empty sorted set over Stages.
func ReadPrerequisites(r *wire.Reader) []Prerequisite {
	items := r.Array(wire.MaxDependencies, false)
	if r.Err() != nil {
		return nil
	}
	if len(items) == 0 {
		r.Fail(wire.CodeMalformed, "executionPrerequisites is omitted, never empty")
		return nil
	}
	out := make([]Prerequisite, 0, len(items))
	for _, e := range items {
		e.Closed("gateId", "obligation", "stages", "ticketId")
		p := Prerequisite{}
		p.TicketID = e.Field("ticketId").TicketID()
		p.Obligation = e.Field("obligation").Enum(Obligations...)
		p.GateID = e.Field("gateId").LabelOrNull()
		st := e.Field("stages")
		p.Stages = st.Strings(len(Stages), false, func(s *wire.Reader) string { return s.Enum(Stages...) })
		if r.Err() != nil {
			return nil
		}
		if len(p.Stages) == 0 {
			st.Fail(wire.CodeMalformed, "stages names at least one stage")
			return nil
		}
		out = append(out, p)
	}
	return out
}

// PrerequisitesValue renders the array in canonical-byte order.
func PrerequisitesValue(ps []Prerequisite) wire.Value {
	vs := make([]wire.Value, 0, len(ps))
	for _, p := range ps {
		o := wire.NewObject()
		o.Set("ticketId", wire.String(p.TicketID.Raw))
		o.Set("obligation", wire.String(p.Obligation))
		o.Set("gateId", wire.StringOrNull(p.GateID))
		st, _ := wire.SortedSet("", wire.Strings(p.Stages).Arr)
		o.Set("stages", st)
		vs = append(vs, wire.ObjectValue(o))
	}
	sorted, _ := wire.SortedSet("", vs)
	return sorted
}

// validatePrerequisites applies the `dependencies` edge rules: gateId
// non-null iff GATE_PASSED, same queue, never the ticket itself, and one
// entry per (ticketId, obligation, gateId).
func (rec *Record) validatePrerequisites() error {
	seen := map[string]bool{}
	for i, p := range rec.ExecutionPrerequisites {
		where := "/executionPrerequisites/" + idx(i)
		if (p.Obligation == "GATE_PASSED") != (p.GateID != nil) {
			return wire.Errorf(wire.CodeMalformed, where+"/gateId", "gateId is non-null iff obligation is GATE_PASSED")
		}
		if p.TicketID.QueueID() != rec.TicketID.QueueID() {
			return wire.Errorf(wire.CodeDependencyMissing, where+"/ticketId", "execution prerequisite %s is outside queue %s", p.TicketID.Raw, rec.TicketID.QueueID())
		}
		if p.TicketID.Raw == rec.TicketID.Raw {
			return wire.Errorf(wire.CodeMalformed, where+"/ticketId", "a ticket cannot be its own execution prerequisite")
		}
		key := p.TicketID.Raw + "\x00" + p.Obligation
		if p.GateID != nil {
			key += "\x00" + *p.GateID
		}
		if seen[key] {
			return wire.Errorf(wire.CodeDuplicateID, where, "duplicate execution prerequisite")
		}
		seen[key] = true
	}
	return nil
}

// PrerequisiteApplies reports whether p gates the given stage. A stageless
// read (claim or plan without --stage, show, blockers) fails closed: every
// prerequisite applies.
func PrerequisiteApplies(p Prerequisite, stage string) bool {
	if stage == "" {
		return true
	}
	for _, s := range p.Stages {
		if s == stage {
			return true
		}
	}
	return false
}
