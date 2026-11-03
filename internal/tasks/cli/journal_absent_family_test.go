package cli

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestTCPV02230_JournalAbsentMissNamesTheSharedJournal pins V1-4440: a lane's
// linked worktree whose primary worktree has no journal is refused, as
// before, but the refusal now says that the primary and every linked worktree
// share that one journal location, so no `cd` among them can reach a journal;
// the ticket is read from another clone or host, or through a snapshot. The
// read creates no store (TCP-V0-2230).
func TestTCPV02230_JournalAbsentMissNamesTheSharedJournal(t *testing.T) {
	fixture.SkipUnsupportedPlatform(t)
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	primary := filepath.Join(base, "primary")
	realGit(t, base, "init", "-q", "-b", "main", primary)
	fixture.Write(t, filepath.Join(primary, ".taskman", "queue.json"), fixture.QueueBytes())
	fixture.Write(t, filepath.Join(primary, ".taskman", "policy.json"), fixture.PolicyBytes())
	realGit(t, primary, "add", ".taskman")
	realGit(t, primary, "commit", "-qm", "queue and policy")
	lane := filepath.Join(base, "lane")
	realGit(t, primary, "worktree", "add", "-q", "-b", "lane", lane)

	const id = "ticket:acme:main:ZZ-404"
	for _, c := range []struct{ cwd, family string }{
		{lane, "This checkout is a linked worktree of the primary worktree " + primary + ","},
		{primary, "This checkout is the primary worktree " + primary + ","},
	} {
		got := projectionRead(t, Env{Cwd: c.cwd, Args: []string{"ticket", "show", id}})
		text := strings.Join(got.Warnings, " ")
		if got.Outcome != wire.OutcomeRefused || len(got.Items) != 0 || !slices.Equal(got.Codes, []string{wire.CodeMalformed}) { // TCL-V0-859
			t.Fatalf("%s: %+v", c.cwd, got)
		}
		for _, want := range []string{"journal-absent", "no journal exists at " + filepath.Join(primary, ".git", "taskman"), c.family,
			"shares that one journal location with all its linked worktrees, so none of them holds a journal either and `cd` among them cannot help (TCP-V0-2230)",
			"the journal is in another clone or on another host", "CORVINT_TASKS_SNAPSHOT=FILE corvint-tasks ticket show " + id} {
			if !strings.Contains(text, want) {
				t.Errorf("%s: refusal lacks %q: %s", c.cwd, want, text)
			}
		}
	}
	if out := realGit(t, primary, "status", "--porcelain", "--ignored"); out != "" {
		t.Errorf("the read changed the primary worktree: %q", out)
	}
	if _, err := os.Lstat(filepath.Join(primary, ".git", "taskman")); !os.IsNotExist(err) {
		t.Errorf("the read created a store: %v", err)
	}
}
