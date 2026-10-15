//go:build darwin || linux

package taskman

import (
	"testing"

	"github.com/Beamfall/corvint/internal/cem/verify"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

// The --touch-paths arguments the Tasks store substitutes for one ticket that
// declares its full write set ["lib/","src/"] as touchPaths and ["src/"] as
// verdictPaths (internal/tasks/store TestTCBV0037_VerdictPathsPlaceholder
// asserts these exact bytes as the gate's substituted argv).
const (
	verdictWriteSetArgument = `["lib/","src/"]`
	verdictDomainArgument   = `["src/"]`
)

func touchArgument(t *testing.T, raw string) []string {
	t.Helper()
	v, err := wire.Parse([]byte(raw))
	if err != nil || v.Kind != wire.KindArray {
		t.Fatalf("argument %q: %v", raw, err)
	}
	var out []string
	for _, x := range v.Arr {
		out = append(out, x.Str)
	}
	return out
}

// TestTCBV0038_VerdictDomainNarrowsTheGovernedSet: through the real gate on a
// dogfood-shaped change whose src/ hunk is anchored and whose lib/ hunk is
// only cited, the claim-scope write set refuses tier-below-anchored (the
// pre-TCB-V0-035 outcome when touchPaths is the only domain), while the
// narrower verdict domain grants, governing only the anchored hunk.
func TestTCBV0038_VerdictDomainNarrowsTheGovernedSet(t *testing.T) {
	f := newVerdictFixture(t, nil)
	full := requireVerdictRefusal(t, f.input(PredicateTouchedAnchored, touchArgument(t, verdictWriteSetArgument)...), CEMVerdictPredicateFailed, "tier-below-anchored")
	if len(full.Governed) != 2 {
		t.Fatalf("the write set governs both hunks: %+v", full.Governed)
	}
	result, view := evaluateVerdict(t, f.input(PredicateTouchedAnchored, touchArgument(t, verdictDomainArgument)...))
	if !result.Granted || view.Outcome != "GRANT" || len(view.Governed) != 1 || view.Governed[0].Path != "src/app.txt" || view.Governed[0].Tier != verify.TierAnchored {
		t.Fatalf("the verdict domain must grant over the anchored hunk only:\n%s", result.Document)
	}
	if view.Inputs == nil || len(view.Inputs.TouchPaths) != 1 || view.Inputs.TouchPaths[0] != "src/" {
		t.Fatalf("gate evidence records the governed domain: %+v", view.Inputs)
	}
}
