package ticket_test

import (
	"bytes"
	"strconv"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// mirrorChain folds seed then each SET payload in order, returning the
// ledger after every event and the first apply error.
func mirrorChain(t *testing.T, sets ...string) ([]*ticket.ObligationLedger, error) {
	t.Helper()
	var l *ticket.ObligationLedger
	var out []*ticket.ObligationLedger
	var prev *wire.Digest
	payloads := append([]string{tolSeed}, sets...)
	id, err := wire.ParseTicketID("/ticketId", "ticket:acme:main:AT-01")
	if err != nil {
		t.Fatal(err)
	}
	for i, p := range payloads {
		op := ticket.OpObligationsSet
		if i == 0 {
			op = ticket.OpObligationsSeed
		}
		req := tolRequest(t, "r-"+strconv.Itoa(i), op, strconv.Itoa(i+1), p)
		raw, err := ticket.ObligationEvent{TicketID: id, Revision: wire.CountOf(int64(i + 1)), Previous: prev, RequestSha256: wire.Sum(req), Request: req}.Encode()
		if err != nil {
			return out, err
		}
		ev, err := ticket.DecodeObligationEvent(raw)
		if err != nil {
			return out, err
		}
		h := wire.Sum(raw)
		if l, err = ticket.ApplyObligationEvent(l, ev, h); err != nil {
			return out, err
		}
		prev = &h
		out = append(out, l)
	}
	return out, nil
}

// TestTOLV0120_SatisfiedByRecordedAndCleared: a SET change may carry only
// satisfiedBy; the entry then encodes the member, which round-trips through
// the entry codec, and a null clears it; an entry that never had one
// encodes without the member; a mirror of the ledger's own ticket refuses
// MALFORMED OBLIGATION_MIRROR_SELF:.
func TestTOLV0120_SatisfiedByRecordedAndCleared(t *testing.T) {
	ls, err := mirrorChain(t,
		`{"changes":[{"core":null,"id":"AC-1","reason":"decided on AT-02","satisfiedBy":{"id":"BC-4","ticketId":"ticket:acme:main:AT-02"},"state":null}]}`,
		`{"changes":[{"core":null,"id":"AC-1","reason":"not a mirror","satisfiedBy":null,"state":null}]}`)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(wire.Encode(ls[0].EntriesValue())), "satisfiedBy") {
		t.Fatal("an entry without a mirror encodes satisfiedBy")
	}
	e := ls[1].Entry("AC-1")
	if e.SatisfiedBy == nil || e.SatisfiedBy.TicketID.Raw != "ticket:acme:main:AT-02" || e.SatisfiedBy.ID != "BC-4" || e.State != ticket.ObligationOpen {
		t.Fatalf("mirror = %+v", e)
	}
	enc := wire.Encode(e.Value())
	if !strings.Contains(string(enc), `"satisfiedBy":{"id":"BC-4","ticketId":"ticket:acme:main:AT-02"}`) {
		t.Fatalf("entry encoding: %s", enc)
	}
	r := wire.NewReader(tolParse(t, string(enc)), "/entry")
	if back := ticket.ReadObligationEntry(r); r.Err() != nil || !bytes.Equal(wire.Encode(back.Value()), enc) {
		t.Fatalf("entry does not round-trip: %v", r.Err())
	}
	if ls[2].Entry("AC-1").SatisfiedBy != nil || strings.Contains(string(wire.Encode(ls[2].EntriesValue())), "satisfiedBy") {
		t.Fatal("null did not clear the mirror")
	}
	_, err = mirrorChain(t, `{"changes":[{"core":null,"id":"AC-1","reason":"self","satisfiedBy":{"id":"AC-2","ticketId":"ticket:acme:main:AT-01"},"state":null}]}`)
	if wire.CodeOf(err) != wire.CodeMalformed || !strings.Contains(err.Error(), ticket.ObligationMirrorSelfDetail) {
		t.Fatalf("self mirror: %v", err)
	}
	for name, p := range map[string]string{
		"open member":  `{"changes":[{"core":null,"id":"AC-1","reason":"x","satisfiedBy":{"id":"BC-4","ticketId":"ticket:acme:main:AT-02","x":"1"},"state":null}]}`,
		"bad owner id": `{"changes":[{"core":null,"id":"AC-1","reason":"x","satisfiedBy":{"id":"bc-4","ticketId":"ticket:acme:main:AT-02"},"state":null}]}`,
		"no ticket":    `{"changes":[{"core":null,"id":"AC-1","reason":"x","satisfiedBy":{"id":"BC-4"},"state":null}]}`,
	} {
		if _, err := mirrorChain(t, p); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
}

// TestTOLV0124_UnblocksCounted: the fold counts entries leaving BLOCKED,
// whatever state they move to; entering BLOCKED and other moves do not
// count.
func TestTOLV0124_UnblocksCounted(t *testing.T) {
	ls, err := mirrorChain(t,
		`{"changes":[{"core":null,"id":"AC-1","reason":"waits","state":"BLOCKED"},{"core":null,"id":"AC-2","reason":"waits","state":"BLOCKED"}]}`,
		`{"changes":[{"core":null,"id":"AC-1","reason":"fixture landed","state":"OPEN"}]}`,
		`{"changes":[{"core":null,"id":"AC-1","reason":"bug","state":"DEFECT"},{"core":null,"id":"AC-2","reason":"later","state":"DEFERRED"}]}`)
	if err != nil {
		t.Fatal(err)
	}
	for i, want := range []int64{0, 0, 1, 2} {
		if ls[i].Unblocks != want {
			t.Errorf("after event %d unblocks = %d, want %d", i+1, ls[i].Unblocks, want)
		}
	}
}
