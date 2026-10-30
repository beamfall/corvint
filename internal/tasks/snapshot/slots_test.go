package snapshot

import (
	"bytes"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func slotEntry(slots, attempt, seq, state string) SlotEntry {
	return SlotEntry{SlotsID: slots, AttemptID: attempt, Generation: "1", Holder: "builder", Count: "1", Class: "", Priority: "P2", Seq: wire.Size(seq), State: state}
}

// TestPSRV0031_SlotEntriesCodec: the pools.json slots key is additive.
// State without slots encodes exactly as before; slot entries round-trip in
// canonical (slotsId, seq, attemptId) order; and malformed entries refuse.
func TestPSRV0031_SlotEntriesCodec(t *testing.T) {
	q, err := wire.ParseQueueID("queueId", "queue:acme:main")
	if err != nil {
		t.Fatal(err)
	}
	legacy, err := (&PoolState{QueueID: q, Entries: []PoolEntry{}}).Encode()
	if err != nil || bytes.Contains(legacy, []byte(`"slots"`)) {
		t.Fatalf("legacy pools %v %s", err, legacy)
	}
	other := "attempt:acme:main:fedcba9876543210fedcba9876543210"
	held := slotEntry("gpu", shareSource, "4", SlotHeld)
	held.Class = "ci"
	waiting := slotEntry("gpu", other, "3", SlotWaiting)
	waiting.Exclusive, waiting.Count = true, "4"
	raw, err := (&PoolState{QueueID: q, Entries: []PoolEntry{}, Slots: []SlotEntry{held, waiting}}).Encode()
	if err != nil {
		t.Fatal(err)
	}
	p, err := DecodePools(raw)
	if err != nil || len(p.Slots) != 2 || p.Slots[0] != waiting || p.Slots[1] != held {
		t.Fatalf("decode %v %+v", err, p)
	}
	if again, _ := p.Encode(); !bytes.Equal(raw, again) {
		t.Fatalf("round trip\n%s\n%s", raw, again)
	}
	mutate := func(f func(slots *wire.Value)) []byte {
		v, err := wire.Parse(raw)
		if err != nil {
			t.Fatal(err)
		}
		s, _ := v.Obj.Get("slots")
		f(&s)
		v.Obj.Set("slots", s)
		return wire.EncodeFile(v)
	}
	entry := func(s *wire.Value) *wire.Object { return s.Arr[0].Obj }
	for name, tc := range map[string]struct {
		edit func(*wire.Value)
		code string
	}{
		"empty":         {func(s *wire.Value) { *s = wire.Array() }, wire.CodeMalformed},
		"unknown key":   {func(s *wire.Value) { entry(s).Set("ttl", wire.String("1")) }, wire.CodeMalformed},
		"zero count":    {func(s *wire.Value) { entry(s).Set("count", wire.String("0")) }, wire.CodeMalformed},
		"count over 64": {func(s *wire.Value) { entry(s).Set("count", wire.String("65")) }, wire.CodeMalformed},
		"zero seq":      {func(s *wire.Value) { entry(s).Set("seq", wire.String("0")) }, wire.CodeMalformed},
		"bad state":     {func(s *wire.Value) { entry(s).Set("state", wire.String("QUARANTINED")) }, wire.CodeMalformed},
		"bad priority":  {func(s *wire.Value) { entry(s).Set("priority", wire.String("P9")) }, wire.CodeMalformed},
		"bad class":     {func(s *wire.Value) { entry(s).Set("class", wire.String("a\nb")) }, wire.CodeMalformed},
		"other queue": {func(s *wire.Value) {
			entry(s).Set("attemptId", wire.String("attempt:acme:other:0123456789abcdef0123456789abcdef"))
		}, wire.CodeMalformed},
		"duplicate": {func(s *wire.Value) { entry(s).Set("attemptId", s.Arr[1].Obj.Vals["attemptId"]) }, wire.CodeDuplicateID},
	} {
		if _, err := DecodePools(mutate(tc.edit)); wire.CodeOf(err) != tc.code {
			t.Fatalf("%s: got %v, want %s", name, err, tc.code)
		}
	}
}
