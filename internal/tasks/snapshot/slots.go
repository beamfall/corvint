package snapshot

import (
	"sort"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// MaxSlotEntries bounds the capacity-slot holders and waiters pools.json
// records across every slots resource (PSR-V0-031).
const MaxSlotEntries = 512

// Slot entry states (PSR-V0-031).
const (
	SlotHeld    = "HELD"
	SlotWaiting = "WAITING"
)

// SlotEntry is one attempt's hold on, or wait for, slots of one declared
// capacity resource (PSR-V0-031). It binds no member, so ending it never
// quarantines anything.
type SlotEntry struct {
	SlotsID    string
	AttemptID  string
	Generation wire.Size
	Holder     string
	Count      wire.Count
	Exclusive  bool
	// Class is the reservation class label, or "" for an unclassed request.
	Class    string
	Priority string
	Seq      wire.Size
	State    string
}

// SortSlotEntries orders entries by resource, then request sequence, then
// attempt id: the canonical pools.json order and the FIFO tie-break.
func SortSlotEntries(entries []SlotEntry) {
	sort.SliceStable(entries, func(i, j int) bool {
		a, b := entries[i], entries[j]
		if a.SlotsID != b.SlotsID {
			return a.SlotsID < b.SlotsID
		}
		if a.Seq.Uint64() != b.Seq.Uint64() {
			return a.Seq.Uint64() < b.Seq.Uint64()
		}
		return a.AttemptID < b.AttemptID
	})
}

func readSlotEntries(r *wire.Reader, q wire.QueueID) []SlotEntry {
	out := []SlotEntry{}
	seen := map[string]bool{}
	for _, x := range r.Array(MaxSlotEntries, true) {
		x.Closed("slotsId", "attemptId", "generation", "holder", "count", "exclusive", "class", "priority", "seq", "state")
		en := SlotEntry{SlotsID: x.Field("slotsId").Label(), AttemptID: x.Field("attemptId").Identifier(), Generation: x.Field("generation").Size(), Holder: x.Field("holder").Label(), Count: x.Field("count").Count(), Exclusive: x.Field("exclusive").Bool(), Class: x.Field("class").String(), Priority: x.Field("priority").Enum("P0", "P1", "P2", "P3"), Seq: x.Field("seq").Size(), State: x.Field("state").Enum(SlotHeld, SlotWaiting)}
		if x.Err() != nil {
			return out
		}
		if en.Class != "" {
			x.Field("class").Label()
		}
		if aq, e := AttemptQueue(en.AttemptID); e != nil || aq.Raw != q.Raw {
			x.Fail(wire.CodeMalformed, "invalid slot attempt")
		}
		if n := en.Count.Int(); n < 1 || n > intent.MaxSlotCapacity || en.Seq.Uint64() == 0 {
			x.Fail(wire.CodeMalformed, "invalid slot count or sequence")
		}
		key := en.SlotsID + "\x00" + en.AttemptID
		if seen[key] {
			x.Fail(wire.CodeDuplicateID, "duplicate slot entry")
		}
		seen[key] = true
		out = append(out, en)
	}
	if r.Err() == nil && len(out) == 0 {
		r.Fail(wire.CodeMalformed, "slots must be non-empty when present")
	}
	return out
}

func slotEntriesValue(entries []SlotEntry) wire.Value {
	sorted := append([]SlotEntry{}, entries...)
	SortSlotEntries(sorted)
	values := []wire.Value{}
	for _, e := range sorted {
		values = append(values, wire.ObjectValue(wire.NewObject().Set("slotsId", wire.String(e.SlotsID)).Set("attemptId", wire.String(e.AttemptID)).Set("generation", wire.String(string(e.Generation))).Set("holder", wire.String(e.Holder)).Set("count", wire.String(string(e.Count))).Set("exclusive", wire.Bool(e.Exclusive)).Set("class", wire.String(e.Class)).Set("priority", wire.String(e.Priority)).Set("seq", wire.String(string(e.Seq))).Set("state", wire.String(e.State))))
	}
	return wire.Array(values...)
}
