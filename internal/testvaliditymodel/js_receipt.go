package testvaliditymodel

import (
	"github.com/Beamfall/corvint/internal/jstestreceipt"
	"github.com/Beamfall/corvint/internal/rubytestreceipt"
)

// JSReceipt returns the decoded JS provider receipt, or false for another provider input. The
// teardown-dependents join reads it (TSA-V0-117).
func (input Input) JSReceipt() (*jstestreceipt.Receipt, bool) {
	return input.JS, input.JS != nil
}

// RubyDocument returns the decoded Ruby provider document, or false for another provider input.
// The example receipt join reads it (TSA-V0-410).
func (input Input) RubyDocument() (*rubytestreceipt.Document, bool) {
	return input.Ruby, input.Ruby != nil
}
