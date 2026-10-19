package intent_test

import (
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestTOLV0102_103_107_ObligationsPolicyMembers: requireDeclaredCites,
// testFilePatterns and environmentBaselines are optional obligations members;
// each refuses a malformed declaration (TOL-V0-102, 103, 107). An absent
// requireDeclaredCites requires declared cites and only an explicit false
// opts out (owner decision 2026-10-19, V1-1963).
func TestTOLV0102_103_107_ObligationsPolicyMembers(t *testing.T) {
	p, err := policyWith(t, "obligations", `{"workerWitness":false}`)
	if err != nil || !p.ObligationCitesDeclaredRequired() || p.ObligationTestFilePatterns() != nil {
		t.Fatalf("absent members: %v", err)
	}
	p, err = policyWith(t, "obligations", `{"workerWitness":false,"requireDeclaredCites":false}`)
	if err != nil || p.ObligationCitesDeclaredRequired() {
		t.Fatalf("explicit false: %v", err)
	}
	if _, ok := p.ObligationEnvironmentBaseline("."); ok {
		t.Fatal("absent baseline resolved")
	}
	p, err = policyWith(t, "obligations", `{"workerWitness":false,"requireDeclaredCites":true,`+
		`"testFilePatterns":[{"kind":"controller","pattern":"*_controller_check.rb"},{"kind":"sql","pattern":"db/checks/*.sql"}],`+
		`"environmentBaselines":{".":"refs/heads/env/staging","app":"`+"0123456789abcdef0123456789abcdef01234567"+`"}}`)
	if err != nil || !p.ObligationCitesDeclaredRequired() || len(p.ObligationTestFilePatterns()) != 2 || p.ObligationTestFilePatterns()[1].Kind != "sql" {
		t.Fatalf("declared members: %v", err)
	}
	if ref, ok := p.ObligationEnvironmentBaseline("app"); !ok || ref != "0123456789abcdef0123456789abcdef01234567" {
		t.Fatalf("app baseline: %q %v", ref, ok)
	}
	var many []string
	for i := 0; i < 18; i++ {
		many = append(many, `"r`+string(rune('a'+i))+`":"refs/heads/x"`)
	}
	for _, over := range []string{
		`{"workerWitness":false,"testFilePatterns":[]}`,
		`{"workerWitness":false,"environmentBaselines":{}}`,
		`{"workerWitness":false,"environmentBaselines":{` + strings.Join(many, ",") + `}}`,
	} {
		if _, err := policyWith(t, "obligations", over); wire.CodeOf(err) != wire.CodeLimitExceeded {
			t.Errorf("count bound %s: %v", over, err)
		}
	}
	for _, bad := range []string{
		`{"workerWitness":false,"testFilePatterns":[{"kind":"x","pattern":"["}]}`,
		`{"workerWitness":false,"testFilePatterns":[{"kind":"x","pattern":"/abs/*.sql"}]}`,
		`{"workerWitness":false,"testFilePatterns":[{"kind":"","pattern":"*.sql"}]}`,
		`{"workerWitness":false,"testFilePatterns":[{"kind":"x","pattern":"*.sql","other":"1"}]}`,
		`{"workerWitness":false,"environmentBaselines":{".":"HEAD"}}`,
		`{"workerWitness":false,"environmentBaselines":{".":"main"}}`,
		`{"workerWitness":false,"environmentBaselines":{".":"refs/heads/a..b"}}`,
		`{"workerWitness":false,"environmentBaselines":{".":"0123abc"}}`,
		`{"workerWitness":false,"environmentBaselines":{"":"refs/heads/x"}}`,
		`{"workerWitness":false,"requireDeclaredCites":"yes"}`,
	} {
		if _, err := policyWith(t, "obligations", bad); wire.CodeOf(err) != wire.CodeMalformed {
			t.Errorf("not MALFORMED %s: %v", bad, err)
		}
	}
}
