package intent_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// intentRepo is a fixture repository whose primary holds the operator-authored
// queue and policy, with its HEAD on branch.
func intentRepo(t *testing.T, branch string) *fixture.Repo {
	t.Helper()
	r := fixture.TempRepo(t)
	fixture.Write(t, filepath.Join(r.IntentDir, "queue.json"), fixture.QueueBytes())
	fixture.Write(t, filepath.Join(r.IntentDir, "policy.json"), fixture.PolicyBytes())
	fixture.Write(t, filepath.Join(r.CommonDir, "HEAD"), []byte("ref: refs/heads/"+branch+"\n"))
	return r
}

// linkWorktree registers a linked worktree named name on branch the way
// `git worktree add` lays it out: an admin dir under <common>/worktrees with
// HEAD, gitdir and commondir, and a sibling checkout whose `.git` file points
// back to it and which carries a copy of the primary's queue and policy.
func linkWorktree(t *testing.T, r *fixture.Repo, name, branch string) (root, admin string) {
	t.Helper()
	admin = filepath.Join(r.CommonDir, "worktrees", name)
	root = filepath.Join(filepath.Dir(r.Root), name)
	fixture.Write(t, filepath.Join(admin, "HEAD"), []byte("ref: refs/heads/"+branch+"\n"))
	fixture.Write(t, filepath.Join(admin, "gitdir"), []byte(filepath.Join(root, ".git")+"\n"))
	fixture.Write(t, filepath.Join(admin, "commondir"), []byte("../..\n"))
	fixture.Write(t, filepath.Join(root, ".git"), []byte("gitdir: "+admin+"\n"))
	fixture.Write(t, filepath.Join(root, intent.Dir, "queue.json"), fixture.QueueBytes())
	fixture.Write(t, filepath.Join(root, intent.Dir, "policy.json"), fixture.PolicyBytes())
	return root, admin
}

func resolve(t *testing.T, dir string) *intent.Repository {
	t.Helper()
	repo, err := intent.Resolve(dir)
	if err != nil {
		t.Fatalf("resolve %s: %v", dir, err)
	}
	return repo
}

// TestCTWV0009_IntentBranchPrimaryKeepsThePrimaryRoot: a primary on the
// intent branch is the intent root even when a linked worktree on the same
// branch exists, and no repair text is recorded.
func TestCTWV0009_IntentBranchPrimaryKeepsThePrimaryRoot(t *testing.T) {
	r := intentRepo(t, "main")
	linkWorktree(t, r, "wt-main", "main")
	linkWorktree(t, r, "wt-feature", "feature")
	repo := resolve(t, r.Root)
	if repo.IntentRoot() != r.Root || repo.IntentLinked() || repo.IntentFix != "" {
		t.Fatalf("root=%q linked=%v fix=%q, want the primary and no fix", repo.IntentRoot(), repo.IntentLinked(), repo.IntentFix)
	}
	if repo.IntentHEAD() != filepath.Join(r.CommonDir, "HEAD") {
		t.Errorf("IntentHEAD = %q", repo.IntentHEAD())
	}
	for _, code := range []string{wire.CodeIntentBranchMismatch, wire.CodeIntentDiverged} {
		in := wire.Errorf(code, ".taskman", "unchanged")
		if out := repo.WithIntentRepair(in); out != in {
			t.Errorf("%s gained repair text on the intent-branch primary: %v", code, out)
		}
	}
	// A Repository built without resolution (zero intent fields) uses the primary.
	bare := &intent.Repository{PrimaryWorktree: r.Root, CommonDir: r.CommonDir}
	if bare.IntentRoot() != r.Root || bare.IntentHEAD() != filepath.Join(r.CommonDir, "HEAD") || bare.IntentLinked() {
		t.Errorf("unresolved repository does not default to the primary: %q %q", bare.IntentRoot(), bare.IntentHEAD())
	}
}

// TestCTWV0002_FeatureBranchPrimarySelectsTheLinkedIntentWorktree: with the
// primary on a feature branch, the one linked worktree on the intent branch
// becomes the intent root, from any checkout.
func TestCTWV0002_FeatureBranchPrimarySelectsTheLinkedIntentWorktree(t *testing.T) {
	r := intentRepo(t, "feature")
	root, admin := linkWorktree(t, r, "wt-main", "main")
	linkWorktree(t, r, "wt-other", "other")
	for _, from := range []string{r.Root, root, filepath.Join(r.Root, "sub")} {
		if err := os.MkdirAll(from, 0o755); err != nil {
			t.Fatal(err)
		}
		repo := resolve(t, from)
		if repo.PrimaryWorktree != r.Root || repo.StateDir != r.StateDir {
			t.Fatalf("from %s: identity moved: %+v", from, repo)
		}
		if repo.IntentRoot() != root || !repo.IntentLinked() || repo.IntentHEAD() != filepath.Join(admin, "HEAD") || repo.IntentFix != "" {
			t.Fatalf("from %s: root=%q head=%q fix=%q, want %q", from, repo.IntentRoot(), repo.IntentHEAD(), repo.IntentFix, root)
		}
		diverged := repo.WithIntentRepair(wire.Errorf(wire.CodeIntentDiverged, "tickets/A.json", "drift"))
		if wire.CodeOf(diverged) != wire.CodeIntentDiverged || !strings.Contains(diverged.Error(), filepath.Join(root, intent.Dir)) || !strings.Contains(diverged.Error(), "reconcile inspect") {
			t.Errorf("from %s: divergence repair = %v", from, diverged)
		}
		mismatch := wire.Errorf(wire.CodeIntentBranchMismatch, "HEAD", "x")
		if out := repo.WithIntentRepair(mismatch); out != mismatch {
			t.Errorf("a selected intent worktree needs no branch repair: %v", out)
		}
	}
}

// TestCTWV0005_NoIntentWorktreeNamesTheFix: with no linked worktree on the
// intent branch the primary stays the root and the recorded fix names
// `git worktree add`; only the two intent codes carry it.
func TestCTWV0005_NoIntentWorktreeNamesTheFix(t *testing.T) {
	r := intentRepo(t, "feature")
	linkWorktree(t, r, "wt-other", "other")
	repo := resolve(t, r.Root)
	if repo.IntentRoot() != r.Root || repo.IntentLinked() {
		t.Fatalf("root = %q, want the primary", repo.IntentRoot())
	}
	want := `run "git -C "` + r.Root
	for _, part := range []string{`branch "feature"`, `intent branch "main"`, "no linked worktree holds it", want, "worktree add", "CTW-V0-005"} {
		if !strings.Contains(repo.IntentFix, part) {
			t.Errorf("fix %q lacks %q", repo.IntentFix, part)
		}
	}
	in := wire.Errorf(wire.CodeIntentBranchMismatch, "HEAD", "HEAD is refs/heads/feature")
	out := repo.WithIntentRepair(in)
	if wire.CodeOf(out) != wire.CodeIntentBranchMismatch || out.(*wire.Error).Where != "HEAD" || !strings.HasPrefix(out.(*wire.Error).Msg, "HEAD is refs/heads/feature; ") {
		t.Errorf("repair did not preserve code, location and message: %#v", out)
	}
	other := wire.Errorf(wire.CodeMalformed, "x", "y")
	if repo.WithIntentRepair(other) != other {
		t.Error("an unrelated code gained repair text")
	}

	fixture.Write(t, filepath.Join(r.CommonDir, "HEAD"), []byte("0123456789abcdef0123456789abcdef01234567\n"))
	if fix := resolve(t, r.Root).IntentFix; !strings.Contains(fix, "a detached or unreadable HEAD") {
		t.Errorf("detached fix = %q", fix)
	}
}

// TestCTWV0004_StaleOrForeignIntentWorktreeIsNotAdmitted: a registration on
// the intent branch that does not round-trip, or whose projection names a
// different queue, is reported in the fix and never becomes the root.
func TestCTWV0004_StaleOrForeignIntentWorktreeIsNotAdmitted(t *testing.T) {
	cases := map[string]struct {
		break_ func(t *testing.T, r *fixture.Repo, root, admin string)
		want   string
	}{
		"missing worktree": {func(t *testing.T, r *fixture.Repo, root, admin string) {
			if err := os.RemoveAll(root); err != nil {
				t.Fatal(err)
			}
		}, "is stale: its worktree directory is missing"},
		"missing gitdir": {func(t *testing.T, r *fixture.Repo, root, admin string) {
			if err := os.Remove(filepath.Join(admin, "gitdir")); err != nil {
				t.Fatal(err)
			}
		}, "is stale: its gitdir file is missing"},
		"foreign back-pointer": {func(t *testing.T, r *fixture.Repo, root, admin string) {
			other := filepath.Join(r.CommonDir, "worktrees", "elsewhere")
			fixture.Write(t, filepath.Join(other, "HEAD"), []byte("ref: refs/heads/x\n"))
			fixture.Write(t, filepath.Join(root, ".git"), []byte("gitdir: "+other+"\n"))
		}, "points to a different registration"},
		"other queue": {func(t *testing.T, r *fixture.Repo, root, admin string) {
			q := fixture.QueueValue()
			q.Obj.Set("intentBranch", wire.String("release"))
			fixture.Write(t, filepath.Join(root, intent.Dir, "queue.json"), wire.EncodeFile(q))
		}, "names a different queue or intent branch"},
		"no queue": {func(t *testing.T, r *fixture.Repo, root, admin string) {
			if err := os.Remove(filepath.Join(root, intent.Dir, "queue.json")); err != nil {
				t.Fatal(err)
			}
		}, "queue.json is missing or unreadable"},
		"symlinked projection": {func(t *testing.T, r *fixture.Repo, root, admin string) {
			if err := os.RemoveAll(filepath.Join(root, intent.Dir)); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(r.IntentDir, filepath.Join(root, intent.Dir)); err != nil {
				t.Skip("symlinks unavailable")
			}
		}, "contains a symlink"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			r := intentRepo(t, "feature")
			root, admin := linkWorktree(t, r, "wt-main", "main")
			c.break_(t, r, root, admin)
			repo := resolve(t, r.Root)
			if repo.IntentRoot() != r.Root {
				t.Fatalf("an unadmitted worktree became the root: %q", repo.IntentRoot())
			}
			for _, part := range []string{`linked worktree entry "wt-main" `, c.want, "git worktree prune", "git worktree repair", "worktree add", "CTW-V0-005"} {
				if !strings.Contains(repo.IntentFix, part) {
					t.Errorf("fix %q lacks %q", repo.IntentFix, part)
				}
			}
		})
	}
}

// TestCTWV0006_TwoIntentWorktreesAreAmbiguous: two admitted linked worktrees
// on the intent branch select neither, and the fix names both.
func TestCTWV0006_TwoIntentWorktreesAreAmbiguous(t *testing.T) {
	r := intentRepo(t, "feature")
	a, _ := linkWorktree(t, r, "wt-a", "main")
	b, _ := linkWorktree(t, r, "wt-b", "main")
	repo := resolve(t, r.Root)
	if repo.IntentRoot() != r.Root {
		t.Fatalf("an ambiguous worktree became the root: %q", repo.IntentRoot())
	}
	for _, part := range []string{"2 linked worktrees hold it", a, b, "git worktree remove", "CTW-V0-006"} {
		if !strings.Contains(repo.IntentFix, part) {
			t.Errorf("fix %q lacks %q", repo.IntentFix, part)
		}
	}
}

// TestCTWV0003_NoHintKeepsThePrimary: without a readable primary queue
// manifest there is no intent branch to route by, so nothing is selected and
// nothing is recorded; the existing refusals apply unchanged.
func TestCTWV0003_NoHintKeepsThePrimary(t *testing.T) {
	r := intentRepo(t, "feature")
	linkWorktree(t, r, "wt-main", "main")
	if err := os.Remove(filepath.Join(r.IntentDir, "queue.json")); err != nil {
		t.Fatal(err)
	}
	repo := resolve(t, r.Root)
	if repo.IntentRoot() != r.Root || repo.IntentFix != "" {
		t.Fatalf("root=%q fix=%q, want the primary and no fix", repo.IntentRoot(), repo.IntentFix)
	}
}
