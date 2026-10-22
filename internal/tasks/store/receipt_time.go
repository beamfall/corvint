package store

import (
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// ReceiptRecordedAt returns the writer timestamp of receipt seq. A mutation
// whose --issued-at defaulted records that same second as its receipt's
// recordedAt, so a REQUEST_ID_CONFLICT explanation can recover the original
// issuedAt from the receipt its request-index entry names even after later
// updates moved the ticket's updatedAt (ATR-V0-275, V1-1438). It is a read:
// it never restores or writes a receipt.
func ReceiptRecordedAt(repo *intent.Repository, seq uint64) (wire.Timestamp, error) {
	rc, err := readReceipt(repo, seq)
	if err != nil {
		return "", err
	}
	return rc.RecordedAt, nil
}
