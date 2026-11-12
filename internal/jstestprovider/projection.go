package jstestprovider

import (
	"github.com/Beamfall/corvint/internal/jstestreceipt"
	"github.com/Beamfall/corvint/internal/testvalidity"
)

// ReceiptTestProjection is jstestreceipt.ReceiptTestProjection, kept under its old name.
func ReceiptTestProjection(r Receipt, t TestOutcome) testvalidity.Projection {
	return jstestreceipt.ReceiptTestProjection(r, t)
}

// EncodeQualified is jstestreceipt.EncodeQualified, kept under its old name.
func EncodeQualified(r Receipt) ([]byte, error) { return jstestreceipt.EncodeQualified(r) }

// ToTestProjection is jstestreceipt.ToTestProjection, kept under its old name.
func ToTestProjection(outcome TestOutcome) testvalidity.Projection {
	return jstestreceipt.ToTestProjection(outcome)
}

// ReceiptRunProjection is jstestreceipt.ReceiptRunProjection, kept under its old name.
func ReceiptRunProjection(r Receipt) testvalidity.Projection {
	return jstestreceipt.ReceiptRunProjection(r)
}
