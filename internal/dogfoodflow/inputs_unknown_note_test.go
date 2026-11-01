package dogfoodflow

import (
	"strings"
	"testing"
)

// TestDCWV0721InputsCheckNamesThePlansUnknownRows (V1-4106): a plan that
// keeps a hunk unknown with a recorded reason passes inputs-check, and the
// pass names the unknown rows by reason; a fully cited plan prints no NOTE.
func TestDCWV0721InputsCheckNamesThePlansUnknownRows(t *testing.T) {
	root, base := preflightRepository(t, nil)
	plan := writeInput(t, "1\tpre.go\t3:3\timplementation\n2\tunknown\tinsufficient-evidence\tno test pins the second hunk yet\n")
	code, stdout, stderr, _ := preflightRun(t, root, ChangeOptions{Base: base, Citations: plan})
	if code != 0 || !strings.Contains(stdout, "inputs-check PASS") {
		t.Fatalf("code=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "inputs-check NOTE unknown-rows=1 reasons=insufficient-evidence:1: admitted by design") {
		t.Errorf("stdout lacks the unknown-rows NOTE:\n%s", stdout)
	}
	cited := writeInput(t, "1\tpre.go\t3:3\timplementation\n2\tpre.go\t20:20\timplementation\n")
	_, stdout, _, _ = preflightRun(t, root, ChangeOptions{Base: base, Citations: cited})
	if strings.Contains(stdout, "unknown-rows=") {
		t.Errorf("a fully cited plan prints the NOTE:\n%s", stdout)
	}
}
