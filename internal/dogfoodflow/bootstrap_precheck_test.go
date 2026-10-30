package dogfoodflow

import (
	"strings"
	"testing"
)

// newIntentSpec is a companion spec the change adds, absent at BASE; its one
// hunk sorts first in the prepared map.
var newIntentSpec = map[string]string{"docs/specs/new-intent.md": "# New intent\n\n## Requirements\n\n- `NEW-V0-001`: a companion requirement.\n"}

// TestDCWV0449InputsCheckRefusesCitedBootstrapIntentHunk (V1-3108): a plan
// that cites the hunk of an intent the change adds is refused at the
// pre-flight, naming the row and the fix, before any bind; ocm prepare would
// refuse bootstrap-intent-not-unknown only after the bind pass. Fails on BASE,
// where the pre-flight passed this plan.
func TestDCWV0449InputsCheckRefusesCitedBootstrapIntentHunk(t *testing.T) {
	root, base := preflightRepository(t, newIntentSpec)
	head := testGit(t, root, "rev-parse", "HEAD")
	plan := writeInput(t, "1\tpre.go\t1:1\tdecision\n2\tpre.go\t3:3\timplementation\n3\tpre.go\t20:20\timplementation\n")
	intents := writeInput(t, "docs/specs/new-intent.md\n")
	code, stdout, stderr, labels := preflightRun(t, root, ChangeOptions{Base: base, Citations: plan, IntentsFile: intents})
	if code == 0 || strings.Contains(stdout, "inputs-check PASS") {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	for _, want := range []string{
		"FAIL dogfood-inputs-precheck ocm-prepare bootstrap-intent-not-unknown input=--citations " + plan + " rows=1",
		`row 1 hunk=1 path="docs/specs/new-intent.md" cites pre.go:1:1 as decision, but docs/specs/new-intent.md is an intent absent at BASE`,
		"  fix: " + bootstrapIntentHint,
		"prepare dogfood-inputs-precheck NOT_PRODUCED bootstrap-intent-not-unknown",
	} {
		if !strings.Contains(stdout+stderr, want) {
			t.Errorf("output lacks %q:\nstdout=%s\nstderr=%s", want, stdout, stderr)
		}
	}
	assertNothingBound(t, root, head, labels)
}

// TestDCWV0449InputsCheckPassesUncitedBootstrapIntentHunk (V1-3108): the same
// change passes when the plan names no row for the new intent's hunk, or keeps
// it unknown, the shapes the bind and OCM-V0-009 admit.
func TestDCWV0449InputsCheckPassesUncitedBootstrapIntentHunk(t *testing.T) {
	for name, plan := range map[string]string{
		"no row":      "2\tpre.go\t3:3\timplementation\n3\tpre.go\t20:20\timplementation\n",
		"unknown row": "1\tunknown\tinsufficient-evidence\tbootstrap intent added by this change\n2\tpre.go\t3:3\timplementation\n3\tpre.go\t20:20\timplementation\n",
	} {
		t.Run(name, func(t *testing.T) {
			root, base := preflightRepository(t, newIntentSpec)
			head := testGit(t, root, "rev-parse", "HEAD")
			code, stdout, stderr, labels := preflightRun(t, root, ChangeOptions{Base: base, Citations: writeInput(t, plan), IntentsFile: writeInput(t, "docs/specs/new-intent.md\n")})
			if code != 0 || !strings.Contains(stdout, "inputs-check PASS nothing was bound or committed") {
				t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout, stderr)
			}
			assertNothingBound(t, root, head, labels)
		})
	}
}
