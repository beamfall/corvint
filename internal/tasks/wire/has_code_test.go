package wire

import "testing"

// TestTCDV0965_HasCode: HasCode reports a carried code and is false for an
// absent code, an uncoded result and a nil result.
func TestTCDV0965_HasCode(t *testing.T) {
	r := &Result{Outcome: OutcomeRefused, Codes: []string{CodeStaleTicket, CodeMalformed}}
	if !HasCode(r, CodeMalformed) || HasCode(r, CodeFenced) || HasCode(&Result{Outcome: OutcomeOK}, CodeMalformed) || HasCode(nil, CodeMalformed) {
		t.Fatalf("HasCode disagrees with %v", r.Codes)
	}
}
