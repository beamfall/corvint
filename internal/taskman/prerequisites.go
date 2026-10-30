package taskman

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/Beamfall/corvint/internal/cem/wire"
	taskswire "github.com/Beamfall/corvint/internal/tasks/wire"
)

// executionPrerequisites validates the optional CAL-V0-099 member: a
// non-empty, canonical-byte-sorted, duplicate-free array of at most
// MaxDependencies closed {ticketId, obligation, gateId, stages} entries,
// gateId non-null iff GATE_PASSED, stages a non-empty sorted set of
// implement/integrate/review. As in the native codec, each ticketId is a
// well-formed ticket ID and each (ticketId, obligation, gateId) edge appears
// once; prerequisiteOwner applies the rules that need the record's own ID.
func executionPrerequisites(v wire.Value, _, _ uint64) error {
	items, e := array(v, taskswire.MaxDependencies)
	if e != nil || len(items) == 0 {
		return errors.New("prerequisite count")
	}
	edges := map[string]bool{}
	var prev []byte
	for i, x := range items {
		if e = object(x, "ticketId obligation gateId stages"); e != nil {
			return e
		}
		if b := canonical(x); i > 0 && bytes.Compare(prev, b) >= 0 {
			return errors.New("unsorted/duplicate prerequisites")
		} else {
			prev = b
		}
		if value(x, "ticketId").Kind != wire.KindString {
			return errors.New("prerequisite ticket")
		}
		if _, e = taskswire.ParseTicketID("/executionPrerequisites/ticketId", stringAt(x, "ticketId")); e != nil {
			return fmt.Errorf("prerequisite ticket: %w", e)
		}
		obligation := stringAt(x, "obligation")
		if value(x, "obligation").Kind != wire.KindString || !oneOf(obligation, "COMPLETED GATE_PASSED") {
			return errors.New("prerequisite obligation")
		}
		gate := value(x, "gateId")
		switch {
		case obligation == "GATE_PASSED" && gate.Kind == wire.KindString:
			if _, e = taskswire.ParseLabel("/executionPrerequisites/gateId", gate.Str); e != nil {
				return e
			}
		case obligation == "COMPLETED" && gate.Kind == wire.KindNull:
		default:
			return errors.New("prerequisite gate")
		}
		edge := stringAt(x, "ticketId") + "\x00" + obligation + "\x00" + gate.Str
		if edges[edge] {
			return errors.New("duplicate prerequisite edge")
		}
		edges[edge] = true
		stages, e := array(value(x, "stages"), 3)
		if e != nil || len(stages) == 0 {
			return errors.New("prerequisite stages")
		}
		for j, s := range stages {
			if s.Kind != wire.KindString || !oneOf(s.Str, "implement integrate review") {
				return errors.New("prerequisite stage")
			}
			if j > 0 && stages[j-1].Str >= s.Str {
				return errors.New("unsorted/duplicate prerequisite stages")
			}
		}
	}
	return nil
}

// prerequisiteOwner applies the native edge rules that need the record's own
// ID: every prerequisite is in the record's queue and is never the record.
func prerequisiteOwner(id string, v wire.Value) error {
	if len(v.Arr) == 0 {
		return nil
	}
	own, e := taskswire.ParseTicketID("/ticketId", id)
	if e != nil {
		return fmt.Errorf("prerequisite owner: %w", e)
	}
	for _, x := range v.Arr {
		pre, _ := taskswire.ParseTicketID("/executionPrerequisites/ticketId", stringAt(x, "ticketId"))
		if pre.QueueID() != own.QueueID() {
			return errors.New("prerequisite outside the queue")
		}
		if pre.Raw == own.Raw {
			return errors.New("self prerequisite")
		}
	}
	return nil
}

// prerequisiteBlocker is the stageless Core planner's reading of
// CAL-V0-099: every prerequisite applies. A GATE_PASSED prerequisite is
// GATE_UNKNOWN, as for dependencies; a missing or uncompleted one is
// PREREQUISITE_UNSATISFIED.
func prerequisiteBlocker(t ticket, all map[string]ticket) string {
	for _, p := range value(t.raw, "executionPrerequisites").Arr {
		if stringAt(p, "obligation") == "GATE_PASSED" {
			return "GATE_UNKNOWN"
		}
		pre, ok := all[stringAt(p, "ticketId")]
		if !ok || (pre.status != "COMPLETED" && !(pre.status == "ARCHIVED" && stringAt(pre.raw, "archivedFrom") == "COMPLETED")) {
			return taskswire.CodePrerequisiteUnsatisfied
		}
	}
	return ""
}
