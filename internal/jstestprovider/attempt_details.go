package jstestprovider

import (
	"github.com/Beamfall/corvint/internal/jstestreceipt"
)

// AttemptExternalProfile is jstestreceipt.AttemptExternalProfile, kept under its old name.
const AttemptExternalProfile = jstestreceipt.AttemptExternalProfile

// DecodeAttemptReceipt is jstestreceipt.DecodeAttemptReceipt, kept under its old name.
func DecodeAttemptReceipt(raw []byte) (Receipt, error) {
	return jstestreceipt.DecodeAttemptReceipt(raw)
}

// ValidateAttemptDetails is jstestreceipt.ValidateAttemptDetails, kept under its old name.
func ValidateAttemptDetails(tests []TestOutcome) error {
	return jstestreceipt.ValidateAttemptDetails(tests)
}
