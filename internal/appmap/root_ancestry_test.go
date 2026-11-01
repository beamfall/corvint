package appmap

import (
	"context"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/gitancestry/ancestrytest"
)

// EEP-V0-773: the site-knowledge repository identity is its one root commit, which a grafted or
// shallow checkout cannot name, so either refuses appmap-invalid-revision with the reason.
func TestRepositoryRootIDRefusesRewrittenAncestry_EEPV0773(t *testing.T) {
	root := t.TempDir()
	gitTest(t, root, "init", "-q")
	gitTest(t, root, "-c", "user.name=F", "-c", "user.email=f@example.invalid", "commit", "-q", "--allow-empty", "-m", "one")
	gitTest(t, root, "-c", "user.name=F", "-c", "user.email=f@example.invalid", "commit", "-q", "--allow-empty", "-m", "two")
	head := strings.TrimSpace(gitTest(t, root, "rev-parse", "HEAD"))
	r := repo{ctx: context.Background(), root: root}
	if id, err := repositoryRootID(r, head); err != nil || id == "" {
		t.Fatalf("plain checkout id = %q, %v", id, err)
	}
	for _, fixture := range []struct {
		name, reason string
		apply        func() func()
	}{
		{"grafted", "checkout rewrites ancestry with info/grafts", func() func() { return ancestrytest.Graft(t, root, head) }},
		{"shallow", "checkout is shallow", func() func() { return ancestrytest.Shallow(t, root, head) }},
	} {
		undo := fixture.apply()
		if id, err := repositoryRootID(r, head); err == nil || !strings.Contains(err.Error(), fixture.reason) {
			t.Errorf("%s: id = %q, err = %v", fixture.name, id, err)
		}
		undo()
	}
}
