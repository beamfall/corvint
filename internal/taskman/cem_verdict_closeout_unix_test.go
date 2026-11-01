//go:build darwin || linux

package taskman

import (
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/verify"
)

// TestTCBV0870_CitedOnlyDeliveryClosesThroughARunnerBoundMap (V1-1840) is
// the closeout route for a merged delivery whose sealed map is cited-only.
// The original map refuses tier-below-anchored over the delivered code. A
// closeout note that no touchPath governs refuses no-governed-hunk. The
// runner-bound map over the same range and touchPaths GRANTs on the code
// itself (TCB-V0-870). Each answer comes from the read-only preflight
// before any gate run, leaving the repository unchanged (TCB-V0-871).
func TestTCBV0870_CitedOnlyDeliveryClosesThroughARunnerBoundMap(t *testing.T) {
	code := []string{"lib/fresh.txt", "lib/util.txt"}
	preflight := func(f verdictFixture, touch []string, check func(CEMVerdictInput)) {
		t.Helper()
		state := func() string {
			return strings.Join([]string{
				f.git("status", "--porcelain=v1", "--ignored", "--untracked-files=all"),
				f.git("for-each-ref"), f.git("rev-parse", "HEAD"), f.git("count-objects", "-v"),
			}, "\n")
		}
		before := state()
		check(f.input(PredicateTouchedAnchored, touch...))
		if after := state(); after != before {
			t.Fatalf("the preflight mutated the repository:\n%s\n---\n%s", before, after)
		}
	}

	citedOnly := newRunnerFixture(t, runnerFixtureOptions{unlinked: code})
	preflight(citedOnly, code, func(in CEMVerdictInput) {
		view := requireVerdictRefusal(t, in, CEMVerdictPredicateFailed, "tier-below-anchored")
		for _, path := range code {
			if h := governedByPath(t, view)[path]; h.Tier != verify.TierCited || h.Satisfied {
				t.Fatalf("%s must stay cited and unsatisfied in the cited-only map: %+v", path, h)
			}
		}
	})
	preflight(citedOnly, []string{"docs/build-log/"}, func(in CEMVerdictInput) {
		if view := requireVerdictRefusal(t, in, CEMVerdictPredicateFailed, "no-governed-hunk"); len(view.Governed) != 0 {
			t.Fatalf("a closeout note no touchPath governs gates nothing: %+v", view.Governed)
		}
	})

	closeout := newRunnerFixture(t, runnerFixtureOptions{})
	preflight(closeout, code, func(in CEMVerdictInput) {
		result, view := evaluateVerdict(t, in)
		if !result.Granted || view.Outcome != "GRANT" {
			t.Fatalf("the runner-bound closeout map must grant over the delivered code:\n%s", result.Document)
		}
		byPath := governedByPath(t, view)
		if len(byPath) != len(code) {
			t.Fatalf("the grant must govern exactly the delivered code paths: %+v", view.Governed)
		}
		for _, path := range code {
			if h := byPath[path]; h.Tier != verify.TierCited || !h.Satisfied || h.Anchor != runnerAnchorInput {
				t.Fatalf("%s must stay cited and be satisfied on runner-input: %+v", path, h)
			}
		}
	})
}
