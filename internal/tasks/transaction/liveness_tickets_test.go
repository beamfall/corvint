package transaction

import (
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func livenessLease(verb string) Request {
	r := admin(Lease, "liveness-"+strings.ToLower(verb))
	r.Lease = &LeaseRequest{Verb: verb}
	return r
}

// named returns in with CanonicalTicketPaths set, as the lease routes supply
// it, naming each canonical record's projection.
func named(in Input, paths ...string) Input {
	in.CanonicalTicketPaths = paths
	return in
}

// sameRefusal runs the liveness binding and the decoding route (PAUSE) on
// the same input and requires the same refusal from both: code and message,
// with detail in the message.
func sameRefusal(t *testing.T, in Input, code, detail string) {
	t.Helper()
	_, live := validateInput(livenessLease(LeaseHeartbeat), in)
	_, full := validateInput(admin(Pause, "pause-ref"), in)
	if live == nil || full == nil || wire.CodeOf(live) != code || live.Error() != full.Error() || !strings.Contains(live.Error(), detail) {
		t.Fatalf("%s: liveness %v, decoding route %v, want the same %s refusal mentioning %q", detail, live, full, code, detail)
	}
}

// TestCALV0233_LivenessLeaseBindsTicketsWithoutDecoding covers CAL-V0-233: a
// HEARTBEAT or RENEW whose caller names each canonical record's projection
// binds the set to those intent/tickets/ projections by digest and length
// and decodes no record, so its validated state carries no ticket
// inventory, while every other verb and operation, and a liveness verb
// whose caller names no paths, still decodes the set. The binding gives the
// decoding route's refusals, code and message alike: a divergent, swapped
// or missing projection and an extra projection are MALFORMED, a repeated
// record or a case-folded identity collision is DUPLICATE_ID, a divergent
// record is refused before a repeated one, and a refused record that does
// not decode gives its decode error. Mutation: binding by a digest multiset
// instead of by path accepts the swapped projections.
func TestCALV0233_LivenessLeaseBindsTicketsWithoutDecoding(t *testing.T) {
	in, _ := initialized(t)
	rec := fixture.Ticket("T-1")
	path := "intent/tickets/T-1.json"
	bound := named(withTicket(t, in, rec, rec.Encode()), path)
	for _, verb := range []string{LeaseHeartbeat, LeaseRenew} {
		st, e := validateInput(livenessLease(verb), bound)
		if e != nil {
			t.Fatalf("%s bound: %v", verb, e)
		}
		if st.tickets != nil {
			t.Fatalf("%s decoded the ticket inventory", verb)
		}
	}
	st, e := validateInput(admin(Pause, "pause"), bound)
	if e != nil || st.tickets == nil {
		t.Fatalf("PAUSE bound: %v, inventory %v", e, st.tickets)
	}
	unnamed := bound
	unnamed.CanonicalTicketPaths = nil
	if st, e := validateInput(livenessLease(LeaseHeartbeat), unnamed); e != nil || st.tickets == nil {
		t.Fatalf("HEARTBEAT without named paths: %v, inventory %v, want the decoding route", e, st.tickets)
	}

	// Undecodable bytes that are byte-identical to their named projection
	// bind without decoding; the decoding verbs refuse them.
	raw := []byte("{")
	opaque := in
	opaque.Inventory = in.Inventory.clone()
	opaque.Inventory.files[path] = bytesEntry(path, raw)
	opaque.CanonicalTickets = [][]byte{raw}
	opaque = named(opaque, path)
	for _, verb := range []string{LeaseHeartbeat, LeaseRenew} {
		st, e := validateInput(livenessLease(verb), opaque)
		if e != nil || st.tickets != nil {
			t.Fatalf("%s opaque: %v, inventory %v", verb, e, st.tickets)
		}
	}
	for _, r := range []Request{admin(Pause, "pause-opaque"), livenessLease(LeaseClaim)} {
		if _, e := validateInput(r, opaque); e == nil {
			t.Fatalf("%s %s accepted an undecodable canonical record", r.Operation, r.RequestID)
		}
	}

	divergent := named(withTicket(t, in, rec, append(rec.Encode(), '\n')), path)
	sameRefusal(t, divergent, wire.CodeMalformed, "unselected divergent ticket")

	other := fixture.Ticket("T-2")
	otherPath := "intent/tickets/T-2.json"
	swapped := withTicket(t, in, rec, other.Encode())
	swapped.Inventory.files[otherPath] = bytesEntry(otherPath, rec.Encode())
	swapped.CanonicalTickets = [][]byte{rec.Encode(), other.Encode()}
	swapped = named(swapped, path, otherPath)
	sameRefusal(t, swapped, wire.CodeMalformed, "unselected divergent ticket")

	missing := named(in, path)
	missing.CanonicalTickets = [][]byte{rec.Encode()}
	sameRefusal(t, missing, wire.CodeMalformed, "canonical ticket has no physical projection")

	extra := withTicket(t, in, rec, rec.Encode())
	extra.Inventory.files[otherPath] = bytesEntry(otherPath, other.Encode())
	sameRefusal(t, named(extra, path), wire.CodeMalformed, "incomplete canonical ticket inventory")

	repeated := withTicket(t, in, rec, rec.Encode())
	repeated.CanonicalTickets = [][]byte{rec.Encode(), rec.Encode()}
	sameRefusal(t, named(repeated, path, path), wire.CodeDuplicateID, "duplicate ticket "+rec.TicketID.Raw)

	folded := fixture.Ticket("t-1")
	foldedPath := "intent/tickets/t-1.json"
	collision := withTicket(t, in, rec, rec.Encode())
	collision.Inventory.files[foldedPath] = bytesEntry(foldedPath, folded.Encode())
	collision.CanonicalTickets = [][]byte{rec.Encode(), folded.Encode()}
	sameRefusal(t, named(collision, path, foldedPath), wire.CodeDuplicateID, "collide under ASCII case folding")

	// The decoding route refuses every record's projection before it finds
	// a repeated identity; so does the binding.
	ordered := withTicket(t, in, rec, rec.Encode())
	ordered.Inventory.files[otherPath] = bytesEntry(otherPath, append(other.Encode(), '\n'))
	ordered.CanonicalTickets = [][]byte{rec.Encode(), rec.Encode(), other.Encode()}
	sameRefusal(t, named(ordered, path, path, otherPath), wire.CodeMalformed, "unselected divergent ticket")

	undecodable := named(in, path)
	undecodable.CanonicalTickets = [][]byte{raw}
	_, live := validateInput(livenessLease(LeaseHeartbeat), undecodable)
	_, full := validateInput(admin(Pause, "pause-undecodable"), undecodable)
	if live == nil || full == nil || live.Error() != full.Error() {
		t.Fatalf("undecodable canonical record with no projection: liveness %v, decoding route %v, want the same decode error", live, full)
	}

	short := named(repeated, path)
	if _, e := validateInput(livenessLease(LeaseRenew), short); e == nil || wire.CodeOf(e) != wire.CodeMalformed || !strings.Contains(e.Error(), "canonical ticket paths") {
		t.Fatalf("paths that do not name every record: %v, want MALFORMED", e)
	}
}
