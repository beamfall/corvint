package taskman

import (
	"bytes"
	"errors"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/wire"
	taskswire "github.com/Beamfall/corvint/internal/tasks/wire"
)

// executionPrerequisites validates the optional CAL-V0-099 member: a
// non-empty, canonical-byte-sorted, duplicate-free array of at most
// MaxDependencies closed {ticketId, obligation, gateId, stages} entries,
// gateId non-null iff GATE_PASSED, stages a non-empty sorted set of
// implement/integrate/review.
func executionPrerequisites(v wire.Value, _, _ uint64) error {
	items, e := array(v, taskswire.MaxDependencies)
	if e != nil || len(items) == 0 {
		return errors.New("prerequisite count")
	}
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
		if value(x, "ticketId").Kind != wire.KindString || !strings.HasPrefix(stringAt(x, "ticketId"), "ticket:") {
			return errors.New("prerequisite ticket")
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
