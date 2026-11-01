package store

import "github.com/Beamfall/corvint/internal/tasks/transaction"

// skipsStagingSync reports a lease write whose session makes no
// staging-directory sync call: heartbeat and renew (LGL-V0-005) and a claim
// (LGL-V0-780). Each keeps every file and destination-directory sync, and a
// redo of another write's pending posts keeps every sync.
func skipsStagingSync(request transaction.Request) bool {
	if transaction.LivenessLease(request) {
		return true
	}
	return request.Operation == transaction.Lease && request.Lease != nil &&
		(request.Lease.Verb == transaction.LeaseClaim || request.Lease.Verb == transaction.LeaseClaimNext)
}
