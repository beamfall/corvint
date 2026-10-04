//go:build darwin || linux

package authority

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// fixtureIntentWorktree is a fixture session on a repository whose primary is
// on a feature branch and whose linked worktree on the intent branch is the
// CTW-V0-002 intent root.
func fixtureIntentWorktree(t *testing.T) (*fixtureSession, string) {
	t.Helper()
	r := fixture.TempRepo(t)
	admin := filepath.Join(r.CommonDir, "worktrees", "intent")
	linked := filepath.Join(filepath.Dir(r.Root), "intent")
	fixture.Write(t, filepath.Join(admin, "HEAD"), []byte("ref: refs/heads/main\n"))
	fixture.Write(t, filepath.Join(admin, "gitdir"), []byte(filepath.Join(linked, ".git")+"\n"))
	fixture.Write(t, filepath.Join(linked, ".git"), []byte("gitdir: "+admin+"\n"))
	for _, root := range []string{r.Root, linked} {
		fixture.Write(t, filepath.Join(root, intent.Dir, "queue.json"), fixture.QueueBytes())
		fixtureMust(t, os.MkdirAll(filepath.Join(root, intent.Dir, "tickets"), 0700))
	}
	fixture.Write(t, filepath.Join(r.CommonDir, "HEAD"), []byte("ref: refs/heads/feature\n"))
	for _, path := range []string{"staging", "receipts", "evidence", "pinned", "requests/00"} {
		fixtureMust(t, os.MkdirAll(filepath.Join(r.StateDir, path), 0700))
	}
	repo, err := intent.Resolve(r.Root)
	fixtureMust(t, err)
	if repo.IntentRoot() != linked {
		t.Fatalf("intent root = %q, want %q (fix %q)", repo.IntentRoot(), linked, repo.IntentFix)
	}
	f, err := os.Open(r.Root)
	fixtureMust(t, err)
	_, observed := fixtureObserveMount(f, fixtureMount{})
	fixtureMust(t, f.Close())
	if observed != nil {
		t.Skipf("NOT_RUN: native fixture host/mount unsupported: %v", observed)
	}
	lock, err := AcquireLock(context.Background(), repo, LockOptions{})
	fixtureMust(t, err)
	s, err := newFixtureSession(repo, lock)
	if err != nil {
		fixtureMust(t, lock.Close())
		t.Fatal(err)
	}
	t.Cleanup(func() { s.before = nil; fixtureMust(t, s.close()) })
	return s, linked
}

// TestCTWV0008_SessionPinsTheLinkedIntentWorktree: the session retains the
// linked intent worktree as its own root, pins `.taskman` under it rather than
// under the primary, and refuses a replaced worktree directory.
func TestCTWV0008_SessionPinsTheLinkedIntentWorktree(t *testing.T) {
	s, linked := fixtureIntentWorktree(t)
	if s.worktree != "worktree" || s.parents["worktree"] == nil {
		t.Fatalf("worktree = %q, retained = %v", s.worktree, s.parents["worktree"] != nil)
	}
	pinned := s.parents["intent"]
	if pinned == nil || pinned.parent != "worktree" {
		t.Fatalf("intent pinned under %+v, want the linked worktree", pinned)
	}
	want, err := os.Lstat(filepath.Join(linked, intent.Dir))
	fixtureMust(t, err)
	if !os.SameFile(pinned.info, want) {
		t.Fatal("the pinned .taskman is not the linked worktree's")
	}
	fixtureMust(t, s.check())

	fixtureMust(t, os.Rename(linked, linked+".moved"))
	fixtureMust(t, os.MkdirAll(filepath.Join(linked, intent.Dir), 0700))
	if err := s.check(); err == nil {
		t.Fatal("a replaced linked intent worktree passed the session check")
	}
	fixtureMust(t, os.RemoveAll(linked))
	fixtureMust(t, os.Rename(linked+".moved", linked))
	fixtureMust(t, s.check())
}

// TestCTWV0008_LinkedIntentWorktreeOnAnotherMountIsRefused: publication links
// from staging under the common dir, so a linked intent worktree whose mount
// differs from the primary's is refused UNSUPPORTED_FILESYSTEM.
func TestCTWV0008_LinkedIntentWorktreeOnAnotherMountIsRefused(t *testing.T) {
	s, linked := fixtureIntentWorktree(t)
	retained := s.parents["worktree"]
	delete(s.parents, "worktree")
	t.Cleanup(func() { s.parents["worktree"] = retained })
	old := s.observe
	s.observe = func(f *os.File, held fixtureMount) (fixtureMount, error) {
		m, err := old(f, held)
		m.mount += "other"
		return m, err
	}
	err := s.retainWorktree()
	if wire.CodeOf(err) != wire.CodeUnsupportedFilesystem {
		t.Fatalf("err = %v, want UNSUPPORTED_FILESYSTEM", err)
	}
	if e := err.(*wire.Error); e.Where != linked {
		t.Errorf("refusal names %q, want %q", e.Where, linked)
	}
	if s.parents["worktree"] != nil {
		t.Error("a refused worktree was retained")
	}
}
