package intent_test

import (
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
)

// TestTCDV0154_PolicyCIEvidence: ciEvidence is optional (absent is the
// rollback: no gate accepts CI evidence); each entry names a declared
// COMMAND gate once, a GitHub owner/name repository and a non-empty check
// list; anything else is refused.
func TestTCDV0154_PolicyCIEvidence(t *testing.T) {
	if p, err := intent.DecodePolicy(fixture.PolicyBytes()); err != nil || p.CIEvidence != nil || p.CIEvidenceGate("verify") != nil {
		t.Fatalf("absent ciEvidence: %v", err)
	}
	base, err := policyWith(t, "ciEvidence", `{"gates":[{"gateId":"verify","repository":"beamfall/corvint","requiredChecks":["go-test","vet"]}]}`)
	if err != nil {
		t.Fatal(err)
	}
	g := base.CIEvidenceGate("verify")
	if g == nil || g.Repository != "beamfall/corvint" || strings.Join(g.RequiredChecks, ",") != "go-test,vet" || base.CIEvidenceGate("lint") != nil {
		t.Fatalf("declared: %+v", g)
	}
	for name, bad := range map[string]string{
		"undeclared gate":  `{"gates":[{"gateId":"lint","repository":"beamfall/corvint","requiredChecks":["vet"]}]}`,
		"duplicate gate":   `{"gates":[{"gateId":"verify","repository":"beamfall/corvint","requiredChecks":["vet"]},{"gateId":"verify","repository":"beamfall/corvint","requiredChecks":["vet"]}]}`,
		"bad repository":   `{"gates":[{"gateId":"verify","repository":"https://github.com/beamfall/corvint","requiredChecks":["vet"]}]}`,
		"no checks":        `{"gates":[{"gateId":"verify","repository":"beamfall/corvint","requiredChecks":[]}]}`,
		"unknown field":    `{"gates":[{"gateId":"verify","repository":"beamfall/corvint","requiredChecks":["vet"],"branch":"main"}]}`,
		"unsorted checks":  `{"gates":[{"gateId":"verify","repository":"beamfall/corvint","requiredChecks":["vet","go-test"]}]}`,
		"duplicate checks": `{"gates":[{"gateId":"verify","repository":"beamfall/corvint","requiredChecks":["vet","vet"]}]}`,
	} {
		if p, err := policyWith(t, "ciEvidence", bad); err == nil {
			t.Errorf("%s decoded: %+v", name, p.CIEvidence)
		}
	}
}
