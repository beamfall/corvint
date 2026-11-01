package main

import (
	"context"
	"testing"

	"github.com/Beamfall/corvint/internal/gitancestry/ancestrytest"
)

// EEP-V0-770: a grafted or shallow checkout cannot place the baseline against the branch point,
// so staleness is NOT_OBSERVED BRANCH_POINT_UNRESOLVED rather than a FRESH or STALE count.
func TestRunsBaselineStalenessDistrustsRewrittenAncestry_EEPV0770(t *testing.T) {
	repo := t.TempDir()
	reuseGit(t, repo, "init", "-q")
	reuseGit(t, repo, "config", "user.email", "corvint@example.test")
	reuseGit(t, repo, "config", "user.name", "Corvint Test")
	baseline := reuseCommit(t, repo, map[string]string{"a.txt": "1"})
	reuseCommit(t, repo, map[string]string{"a.txt": "2"})
	ctx := context.Background()
	if s := baselineStaleness(ctx, repo, baseline, "HEAD", 50); s.Status != "FRESH" {
		t.Fatalf("plain checkout staleness = %+v", s)
	}
	for name, apply := range map[string]func() func(){
		"grafted": func() func() { return ancestrytest.Graft(t, repo, "HEAD") },
		"shallow": func() func() { return ancestrytest.Shallow(t, repo, "HEAD") },
	} {
		undo := apply()
		if s := baselineStaleness(ctx, repo, baseline, "HEAD", 50); s.Status != "NOT_OBSERVED" || s.Reason != "BRANCH_POINT_UNRESOLVED" || s.Commits != nil {
			t.Errorf("%s: staleness = %+v", name, s)
		}
		undo()
	}
}
