package tracerecordrepo

import (
	"context"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/gitancestry/ancestrytest"
)

// EEP-V0-771: the replay-window probe ignores info/grafts, and a shallow boundary can only turn a
// real ancestor into a non-ancestor, which still ends in a read failure.
func TestProbeGitAncestorUnderRewrittenAncestry_EEPV0771(t *testing.T) {
	root := newAdapterFixture(t)
	parent := gitAdapterFixture(t, root, "rev-parse", "HEAD")
	writeAdapterFixture(t, root, "internal/value.go", "package internal\n\nconst Value = 2\n")
	gitAdapterFixture(t, root, "add", "internal/value.go")
	gitAdapterFixture(t, root, "commit", "-qm", "real child")
	head := gitAdapterFixture(t, root, "rev-parse", "HEAD")
	probe := func() bool {
		t.Helper()
		ancestor, err := probeGitAncestor(context.Background(), gitrun.NewDefaultBudget(), root, parent, head)
		if err != nil {
			t.Fatal(err)
		}
		return ancestor
	}
	if !probe() {
		t.Fatal("plain checkout: real parent is not an ancestor")
	}
	undo := ancestrytest.Graft(t, root, head)
	if !probe() {
		t.Error("grafted checkout lost the real parent")
	}
	undo()
	ancestrytest.Shallow(t, root, head)
	if probe() {
		t.Error("parent past the shallow boundary probed as an ancestor")
	}
}
