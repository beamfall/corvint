package taskman

import (
	"errors"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/wire"
	taskswire "github.com/Beamfall/corvint/internal/tasks/wire"
)

// knowHow validates the optional KHN-V0-002 ledger with the bounds and
// relationships the Tasks codec enforces: 1..32 closed ADD, RETRACT or
// RECONFIRM entries in append order, seq equal to the 1-based position, a
// superseding ADD, a RETRACT or a RECONFIRM naming an earlier still-active
// ADD, a reason exactly when an ADD supersedes, and a RECONFIRM re-pinning
// its note's anchors with at least one changed pin (KHN-V0-011). Absence is
// valid. The ledger is agent-authored data: this reader admits it and nothing
// in Core ranks, cites or trusts it.
func knowHow(v wire.Value, _, _ uint64) error {
	entries, e := array(v, taskswire.KnowHowMaxEntries)
	if e != nil || len(entries) == 0 {
		return errors.New("know-how entries")
	}
	active := map[uint64]bool{}
	pins := map[uint64][]taskswire.KnowHowPin{}
	for i, x := range entries {
		op := value(x, "operation")
		var target *uint64
		var anchors []taskswire.KnowHowPin
		switch {
		case op.Kind == wire.KindString && op.Str == "ADD":
			if e = object(x, "actor anchors attempt commit evidencePath generation operation reason recordedAt routes seq supersedes text"); e != nil {
				return errors.New("know-how ADD entry")
			}
			if anchors, e = knowHowAdd(x); e != nil {
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
		case op.Kind == wire.KindString && op.Str == "RECONFIRM":
			if e = object(x, "actor anchors attempt commit generation note operation recordedAt seq"); e != nil {
				return errors.New("know-how RECONFIRM entry")
			}
			if anchors, e = knowHowPins(x); e != nil {
				return e
			}
			n, e := number(value(x, "note"), 2147483647)
			if e != nil {
				return errors.New("know-how note")
			}
			target = &n
		default:
			return errors.New("know-how operation")
		}
		if op.Str != "RECONFIRM" {
			if r := value(x, "reason"); r.Kind != wire.KindNull && !knowHowProse(r, taskswire.KnowHowMaxReasonBytes) {
				return errors.New("know-how reason")
			}
		}
		if seq, e := number(value(x, "seq"), 2147483647); e != nil || seq != uint64(i+1) {
			return errors.New("know-how seq")
		}
		if e = knowHowActor(x); e != nil {
			return e
		}
		if t := value(x, "recordedAt"); t.Kind != wire.KindString {
			return errors.New("know-how time")
		} else if _, e = taskswire.ParseTimestamp("recordedAt", t.Str); e != nil {
			return errors.New("know-how time")
		}
		if target != nil && !active[*target] {
			return errors.New("know-how entry names a note that is not an earlier active note")
		}
		if op.Str == "RECONFIRM" {
			if why := taskswire.KnowHowReconfirmRefusal(pins[*target], anchors); why != "" {
				return errors.New("know-how " + why)
			}
			pins[*target] = anchors
			continue
		}
		if target != nil {
			delete(active, *target)
		}
		if op.Str == "ADD" {
			active[uint64(i+1)] = true
			pins[uint64(i+1)] = anchors
		}
	}
	return nil
}

func knowHowAdd(x wire.Value) ([]taskswire.KnowHowPin, error) {
	if !knowHowProse(value(x, "text"), taskswire.KnowHowMaxTextBytes) {
		return nil, errors.New("know-how text")
	}
	pins, e := knowHowPins(x)
	if e != nil {
		return nil, e
	}
	routes, e := array(value(x, "routes"), taskswire.KnowHowMaxRoutes)
	if e != nil {
		return nil, errors.New("know-how routes")
	}
	for i, r := range routes {
		if r.Kind != wire.KindString || (i > 0 && routes[i-1].Str >= r.Str) {
			return nil, errors.New("know-how routes are a sorted set")
		}
		if _, e := taskswire.ParseToken("route", r.Str, taskswire.KnowHowMaxRouteBytes); e != nil {
			return nil, errors.New("know-how route")
		}
	}
	if p := value(x, "evidencePath"); p.Kind != wire.KindNull && !knowHowFile(p) {
		return nil, errors.New("know-how evidence path")
	}
	return pins, nil
}

// knowHowPins validates the commit, anchors, attempt and generation an ADD
// and a RECONFIRM share and returns the anchors' freshness identities. An
// anchor is {blob, path} or, for a symbol anchor (KHN-V0-008), {blob, path,
// symbol, symbolSha256}, in strictly ascending (path, symbol) order, and the
// anchors of one path pin one blob.
func knowHowPins(x wire.Value) ([]taskswire.KnowHowPin, error) {
	if c := value(x, "commit"); c.Kind != wire.KindString {
		return nil, errors.New("know-how commit")
	} else if _, e := taskswire.ParseOID("commit", c.Str); e != nil {
		return nil, errors.New("know-how commit")
	}
	anchors, e := array(value(x, "anchors"), taskswire.KnowHowMaxAnchors)
	if e != nil || len(anchors) == 0 {
		return nil, errors.New("know-how anchors")
	}
	pins := make([]taskswire.KnowHowPin, 0, len(anchors))
	for i, a := range anchors {
		symbol := object(a, "blob path symbol symbolSha256") == nil
		if (!symbol && object(a, "blob path") != nil) || !knowHowFile(value(a, "path")) {
			return nil, errors.New("know-how anchor")
		}
		b := value(a, "blob")
		if b.Kind != wire.KindString {
			return nil, errors.New("know-how anchor blob")
		} else if _, e := taskswire.ParseOID("blob", b.Str); e != nil {
			return nil, errors.New("know-how anchor blob")
		}
		pin := taskswire.KnowHowPin{Path: stringAt(a, "path"), Pin: b.Str}
		if symbol {
			s, d := value(a, "symbol"), value(a, "symbolSha256")
			if s.Kind != wire.KindString || d.Kind != wire.KindString {
				return nil, errors.New("know-how anchor symbol")
			}
			if _, e := taskswire.ParseKnowHowSymbol("symbol", s.Str); e != nil {
				return nil, errors.New("know-how anchor symbol")
			}
			if _, e := taskswire.ParseDigest("symbolSha256", d.Str); e != nil {
				return nil, errors.New("know-how anchor symbol digest")
			}
			pin.Symbol, pin.Pin = s.Str, d.Str
		}
		if i > 0 {
			prev := pins[i-1]
			if prev.Path > pin.Path || (prev.Path == pin.Path && prev.Symbol >= pin.Symbol) {
				return nil, errors.New("know-how anchors are sorted by path and symbol without duplicates")
			}
			if prev.Path == pin.Path && stringAt(anchors[i-1], "blob") != b.Str {
				return nil, errors.New("know-how anchors of one path pin one blob")
			}
		}
		pins = append(pins, pin)
	}
	if a := value(x, "attempt"); a.Kind != wire.KindNull {
		if a.Kind != wire.KindString {
			return nil, errors.New("know-how attempt")
		} else if _, e := taskswire.ParseIdentifier("attempt", a.Str); e != nil {
			return nil, errors.New("know-how attempt")
		}
	}
	if g := value(x, "generation"); g.Kind != wire.KindNull {
		if g.Kind != wire.KindString {
			return nil, errors.New("know-how generation")
		} else if _, e := taskswire.ParseSize("generation", g.Str); e != nil {
			return nil, errors.New("know-how generation")
		}
	}
	return pins, nil
}

func knowHowActor(x wire.Value) error {
	a := value(x, "actor")
	if object(a, "id role") != nil {
		return errors.New("know-how actor")
	}
	id, role := value(a, "id"), value(a, "role")
	if id.Kind != wire.KindString || role.Kind != wire.KindString || (role.Str != "OWNER" && role.Str != "OPERATOR") {
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
