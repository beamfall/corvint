package store

import (
	"time"

	"github.com/Beamfall/corvint/internal/tasks/intent"
)

var ReconcileBeforeCommitForTest = reconcile

var BarrierWithFaultsForTest = barrier

var PolicyUpdateBeforeCommitForTest = policyUpdate

var ImportWithFaultsForTest = importWithFaults

const AuditCacheNameForTest = auditCacheName

// FullAuditsForTest counts the full writer audits taken so far.
func FullAuditsForTest() int64 { return fullAudits.Load() }

// AuditCacheHitForTest reports whether the audit cache validates against the
// store as it is now.
func AuditCacheHitForTest(repo *intent.Repository) bool {
	head, err := readHead(repo)
	return err == nil && cachedView(repo, head) != nil
}

// SetRacyWindowForTest replaces the racy window and returns a restore func.
func SetRacyWindowForTest(d time.Duration) func() {
	old := racyWindow
	racyWindow = d
	return func() { racyWindow = old }
}
