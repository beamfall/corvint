package cli

import (
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestTCLV0870_EveryRefusalOutcomeHasACode (TCL-V0-870, V1-3552): every
// refusal outcome recorded without codes reports exactly one closed §11 code,
// recorded codes pass through unchanged, and COMPLETED reports none.
func TestTCLV0870_EveryRefusalOutcomeHasACode(t *testing.T) {
	for _, outcome := range mutation.Outcomes {
		got := refusalCodes(mutation.Outcome{Outcome: outcome})
		if outcome == mutation.OutcomeCompleted {
			if len(got) != 0 {
				t.Errorf("COMPLETED reports %v", got)
			}
			continue
		}
		if len(got) != 1 || !wire.IsCode(got[0]) {
			t.Errorf("%s reports %v, want one closed code", outcome, got)
		}
	}
	if got := refusalCodes(mutation.Outcome{Outcome: mutation.OutcomeRevisionConflict}); got[0] != wire.CodeStaleTicket {
		t.Errorf("REVISION_CONFLICT reports %v", got)
	}
	recorded := []string{wire.CodeAttemptLive, wire.CodeTicketState}
	if got := refusalCodes(mutation.Outcome{Outcome: mutation.OutcomeBlocked, Codes: recorded}); len(got) != 2 || got[0] != recorded[0] || got[1] != recorded[1] {
		t.Errorf("recorded codes changed: %v", got)
	}
}
