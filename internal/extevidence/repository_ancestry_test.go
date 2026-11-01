package extevidence

import (
	"context"
	"testing"

	"github.com/Beamfall/corvint/internal/gitancestry/ancestrytest"
)

// EEP-V0-773: a grafted or shallow checkout's parentless commits are the graft or the boundary,
// not its origin, so no root commit is read as identity and an explicit checkout is unavailable.
func TestRootCommitsDistrustRewrittenAncestry_EEPV0773(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := newRepository(t)
	if roots := rootCommits(ctx, repo.root, repo.head); len(roots) != 1 {
		t.Fatalf("plain checkout roots = %v", roots)
	}
	for _, fixture := range []struct {
		name, reason string
		apply        func() func()
	}{
		{"grafted", "checkout rewrites ancestry with info/grafts", func() func() { return ancestrytest.Graft(t, repo.root, repo.head) }},
		{"shallow", "checkout is shallow", func() func() { return ancestrytest.Shallow(t, repo.root, repo.head) }},
	} {
		undo := fixture.apply()
		if roots := rootCommits(ctx, repo.root, repo.head); len(roots) != 0 {
			t.Errorf("%s: roots = %v, want none", fixture.name, roots)
		}
		if entry := resolveCheckout(ctx, repo.root, repo.root); entry.state != BindingUnavailable || entry.reason != fixture.reason {
			t.Errorf("%s: checkout = %s %q", fixture.name, entry.state, entry.reason)
		}
		undo()
	}
}
