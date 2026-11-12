package jstestprovider

import "github.com/Beamfall/corvint/internal/jstestreceipt"

// QualifiedReceiptBindingReady is jstestreceipt.QualifiedReceiptBindingReady, kept under its old name.
func QualifiedReceiptBindingReady(r Receipt, t TestOutcome) bool {
	return jstestreceipt.QualifiedReceiptBindingReady(r, t)
}

// QualifiedApplicationRevision is jstestreceipt.QualifiedApplicationRevision, kept under its old name.
func QualifiedApplicationRevision(r Receipt, t TestOutcome) (string, bool) {
	return jstestreceipt.QualifiedApplicationRevision(r, t)
}
