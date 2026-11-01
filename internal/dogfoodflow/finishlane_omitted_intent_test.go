package dogfoodflow

import (
	"fmt"
	"strings"
	"testing"
)

// TestDCWV0731InputsCheckNamesAnOmittedOverBoundSpec (V1-2275): a passing
// inputs-check NOTEs a changed spec that derives more obligations than one
// OCM map admits and that no intent or deferred row names, and is silent once
// a deferred row records the omission.
func TestDCWV0731InputsCheckNamesAnOmittedOverBoundSpec(t *testing.T) {
	spec := new(strings.Builder)
	spec.WriteString("# Intent\n\n## Requirements\n\n")
	for requirement := 1; requirement <= 257; requirement++ {
		fmt.Fprintf(spec, "- `BIG-V0-%03d`: keep the bound.\n", requirement)
	}
	spec.WriteString("\n## Non-goals\n")
	root, base := preflightRepository(t, map[string]string{"docs/specs/big.md": spec.String()})
	rows := "1\tpre.go\t5:5\tdependency\n2\tpre.go\t3:3\timplementation\n3\tpre.go\t20:20\timplementation\n"
	note := "inputs-check NOTE intent-over-bound-omitted path=docs/specs/big.md:"
	code, stdout, stderr, _ := preflightRun(t, root, ChangeOptions{Base: base, Citations: writeInput(t, rows)})
	if code != 0 || !strings.Contains(stdout, note) || !strings.Contains(stdout, "deferred<TAB>docs/specs/big.md<TAB>REASON (DCW-V0-370)") || !strings.Contains(stdout, "inputs-check PASS") {
		t.Fatalf("code=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
	deferred := rows + "deferred\tdocs/specs/big.md\tover the OCM map bound; its requirements are cited from BASE\n"
	code, stdout, stderr, _ = preflightRun(t, root, ChangeOptions{Base: base, Citations: writeInput(t, deferred)})
	if code != 0 || strings.Contains(stdout, note) {
		t.Fatalf("deferred: code=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
}
