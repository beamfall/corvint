package lspprovider

import (
	"context"
	"testing"

	"github.com/Beamfall/corvint/internal/gitancestry/ancestrytest"
)

// EEP-V0-773: a grafted or shallow checkout has no trustworthy origin, so rootCommit refuses
// (the provider then fails `root commit unavailable`) instead of pinning the graft or boundary.
func TestRootCommitRefusesRewrittenAncestry_EEPV0773(t *testing.T) {
	t.Parallel()
	m := newModule(t)
	if origin, err := rootCommit(context.Background(), m.root, m.head); err != nil || origin == "" {
		t.Fatalf("plain checkout root = %q, %v", origin, err)
	}
	for name, apply := range map[string]func() func(){
		"grafted": func() func() { return ancestrytest.Graft(t, m.root, m.head) },
		"shallow": func() func() { return ancestrytest.Shallow(t, m.root, m.head) },
	} {
		undo := apply()
		if origin, err := rootCommit(context.Background(), m.root, m.head); err == nil {
			t.Errorf("%s checkout root = %q, want refusal", name, origin)
		}
		undo()
	}
}
