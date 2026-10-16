package journal

import (
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestCALV0255_AuditAcceptsPostedGateTimeoutBound replays a posted policy
// whose gate declares the 14400-second bound, and refuses one second more.
func TestCALV0255_AuditAcceptsPostedGateTimeoutBound(t *testing.T) {
	for _, tc := range []struct{ seconds, code string }{{"14400", ""}, {"14401", wire.CodeLimitExceeded}} {
		repo, r := setup(t)
		v := value(t, filepath.Join(repo.IntentDir, "policy.json"))
		v.Obj.Set("policyVersion", str("2"))
		gates, _ := v.Obj.Get("gates")
		gates.Arr[0].Obj.Set("timeoutSeconds", str(tc.seconds))
		appendReceipt(t, repo, snapshot.StagePolicyUpdate, map[string][]byte{"intent/policy.json": wire.EncodeFile(v)}, "", true, true, false)
		_, err := r.Audit()
		requireCode(t, err, tc.code)
	}
}
