package transaction

import (
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
)

// PSR-V0-083: replacing a member's identity record drops the records of
// members the current policy no longer declares, so a replaced member never
// pushes the records past the policy member bound.
func TestPSRV0083_IdentityRecordsFollowPolicyMembers(t *testing.T) {
	policy := &intent.Policy{Pools: []intent.Pool{{ID: "db", Members: []string{"a", "c"}}}}
	list := []snapshot.PoolIdentity{{PoolID: "db", MemberID: "a"}, {PoolID: "db", MemberID: "b"}, {PoolID: "old", MemberID: "c"}, {PoolID: "db", MemberID: "c"}}
	got := withIdentity(policy, list, snapshot.PoolIdentity{PoolID: "db", MemberID: "a", ObservedSeq: "9"})
	if len(got) != 2 || got[0].MemberID != "c" || got[0].PoolID != "db" || got[1].MemberID != "a" || got[1].ObservedSeq != "9" {
		t.Fatalf("identities %+v", got)
	}
}
