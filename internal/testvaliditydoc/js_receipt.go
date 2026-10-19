package testvaliditydoc

import "github.com/Beamfall/corvint/internal/jstestprovider"

// JSReceipt returns the decoded JS provider receipt, or false for another provider input. The
// teardown-dependents join reads it (TSA-V0-117).
func (input Input) JSReceipt() (*jstestprovider.Receipt, bool) {
	return input.js, input.js != nil
}
