package appflows

import (
	"context"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/gitancestry/ancestrytest"
)

// EEP-V0-770: in a grafted or shallow root checkout a review anchor cannot be proved an ancestor,
// so it is anchor-unknown rather than reviewed, and no root coverage evidence diff is read: the
// e2e-safe selection falls back naming the rewritten ancestry.
func TestRootAncestryRewrittenIsUnknown_EEPV0770(t *testing.T) {
	t.Parallel()
	root := intentRepo(t)
	anchor := gitOut(t, root, "rev-parse", "HEAD")
	if got := spanState(t, root, anchor, "HEAD"); got.ReviewState != ReviewReviewed {
		t.Fatalf("plain checkout anchor = %s", got.ReviewState)
	}
	for _, fixture := range []struct {
		name, reason string
		apply        func() func()
	}{
		{"grafted", "grafts", func() func() { return ancestrytest.Graft(t, root, "HEAD") }},
		{"shallow", "shallow", func() func() { return ancestrytest.Shallow(t, root, "HEAD") }},
	} {
		undo := fixture.apply()
		if got := spanState(t, root, anchor, "HEAD"); got.ReviewState != ReviewAnchorUnknown || got.Basis == "reviewed" {
			t.Errorf("%s: anchor = %+v, want %s", fixture.name, got, ReviewAnchorUnknown)
		}
		s := newE2ESelector(context.Background(), root, E2EInput{Base: anchor, Revision: anchor}, SelectionProvider{}, nil, nil)
		if diff := s.evidenceDiff(anchor); diff.ok || !s.ancestryRefused || !strings.Contains(s.rootRewritten, fixture.reason) {
			t.Errorf("%s: evidence diff ok=%v refused=%v reason=%q", fixture.name, diff.ok, s.ancestryRefused, s.rootRewritten)
		}
		undo()
	}
	s := newE2ESelector(context.Background(), root, E2EInput{Base: anchor, Revision: anchor}, SelectionProvider{}, nil, nil)
	if diff := s.evidenceDiff(anchor); !diff.ok || s.ancestryRefused {
		t.Fatalf("plain checkout evidence diff ok=%v refused=%v", diff.ok, s.ancestryRefused)
	}
}
