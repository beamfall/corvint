// SPDX-License-Identifier: Apache-2.0

package transaction

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// indexedInput is an inventory of n tickets with an index entry for each
// one's exact bytes; every entry's summary carries a sentinel title, so a
// summary taken from the index is told apart from one derived by decoding.
func indexedInput(t *testing.T, n int) (Input, wire.QueueID) {
	t.Helper()
	base, e := NewInventory(nil, nil)
	if e != nil {
		t.Fatal(e)
	}
	in := Input{Inventory: base.clone(), TicketIndex: map[wire.Digest]*intent.ClosedEntry{}}
	for k := 0; k < n; k++ {
		rec := fixture.Ticket(fmt.Sprintf("T-%d", k+1))
		raw := rec.Encode()
		path := "intent/tickets/" + rec.TicketID.Local + ".json"
		in.Inventory.files[path] = bytesEntry(path, raw)
		in.CanonicalTickets = append(in.CanonicalTickets, raw)
		summary := ticket.SummaryOf(rec)
		summary.Title = "indexed " + rec.TicketID.Local
		in.TicketIndex[wire.Sum(raw)] = &intent.ClosedEntry{Size: int64(len(raw)), Sha: wire.Sum(raw), Summary: summary}
	}
	q, e := intent.DecodeQueue(fixture.QueueBytes())
	if e != nil {
		t.Fatal(e)
	}
	return in, q.QueueID
}

func tid(local string) string { return fixture.Ticket(local).TicketID.Raw }

func fromIndex(inv *ticket.Inventory, local string) bool {
	s, ok := inv.Summary(tid(local))
	return ok && strings.HasPrefix(s.Title, "indexed ")
}

// TestTCXV0846ModelSkipsOnlyByteIdenticalIndexedTickets: the model holds a
// non-target ticket as its entry's summary only for bytes whose digest and
// length equal the entry's, keeps every projection refusal, decodes the
// target and every unmatched ticket, materializes the same record as a full
// decode, and reports an indexed record that does not decode as a
// MaterializeError the model turns into a refusal.
func TestTCXV0846ModelSkipsOnlyByteIdenticalIndexedTickets(t *testing.T) {
	const n = 3 * loadTicketsSerialBelow
	in, queueID := indexedInput(t, n)
	r := Request{Operation: Pause, QueueID: fixture.QueueID}

	inv, err := loadTickets(r, in, queueID)
	if err != nil || inv.Len() != n {
		t.Fatalf("indexed load: %v %v", inv, err)
	}
	full, err := loadTickets(r, Input{Inventory: in.Inventory, CanonicalTickets: in.CanonicalTickets}, queueID)
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(inv.Sorted()) != fmt.Sprint(full.Sorted()) {
		t.Fatal("indexed planning order differs from the full decode")
	}
	for k := 0; k < n; k++ {
		local := fmt.Sprintf("T-%d", k+1)
		if !fromIndex(inv, local) {
			t.Fatalf("%s was decoded although its bytes equal its entry", local)
		}
		a, _ := inv.Get(tid(local))
		b, _ := full.Get(tid(local))
		if !bytes.Equal(a.Encode(), b.Encode()) || a.Title == "indexed "+local {
			t.Fatalf("%s materialized differently from the full decode", local)
		}
	}

	// The target is always decoded.
	target := Request{Operation: Pause, QueueID: fixture.QueueID, TargetID: tid("T-7")}
	if inv, err = loadTickets(target, in, queueID); err != nil || fromIndex(inv, "T-7") || !fromIndex(inv, "T-8") {
		t.Fatalf("target load: %v, T-7 indexed=%v", err, fromIndex(inv, "T-7"))
	}

	// An entry of another length is not used.
	wrong := in
	wrong.TicketIndex = map[wire.Digest]*intent.ClosedEntry{}
	for k, v := range in.TicketIndex {
		copied := *v
		if v.Summary.TicketID.Local == "T-9" {
			copied.Size++
		}
		wrong.TicketIndex[k] = &copied
	}
	if inv, err = loadTickets(r, wrong, queueID); err != nil || fromIndex(inv, "T-9") {
		t.Fatalf("size mismatch: %v, T-9 indexed=%v", err, fromIndex(inv, "T-9"))
	}

	// A divergent unselected projection is refused exactly as without the index.
	divergent := in
	divergent.Inventory = in.Inventory.clone()
	divergent.Inventory.files["intent/tickets/T-5.json"] = bytesEntry("intent/tickets/T-5.json", []byte("{}\n"))
	_, want := loadTickets(r, Input{Inventory: divergent.Inventory, CanonicalTickets: in.CanonicalTickets}, queueID)
	if _, got := loadTickets(r, divergent, queueID); want == nil || fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("divergent projection with the index: %v, without: %v", got, want)
	}

	// Canonical bytes that differ from every entry are decoded and refused as before.
	broken := in
	broken.CanonicalTickets = append([][]byte(nil), in.CanonicalTickets...)
	broken.CanonicalTickets[3] = bytes.TrimSuffix(broken.CanonicalTickets[3], []byte("\n"))
	_, want = loadTickets(r, Input{Inventory: in.Inventory, CanonicalTickets: broken.CanonicalTickets}, queueID)
	if _, got := loadTickets(r, broken, queueID); want == nil || fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("noncanonical bytes with the index: %v, without: %v", got, want)
	}

	// A summary whose status differs from the bytes cannot materialize.
	lying := in
	lying.TicketIndex = map[wire.Digest]*intent.ClosedEntry{}
	for k, v := range in.TicketIndex {
		copied := *v
		if v.Summary.TicketID.Local == "T-2" {
			s := *v.Summary
			s.Status = ticket.StatusCompleted
			copied.Summary = &s
		}
		lying.TicketIndex[k] = &copied
	}
	inv, err = loadTickets(r, lying, queueID)
	if err != nil {
		t.Fatal(err)
	}
	func() {
		defer func() {
			if _, ok := recover().(ticket.MaterializeError); !ok {
				t.Fatal("a summary that differs from its bytes did not raise MaterializeError")
			}
		}()
		inv.Get(tid("T-2"))
	}()
}
