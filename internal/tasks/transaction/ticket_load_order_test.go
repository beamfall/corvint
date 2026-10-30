package transaction

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// serialLoadTickets is loadTickets as it was before LGL-V0-014: one record
// at a time, the byte bound before each decode.
func serialLoadTickets(r Request, in Input, queueID wire.QueueID) (*ticket.Inventory, error) {
	records := make([]*ticket.Record, 0, len(in.CanonicalTickets))
	var total uint64
	for _, raw := range in.CanonicalTickets {
		if uint64(len(raw)) > wire.MaxIntentTreeBytes-total {
			return nil, limit("canonical inventory bytes")
		}
		total += uint64(len(raw))
		rec, e := loadTicket(r, in, raw)
		if e != nil {
			return nil, e
		}
		records = append(records, rec)
	}
	inv, e := ticket.NewInventory(queueID, records)
	if e != nil {
		return nil, e
	}
	if countTicketProjections(in) != len(records) {
		return nil, malformed("incomplete canonical ticket inventory")
	}
	return inv, nil
}

// TestLGLV0014_ConcurrentTicketLoadKeepsSerialRefusalOrder covers
// LGL-V0-014 (proposed). An inventory above the serial threshold is decoded
// on several goroutines, and loadTickets returns the same inventory, or the
// same first refusal in record order, as the serial walk: a malformed record
// before a divergent one, a divergent record before a malformed one, the
// cumulative byte bound after valid records, and a malformed record before
// the record that would exceed the bound.
func TestLGLV0014_ConcurrentTicketLoadKeepsSerialRefusalOrder(t *testing.T) {
	const n = 3 * loadTicketsSerialBelow
	base, e := NewInventory(nil, nil)
	if e != nil {
		t.Fatal(e)
	}
	in := Input{Inventory: base.clone()}
	for k := 0; k < n; k++ {
		c := fixture.Ticket(fmt.Sprintf("T-%d", k+1))
		raw := c.Encode()
		path := "intent/tickets/" + c.TicketID.Local + ".json"
		in.Inventory.files[path] = bytesEntry(path, raw)
		in.CanonicalTickets = append(in.CanonicalTickets, raw)
	}
	r := Request{Operation: Pause, QueueID: fixture.QueueID}
	q, e := intent.DecodeQueue(fixture.QueueBytes())
	if e != nil {
		t.Fatal(e)
	}
	queueID := q.QueueID
	malformedAt := func(in Input, i int) Input {
		in.CanonicalTickets = append([][]byte(nil), in.CanonicalTickets...)
		in.CanonicalTickets[i] = bytes.TrimSuffix(in.CanonicalTickets[i], []byte("\n"))
		return in
	}
	divergentAt := func(in Input, i int) Input {
		files := in.Inventory.clone()
		path := fmt.Sprintf("intent/tickets/T-%d.json", i+1)
		files.files[path] = bytesEntry(path, []byte("{}\n"))
		in.Inventory = files
		return in
	}
	overBoundAt := func(in Input, i int) Input {
		in.CanonicalTickets = append([][]byte(nil), in.CanonicalTickets...)
		in.CanonicalTickets[i] = make([]byte, wire.MaxIntentTreeBytes+1)
		return in
	}
	cases := map[string]struct {
		in     Input
		detail string
	}{
		"valid":                                 {in, ""},
		"malformed-before-divergent":            {divergentAt(malformedAt(in, 70), 150), "MALFORMED"},
		"divergent-before-malformed":            {malformedAt(divergentAt(in, 70), 150), "unselected divergent ticket"},
		"bound-after-valid-records":             {overBoundAt(in, 100), "canonical inventory bytes"},
		"malformed-before-bound":                {overBoundAt(malformedAt(in, 90), 120), "MALFORMED"},
		"early-divergent-before-late-malformed": {divergentAt(malformedAt(in, 170), 10), "unselected divergent ticket"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			want, wantErr := serialLoadTickets(r, tc.in, queueID)
			got, gotErr := loadTickets(r, tc.in, queueID)
			if fmt.Sprint(gotErr) != fmt.Sprint(wantErr) {
				t.Fatalf("refusal %v, serial %v", gotErr, wantErr)
			}
			if tc.detail == "" {
				if gotErr != nil || got.Len() != n || want.Len() != n || fmt.Sprint(got.Sorted()) != fmt.Sprint(want.Sorted()) {
					t.Fatalf("inventory %v %v", gotErr, got)
				}
				for _, id := range want.IDs() {
					a, _ := got.Get(id)
					b, _ := want.Get(id)
					if !bytes.Equal(a.Encode(), b.Encode()) {
						t.Fatalf("record %s differs", id)
					}
				}
				return
			}
			if gotErr == nil || !bytes.Contains([]byte(gotErr.Error()), []byte(tc.detail)) {
				t.Fatalf("refusal %v, want %q", gotErr, tc.detail)
			}
		})
	}
}
