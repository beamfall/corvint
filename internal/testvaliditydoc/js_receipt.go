package testvaliditydoc

import (
	"github.com/Beamfall/corvint/internal/jstestprovider"
	"github.com/Beamfall/corvint/internal/rubytestprovider"
)

// JSReceipt returns the decoded JS provider receipt, or false for another provider input. The
// teardown-dependents join reads it (TSA-V0-117).
func (input Input) JSReceipt() (*jstestprovider.Receipt, bool) {
	return input.js, input.js != nil
}

// RubyDocument returns the decoded Ruby provider document, or false for another provider input.
// The example receipt join reads it (TSA-V0-410).
func (input Input) RubyDocument() (*rubytestprovider.Document, bool) {
	return input.ruby, input.ruby != nil
}
