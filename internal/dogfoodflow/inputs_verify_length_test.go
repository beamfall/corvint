// SPDX-License-Identifier: Apache-2.0

package dogfoodflow

import (
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/trace"
)

// TestASDV0088InputsCheckRefusesOverlongVerifyLine (V1-3107): a verify line
// longer than the recorder's 512-character command bound is refused by
// dogfood-inputs-check, naming the line, before any step runs or binds, so
// the bound never costs a closeout try.
func TestASDV0088InputsCheckRefusesOverlongVerifyLine(t *testing.T) {
	root, base := preflightRepository(t, nil)
	head := testGit(t, root, "rev-parse", "HEAD")
	plan := writeInput(t, "1\tpre.go\t3:3\timplementation\n2\tpre.go\t20:20\timplementation\n")
	long := "go test -run TestA ./pre/" + strings.Repeat("a", trace.MaxCommandCharacters)
	verify := writeInput(t, "go test -run TestA ./pre\n"+long+"\n")
	code, stdout, stderr, labels := preflightRun(t, root, ChangeOptions{Base: base, Citations: plan, VerifyFile: verify})
	if code != 2 || len(labels) != 0 {
		t.Fatalf("code=%d steps=%q stdout=%q stderr=%q", code, labels, stdout, stderr)
	}
	for _, want := range []string{
		"dogfood-inputs-check: REFUSE ",
		"  input: --verify " + verify + " line 2\n",
		"exceeds 512",
		"  fix: ",
	} {
		if !strings.Contains(stderr, want) {
			t.Errorf("stderr lacks %q:\n%s", want, stderr)
		}
	}
	assertNothingBound(t, root, head, labels)
}
