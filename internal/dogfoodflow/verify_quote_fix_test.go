// SPDX-License-Identifier: Apache-2.0

package dogfoodflow

import (
	"context"
	"io"
	"path/filepath"
	"strings"
	"testing"
)

// TestDCWV0403UnquotedRunAlternationRefusesInOneLine (V1-3131): an unquoted
// -run TestA|TestB is refused with one fix line naming the admitted quoted
// form, by inputs-check and by the dogfood-change local-outcome hint, and that
// quoted form is admitted.
func TestDCWV0403UnquotedRunAlternationRefusesInOneLine(t *testing.T) {
	root, base := preflightRepository(t, nil)
	plan := writeInput(t, "1\tpre.go\t3:3\timplementation\n2\tpre.go\t20:20\timplementation\n")
	verify := writeInput(t, "go test -run TestA ./pre\ngo test -count=1 -run TestA|TestB ./pre\n")
	code, _, stderr, labels := preflightRun(t, root, ChangeOptions{Base: base, Citations: plan, VerifyFile: verify})
	quoted := "go test -count=1 -run 'TestA|TestB' ./pre"
	want := "dogfood-inputs-check: REFUSE unsupported-verify-syntax\n" +
		"  fix: --verify " + verify + " line 2: quote the -run alternation as one word: " + quoted + " (DCW-V0-403; quoting rule: docs/DOGFOOD.md Inputs, DOGFOOD_VERIFY_FILE), then rerun corvint dogfood finish-lane --base " + base + "\n"
	if code != 2 || len(labels) != 0 || stderr != want {
		t.Fatalf("code=%d steps=%q stderr=\n%q\nwant\n%q", code, labels, stderr, want)
	}
	if validateVerify(verifyLineArguments(quoted)) != nil {
		t.Fatalf("quoted form %q is not admitted", quoted)
	}
	c := &change{options: ChangeOptions{VerifyFile: verify}}
	hint := c.fixHint(step{"local-outcome", "NOT_PRODUCED", "unsupported-verify-syntax"})
	if hint != "DOGFOOD_VERIFY_FILE "+verify+" line 2: quote the -run alternation as one word: "+quoted+" (DCW-V0-403; quoting rule: docs/DOGFOOD.md Inputs, DOGFOOD_VERIFY_FILE)" {
		t.Fatalf("local-outcome hint = %q", hint)
	}

	// The quoted form passes the early check end to end and reaches prepare.
	good := writeInput(t, quoted+"\n")
	calls := 0
	steps := Runner{Path: "corvint", Run: func(context.Context, string, []string, io.Writer, io.Writer) int {
		calls++
		return 1
	}}
	var stdout, errout strings.Builder
	if _, err := FinishLane(context.Background(), FinishLaneOptions{Change: ChangeOptions{Root: root, Base: base, Steps: steps, Citations: plan, VerifyFile: good, Outcome: "passed"}}, &stdout, &errout); err != nil {
		t.Fatal(err)
	}
	if calls == 0 || strings.Contains(errout.String(), "REFUSE") {
		t.Fatalf("quoted verify did not reach prepare: calls=%d stderr=%q", calls, errout.String())
	}

	// A grouped regex is not an alternation of names: the grammar hint stays.
	other := writeInput(t, "go test -run (TestA|TestB) ./pre\n")
	if fix := quotedRunFix(other); fix != "" {
		t.Fatalf("grouped regex got quote fix %q", fix)
	}
	if fix := quotedRunFix(filepath.Join(t.TempDir(), "missing")); fix != "" {
		t.Fatalf("missing file got quote fix %q", fix)
	}
}
