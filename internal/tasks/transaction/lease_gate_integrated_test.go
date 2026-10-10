package transaction

import "testing"

// TestCALV0017_CommitNotIntegratedIsIntentOnly (V1-1081 review): the
// COMMIT_NOT_INTEGRATED code marks only an intent commit that neither the
// intent branch nor its upstream contains; an extra repository that is not
// integrated (CAL-V0-087) and a reachable commit refuse STALE_TREE alone.
func TestCALV0017_CommitNotIntegratedIsIntentOnly(t *testing.T) {
	for _, c := range []struct {
		name string
		f    LeaseFacts
		want bool
	}{
		{"intent unreachable, no upstream", CompleteFacts("t", false, "", "", nil), true},
		{"intent unreachable, upstream checked", CompleteFacts("t", false, "refs/remotes/origin/main", "", nil), true},
		{"extra repository not integrated", CompleteFacts("t", false, "", "docs", nil), false},
		{"reachable", CompleteFacts("t", true, "refs/remotes/origin/main", "", nil), false},
	} {
		if got := c.f.commitNotIntegrated(); got != c.want {
			t.Errorf("%s: commitNotIntegrated %v, want %v", c.name, got, c.want)
		}
	}
}
