package sharedpath

import (
	"context"
	"testing"

	"github.com/Beamfall/corvint/internal/gitancestry/ancestrytest"
)

// EEP-V0-771: the shared-path Git reader ignores info/grafts, so a graft that cuts head from its
// parent changes no ancestry answer, while a shallow boundary on the same head still refuses as
// changes-shared-shallow-merge-base (SPC-V0-015).
func TestAncestryReadsIgnoreGraftsAndRefuseShallow_EEPV0771(t *testing.T) {
	f := newFixture(t)
	base := f.commit("base", map[string]string{"a.txt": "a\n"})
	head := f.commit("head", map[string]string{"a.txt": "b\n"})
	g := git{ctx: context.Background(), root: f.root}
	undo := ancestrytest.Graft(t, f.root, head)
	if _, code, err := g.run(nil, nil, "merge-base", "--is-ancestor", base, head); err != nil || code != 0 {
		t.Errorf("grafted head lost its real parent: code=%d err=%v", code, err)
	}
	if err := refuseShallowMergeBase(g, base, head); err != nil {
		t.Errorf("grafted full checkout refused: %v", err)
	}
	undo()
	ancestrytest.Shallow(t, f.root, head)
	if err := refuseShallowMergeBase(g, base, head); err == nil || err.Error() != CodeShallowMergeBase {
		t.Errorf("shallow head = %v, want %s", err, CodeShallowMergeBase)
	}
}
