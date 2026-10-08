package taskman

import (
	"errors"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/wire"
	taskswire "github.com/Beamfall/corvint/internal/tasks/wire"
)

// knowHow validates the optional KHN-V0-002 ledger with the bounds and
// relationships the Tasks codec enforces: 1..32 closed ADD or RETRACT entries
// in append order, seq equal to the 1-based position, a superseding ADD or a
// RETRACT naming an earlier still-active ADD, and a reason exactly when an
// ADD supersedes. Absence is valid. The ledger is agent-authored data: this
// reader admits it and nothing in Core ranks, cites or trusts it.
func knowHow(v wire.Value, _, _ uint64) error {
	entries, e := array(v, taskswire.KnowHowMaxEntries)
	if e != nil || len(entries) == 0 {
		return errors.New("know-how entries")
	}
	active := map[uint64]bool{}
	for i, x := range entries {
		op := value(x, "operation")
		var target *uint64
		switch {
		case op.Kind == wire.KindString && op.Str == "ADD":
			if e = object(x, "actor anchors attempt commit evidencePath generation operation reason recordedAt routes seq supersedes text"); e != nil {
				return errors.New("know-how ADD entry")
			}
			if e = knowHowAdd(x); e != nil {
				return e
			}
			if s := value(x, "supersedes"); s.Kind != wire.KindNull {
				n, e := number(s, 2147483647)
				if e != nil {
					return errors.New("know-how supersedes")
				}
				target = &n
			}
			if (target == nil) != (value(x, "reason").Kind == wire.KindNull) {
				return errors.New("know-how reason is required exactly when an ADD supersedes")
			}
		case op.Kind == wire.KindString && op.Str == "RETRACT":
			if e = object(x, "actor note operation reason recordedAt seq"); e != nil {
				return errors.New("know-how RETRACT entry")
			}
			if value(x, "reason").Kind == wire.KindNull {
				return errors.New("know-how RETRACT reason is required")
			}
			n, e := number(value(x, "note"), 2147483647)
			if e != nil {
				return errors.New("know-how note")
			}
			target = &n
		default:
			return errors.New("know-how operation")
		}
		if r := value(x, "reason"); r.Kind != wire.KindNull && !knowHowProse(r, taskswire.KnowHowMaxReasonBytes) {
			return errors.New("know-how reason")
		}
		if seq, e := number(value(x, "seq"), 2147483647); e != nil || seq != uint64(i+1) {
			return errors.New("know-how seq")
		}
		if e = knowHowActor(x); e != nil {
			return e
		}
		if value(value(x, "actor"), "role").Str == "WORKER" && (op.Str != "ADD" || target != nil || value(x, "attempt").Kind == wire.KindNull || value(x, "generation").Kind == wire.KindNull) {
			// KHN-V0-023: a WORKER entry is a non-superseding ADD that records
			// its attempt and generation.
			return errors.New("know-how WORKER entry")
		}
		if t := value(x, "recordedAt"); t.Kind != wire.KindString {
			return errors.New("know-how time")
		} else if _, e = taskswire.ParseTimestamp("recordedAt", t.Str); e != nil {
			return errors.New("know-how time")
		}
		if target != nil {
			if !active[*target] {
				return errors.New("know-how entry names a note that is not an earlier active note")
			}
			delete(active, *target)
		}
		if op.Str == "ADD" {
			active[uint64(i+1)] = true
		}
	}
	return nil
}

func knowHowAdd(x wire.Value) error {
	if !knowHowProse(value(x, "text"), taskswire.KnowHowMaxTextBytes) {
		return errors.New("know-how text")
	}
	if c := value(x, "commit"); c.Kind != wire.KindString {
		return errors.New("know-how commit")
	} else if _, e := taskswire.ParseOID("commit", c.Str); e != nil {
		return errors.New("know-how commit")
	}
	anchors, e := array(value(x, "anchors"), taskswire.KnowHowMaxAnchors)
	if e != nil || len(anchors) == 0 {
		return errors.New("know-how anchors")
	}
	for i, a := range anchors {
		if object(a, "blob path") != nil || !knowHowFile(value(a, "path")) {
			return errors.New("know-how anchor")
		}
		if b := value(a, "blob"); b.Kind != wire.KindString {
			return errors.New("know-how anchor blob")
		} else if _, e := taskswire.ParseOID("blob", b.Str); e != nil {
			return errors.New("know-how anchor blob")
		}
		if i > 0 && stringAt(anchors[i-1], "path") >= stringAt(a, "path") {
			return errors.New("know-how anchors are sorted by path without duplicates")
		}
	}
	routes, e := array(value(x, "routes"), taskswire.KnowHowMaxRoutes)
	if e != nil {
		return errors.New("know-how routes")
	}
	for i, r := range routes {
		if r.Kind != wire.KindString || (i > 0 && routes[i-1].Str >= r.Str) {
			return errors.New("know-how routes are a sorted set")
		}
		if _, e := taskswire.ParseToken("route", r.Str, taskswire.KnowHowMaxRouteBytes); e != nil {
			return errors.New("know-how route")
		}
	}
	if a := value(x, "attempt"); a.Kind != wire.KindNull {
		if a.Kind != wire.KindString {
			return errors.New("know-how attempt")
		} else if _, e := taskswire.ParseIdentifier("attempt", a.Str); e != nil {
			return errors.New("know-how attempt")
		}
	}
	if g := value(x, "generation"); g.Kind != wire.KindNull {
		if g.Kind != wire.KindString {
			return errors.New("know-how generation")
		} else if _, e := taskswire.ParseSize("generation", g.Str); e != nil {
			return errors.New("know-how generation")
		}
	}
	if p := value(x, "evidencePath"); p.Kind != wire.KindNull && !knowHowFile(p) {
		return errors.New("know-how evidence path")
	}
	return nil
}

func knowHowActor(x wire.Value) error {
	a := value(x, "actor")
	if object(a, "id role") != nil {
		return errors.New("know-how actor")
	}
	id, role := value(a, "id"), value(a, "role")
	if id.Kind != wire.KindString || role.Kind != wire.KindString || (role.Str != "OWNER" && role.Str != "OPERATOR" && role.Str != "WORKER") {
		return errors.New("know-how actor")
	}
	if _, e := taskswire.ParseLabel("actor", id.Str); e != nil {
		return errors.New("know-how actor")
	}
	return nil
}

func knowHowProse(v wire.Value, max int) bool {
	if v.Kind != wire.KindString || strings.TrimSpace(v.Str) == "" {
		return false
	}
	_, e := taskswire.ParseProse("prose", v.Str, 1, max)
	return e == nil
}

func knowHowFile(v wire.Value) bool {
	if v.Kind != wire.KindString || strings.HasSuffix(v.Str, "/") {
		return false
	}
	_, e := taskswire.ParsePath("path", v.Str)
	return e == nil
}
