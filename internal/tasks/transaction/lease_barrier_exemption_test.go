package transaction

import (
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/snapshot"
)

// TestTRMV0801_BarrierExemptionAppliesOnlyToOperatorAdmission (V1-3191): the
// explicit exemption passes only an OPERATOR ADMISSION barrier for an OWNER
// or OPERATOR; without the flag, under another reason, or for another role
// the PAUSED detail says why, and with no barrier nothing is refused.
func TestTRMV0801_BarrierExemptionAppliesOnlyToOperatorAdmission(t *testing.T) {
	const remedy = "remedy"
	cases := []struct {
		name, reason, role string
		admit              bool
		want               string
	}{
		{"no barrier", "", "OPERATOR", true, ""},
		{"no flag", "OPERATOR", "OPERATOR", false, remedy},
		{"operator", "OPERATOR", "OPERATOR", true, ""},
		{"owner", "OPERATOR", "OWNER", true, ""},
		{"worker", "OPERATOR", "WORKER", true, "not WORKER"},
		{"reviewer", "OPERATOR", "REVIEWER", true, "not REVIEWER"},
		{"emergency", "EMERGENCY", "OPERATOR", true, "reason EMERGENCY"},
		{"cutover", "CUTOVER", "OWNER", true, "reason CUTOVER"},
		{"drain", "DRAIN", "OPERATOR", true, "reason DRAIN"},
	}
	for _, tc := range cases {
		c := leaseContext{r: admin(Lease, "claim"), l: &LeaseRequest{Verb: LeaseClaim, AdmitUnderBarrier: tc.admit}}
		c.r.Actor.Role = tc.role
		if tc.reason != "" {
			c.st.barrier = &snapshot.Barrier{Scope: "ADMISSION", Reason: tc.reason}
		}
		got := c.barrierRefusal(remedy)
		if (tc.want == "") != (got == "") || !strings.Contains(got, tc.want) {
			t.Fatalf("%s: refusal %q, want %q", tc.name, got, tc.want)
		}
	}
}
