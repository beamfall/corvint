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

func expectMalformed(t *testing.T, r Request, in Input, detail string) {
	t.Helper()
	_, e := validateInput(r, in)
	if e == nil || wire.CodeOf(e) != wire.CodeMalformed || !strings.Contains(e.Error(), detail) {
		t.Fatalf("%s %s: got %v, want MALFORMED %q", r.Operation, r.RequestID, e, detail)
	}
}

// TestCALV0233_LivenessLeaseBindsTicketsWithoutDecoding covers CAL-V0-233: a
// HEARTBEAT or RENEW binds the canonical ticket set to its intent/tickets/
// projections by digest and length and decodes no record, so its validated
// state carries no ticket inventory, while every other verb and operation
// still decodes the set. The binding keeps the same refusals as the decoding
// route: a divergent projection, a missing projection, an extra projection
// and a repeated canonical record are each MALFORMED.
func TestCALV0233_LivenessLeaseBindsTicketsWithoutDecoding(t *testing.T) {
	in, _ := initialized(t)
	rec := fixture.Ticket("T-1")
	path := "intent/tickets/T-1.json"
	bound := withTicket(t, in, rec, rec.Encode())
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

	// Undecodable bytes that are byte-identical to their projection bind
	// without decoding; the decoding verbs refuse them.
	raw := []byte("{")
	opaque := in
	opaque.Inventory = in.Inventory.clone()
	opaque.Inventory.files[path] = bytesEntry(path, raw)
	opaque.CanonicalTickets = [][]byte{raw}
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

	divergent := withTicket(t, in, rec, append(rec.Encode(), '\n'))
	expectMalformed(t, livenessLease(LeaseHeartbeat), divergent, "unselected divergent ticket")

	missing := in
	missing.CanonicalTickets = [][]byte{rec.Encode()}
	expectMalformed(t, livenessLease(LeaseRenew), missing, "canonical ticket has no physical projection")

	extra := withTicket(t, in, rec, rec.Encode())
	extra.Inventory = extra.Inventory.clone()
	other := fixture.Ticket("T-2")
	extra.Inventory.files["intent/tickets/T-2.json"] = bytesEntry("intent/tickets/T-2.json", other.Encode())
	expectMalformed(t, livenessLease(LeaseHeartbeat), extra, "incomplete canonical ticket inventory")

	repeated := withTicket(t, in, rec, rec.Encode())
	repeated.CanonicalTickets = [][]byte{rec.Encode(), rec.Encode()}
	expectMalformed(t, livenessLease(LeaseRenew), repeated, "unselected divergent ticket")

	undecodable := in
	undecodable.CanonicalTickets = [][]byte{raw}
	if _, e := validateInput(livenessLease(LeaseHeartbeat), undecodable); e == nil {
		t.Fatal("HEARTBEAT accepted an undecodable canonical record with no projection")
	}
}
