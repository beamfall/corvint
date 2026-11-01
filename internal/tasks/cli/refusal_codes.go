package cli

import (
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// derivedRefusalCodes maps a refusal outcome recorded without codes to the
// closed §11 code the envelope reports (TCL-V0-870). UNAUTHORIZED has no code
// of its own and reports MALFORMED, as wire.CodeOf does for an uncoded error.
var derivedRefusalCodes = map[string]string{
	mutation.OutcomeRevisionConflict:  wire.CodeStaleTicket,
	mutation.OutcomeValidationFailed:  wire.CodeMalformed,
	mutation.OutcomeUnauthorized:      wire.CodeMalformed,
	mutation.OutcomeBlocked:           wire.CodeTicketState,
	mutation.OutcomeUnsupported:       wire.CodeUnsupported,
	mutation.OutcomeCapacityExhausted: wire.CodeResourceExhausted,
	mutation.OutcomeUncertainEffect:   wire.CodeUncertainEffect,
	mutation.OutcomeRequestIDConflict: wire.CodeRequestIDConflict,
	mutation.OutcomeStorageFailed:     wire.CodeUnsupportedFilesystem,
}

// refusalCodes is the envelope codes of a refused outcome: its recorded
// codes, or the one code derived from the outcome when it recorded none.
// Receipts keep the recorded codes unchanged.
func refusalCodes(o mutation.Outcome) []string {
	if len(o.Codes) > 0 || o.Outcome == mutation.OutcomeCompleted {
		return o.Codes
	}
	if c, ok := derivedRefusalCodes[o.Outcome]; ok {
		return []string{c}
	}
	return []string{wire.CodeMalformed}
}
