package repository

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	dashboardauthority "github.com/Beamfall/corvint/internal/dashboard/authority"
	"github.com/Beamfall/corvint/internal/gitancestry/ancestrytest"
)

// EEP-V0-770: a shallow (or grafted) checkout qualifies no trace, even HEAD itself, because its
// ancestry answers come from rewritten parents.
func TestQualifyTraceRefusesRewrittenAncestry_EEPV0770(t *testing.T) {
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("upstream Git unavailable")
	}
	root := filepath.Join(stableTestDirectory(t, "rewritten-"), "repository")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	runTestGit(t, git, root, "init", "-q")
	if err := os.WriteFile(filepath.Join(root, "tracked"), []byte("tracked\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runTestGit(t, git, root, "add", "tracked")
	runTestGit(t, git, root, "-c", "user.name=Corvint", "-c", "user.email=corvint@example.invalid", "commit", "-q", "-m", "initial")
	qualify := func() dashboardauthority.TraceCode {
		t.Helper()
		authority, snapshot, failure := New(context.Background(), root)
		if failure != nil {
			t.Fatalf("New failure = %#v", failure)
		}
		defer authority.Close()
		return authority.QualifyTrace(context.Background(), snapshot.HeadRevision, []string{"tracked"}).Code
	}
	if code := qualify(); code != dashboardauthority.TraceQualified {
		t.Fatalf("plain checkout HEAD = %s", code)
	}
	for name, apply := range map[string]func() func(){
		"grafted": func() func() { return ancestrytest.Graft(t, root, "HEAD") },
		"shallow": func() func() { return ancestrytest.Shallow(t, root, "HEAD") },
	} {
		undo := apply()
		if code := qualify(); code != dashboardauthority.TraceAncestryBound {
			t.Errorf("%s checkout HEAD = %s", name, code)
		}
		undo()
	}
}
