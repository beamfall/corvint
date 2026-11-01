package gitnotes

import (
	"context"
	"testing"

	"github.com/Beamfall/corvint/internal/gitancestry/ancestrytest"
)

// EEP-V0-771: the notes-tip containment read ignores info/grafts, and a shallow boundary can only
// hide containment (rollback), never invent it (current).
func TestRefTipStateUnderRewrittenAncestry_EEPV0771(t *testing.T) {
	root := t.TempDir()
	fixtureGit(t, root, nil, "init", "-q")
	commit := func(message string) string {
		fixtureGit(t, root, nil, "-c", "user.name=F", "-c", "user.email=f@example.invalid", "commit", "-q", "--allow-empty", "-m", message)
		return fixtureGit(t, root, nil, "rev-parse", "HEAD")
	}
	pinned := commit("pinned")
	live := commit("live")
	fixtureGit(t, root, nil, "update-ref", Ref, live)
	state := func() any {
		t.Helper()
		r, err := open(root)
		if err != nil {
			t.Fatal(err)
		}
		got, err := r.refTipState(context.Background(), Ref, "--notes-tip", pinned)
		if err != nil {
			t.Fatal(err)
		}
		return got["state"]
	}
	if got := state(); got != "current" {
		t.Fatalf("plain checkout state = %v", got)
	}
	undo := ancestrytest.Graft(t, root, live)
	if got := state(); got != "current" {
		t.Errorf("grafted live tip state = %v, want current from real parents", got)
	}
	undo()
	ancestrytest.Shallow(t, root, live)
	if got := state(); got != "rollback" {
		t.Errorf("shallow live tip state = %v, want rollback", got)
	}
}
