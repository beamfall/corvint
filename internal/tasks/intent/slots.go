package intent

import (
	"sort"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// Capacity slot bounds (PSR-V0-030).
const (
	MaxSlotResources = 16
	MaxSlotCapacity  = 64
	MaxSlotClasses   = 8
)

// SlotResource is one declared shared-capacity resource (PSR-V0-030): a
// named capacity of interchangeable stateless slots with optional per-class
// reservations. It has no members, health, cleanup or quarantine.
type SlotResource struct {
	ID       string
	Capacity wire.Count
	// Reserved maps a class label to the slots held back for it.
	Reserved map[string]wire.Count
}

// ReservedFor returns the reservation of class, or 0.
func (s SlotResource) ReservedFor(class string) int64 {
	return s.Reserved[class].Int()
}

// Classes returns the reserved class labels in byte order.
func (s SlotResource) Classes() []string {
	out := make([]string, 0, len(s.Reserved))
	for k := range s.Reserved {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// SlotResource returns the declared resource id, or nil.
func (p *Policy) SlotResource(id string) *SlotResource {
	for i := range p.Slots {
		if p.Slots[i].ID == id {
			return &p.Slots[i]
		}
	}
	return nil
}

// readSlots decodes the optional policy "slots" list (PSR-V0-030).
func readSlots(r *wire.Reader) []SlotResource {
	out := []SlotResource{}
	items := r.Array(MaxSlotResources, true)
	if r.Err() == nil && len(items) == 0 {
		r.Fail(wire.CodeMalformed, "slots must be non-empty")
	}
	ids := map[string]bool{}
	for _, x := range items {
		x.Closed(wire.OptionalKeys(x.Value(), []string{"id", "capacity"}, "reserved")...)
		s := SlotResource{ID: x.Field("id").Label(), Capacity: boundCount(x.Field("capacity"), 1, MaxSlotCapacity), Reserved: map[string]wire.Count{}}
		if x.Err() != nil {
			return out
		}
		if ids[s.ID] {
			x.Fail(wire.CodeDuplicateID, "duplicate slots id")
			return out
		}
		ids[s.ID] = true
		if wire.Has(x.Value(), "reserved") {
			rr := x.Field("reserved")
			if rr.Value().Kind != wire.KindObject || len(rr.Value().Obj.Keys) == 0 {
				rr.Fail(wire.CodeMalformed, "reserved must map 1..%d class labels", MaxSlotClasses)
				return out
			}
			if len(rr.Value().Obj.Keys) > MaxSlotClasses {
				rr.Fail(wire.CodeLimitExceeded, "reserved names more than %d classes", MaxSlotClasses)
				return out
			}
			var sum int64
			for _, name := range rr.Value().Obj.Keys {
				cr := rr.Field(name)
				if _, err := wire.ParseLabel("reserved", name); err != nil {
					cr.Fail(wire.CodeMalformed, "reserved class is not a label")
					return out
				}
				n := boundCount(cr, 1, MaxSlotCapacity)
				if cr.Err() != nil {
					return out
				}
				s.Reserved[name] = n
				sum += n.Int()
			}
			if sum > s.Capacity.Int() {
				rr.Fail(wire.CodeLimitExceeded, "reserved slots %d exceed capacity %s", sum, s.Capacity)
				return out
			}
		}
		out = append(out, s)
	}
	return out
}
