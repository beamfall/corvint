package mutation

import (
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/ticket"
)

// TestTOLV0081_ScopeAcceptOwnerOnly reaches the ACCEPT guard directly: the
// default role matrix already refuses OBLIGATIONS_SET to OPERATOR, so the CLI
// path never gets here, and only this test proves the guard bites if a future
// matrix grants the operation (TOL-V0-081).
func TestTOLV0081_ScopeAcceptOwnerOnly(t *testing.T) {
	accept := &ObligationsPayload{Set: &ticket.ObligationSetPayload{Changes: []ticket.ObligationChange{
		{ID: "AC-1", Scope: &ticket.ObligationScopeChange{Action: ticket.ObligationScopeAccept}}}}}
	propose := &ObligationsPayload{Set: &ticket.ObligationSetPayload{Changes: []ticket.ObligationChange{
		{ID: "AC-1", Scope: &ticket.ObligationScopeChange{Action: ticket.ObligationScopePropose}}}}}
	operator := Context{Binding: Binding{Role: "OPERATOR"}}
	r := operator.obligationAuthority(nil, accept)
	if r == nil || r.outcome != OutcomeUnauthorized || !strings.Contains(r.detail, "TOL-V0-081") {
		t.Fatalf("OPERATOR ACCEPT: %+v", r)
	}
	if r := operator.obligationAuthority(nil, propose); r != nil {
		t.Fatalf("OPERATOR PROPOSE: %+v", r)
	}
	if r := (Context{Binding: Binding{Role: "OWNER"}}).obligationAuthority(nil, accept); r != nil {
		t.Fatalf("OWNER ACCEPT: %+v", r)
	}
}
