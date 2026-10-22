// SPDX-License-Identifier: AGPL-3.0-or-later

package corpusbridge

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/doccorpus"
)

// shortenStampWindow lets a test stamp entries it has just written; the
// production window is covered by TestDCPV1314StampRefusesRacyEntries.
func shortenStampWindow(t *testing.T) {
	t.Helper()
	saved := stampRacyWindow
	stampRacyWindow = 0
	t.Cleanup(func() { stampRacyWindow = saved })
}

func stampGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	c := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "-c", "maintenance.auto=false", "-c", "gc.auto=0"}, args...)...)
	c.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null")
	if out, err := c.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v %s", args, err, out)
	}
}

func stampRepository(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "docs"), 0o700); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{"docs/a.md": "alpha\n", "docs/b.md": "bravo\n", "README": "readme\n"} {
		if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(name)), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink("docs/a.md", filepath.Join(dir, "link")); err != nil {
		t.Fatal(err)
	}
	stampGit(t, dir, "init", "-q")
	stampGit(t, dir, "add", ".")
	stampGit(t, dir, "commit", "-qm", "init")
	return dir
}

func supportedStampPlatform(t *testing.T) {
	t.Helper()
	if _, _, _, ok := fileTimes(mustLstat(t, t.TempDir())); !ok {
		t.Skip("the checkout stamp is unavailable on this platform (DCP-V1-314)")
	}
}

func mustLstat(t *testing.T, path string) os.FileInfo {
	t.Helper()
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	return info
}

// TestDCPV1314StampChangesWithCheckoutState: every worktree or repository
// control change the reuse key or pin checks could observe changes the stamp,
// and an unchanged checkout keeps it.
func TestDCPV1314StampChangesWithCheckoutState(t *testing.T) {
	supportedStampPlatform(t)
	shortenStampWindow(t)
	dir := stampRepository(t)
	previous := checkoutStamp([]string{dir})
	if previous == "" || checkoutStamp([]string{dir}) != previous {
		t.Fatalf("unchanged checkout stamp unstable or unavailable: %q", previous)
	}
	write := func(name, body string) func() {
		return func() {
			if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(name)), []byte(body), 0o600); err != nil {
				t.Fatal(err)
			}
		}
	}
	for _, change := range []struct {
		name  string
		apply func()
	}{
		{"same-size tracked rewrite", write("docs/a.md", "ALPHA\n")},
		{"mode change", func() {
			if err := os.Chmod(filepath.Join(dir, "docs", "b.md"), 0o700); err != nil {
				t.Fatal(err)
			}
		}},
		{"untracked file in a subdirectory", write("docs/new.md", "new\n")},
		{"empty directory", func() {
			if err := os.Mkdir(filepath.Join(dir, "empty"), 0o700); err != nil {
				t.Fatal(err)
			}
		}},
		{"symlink retarget", func() {
			if err := os.Remove(filepath.Join(dir, "link")); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink("docs/b.md", filepath.Join(dir, "link")); err != nil {
				t.Fatal(err)
			}
		}},
		{"deleted file", func() {
			if err := os.Remove(filepath.Join(dir, "docs", "new.md")); err != nil {
				t.Fatal(err)
			}
		}},
		{"info/exclude", write(".git/info/exclude", "*.tmp\n")},
		{"commit", func() { stampGit(t, dir, "commit", "-qam", "next") }},
		{"packed refs", func() { stampGit(t, dir, "pack-refs", "--all") }},
		{"repack", func() { stampGit(t, dir, "repack", "-qad") }},
	} {
		change.apply()
		next := checkoutStamp([]string{dir})
		if next == "" || next == previous {
			t.Fatalf("%s: stamp %q after %q", change.name, next, previous)
		}
		previous = next
	}
}

// TestDCPV1314StampUnavailableOutsideCoveredState: state the stamp does not
// cover, or cannot prove unchanged, makes it unavailable instead of stale.
func TestDCPV1314StampUnavailableOutsideCoveredState(t *testing.T) {
	supportedStampPlatform(t)
	shortenStampWindow(t)
	for _, c := range []struct {
		name  string
		apply func(t *testing.T, dir string) string
	}{
		{"nested repository", func(t *testing.T, dir string) string {
			stampGit(t, filepath.Join(dir, "docs"), "init", "-q")
			return dir
		}},
		{"gitfile worktree", func(t *testing.T, dir string) string {
			linked := filepath.Join(t.TempDir(), "linked")
			stampGit(t, dir, "worktree", "add", "-q", linked)
			return linked
		}},
		{"alternate object store", func(t *testing.T, dir string) string {
			if err := os.WriteFile(filepath.Join(dir, ".git", "objects", "info", "alternates"), []byte("/elsewhere\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			return dir
		}},
		{"config include", func(t *testing.T, dir string) string {
			stampGit(t, dir, "config", "include.path", "/elsewhere")
			return dir
		}},
		{"not a repository", func(t *testing.T, dir string) string { return t.TempDir() }},
		{"missing checkout", func(t *testing.T, dir string) string { return filepath.Join(dir, "absent") }},
	} {
		t.Run(c.name, func(t *testing.T) {
			dir := stampRepository(t)
			if checkoutStamp([]string{dir}) == "" {
				t.Fatal("baseline stamp unavailable")
			}
			if got := checkoutStamp([]string{c.apply(t, dir)}); got != "" {
				t.Fatalf("stamp %q, want unavailable", got)
			}
		})
	}
}

// TestDCPV1314StampRefusesRacyEntries: an entry changed inside the racy window
// leaves no stamp, so a later same-granule change cannot go unseen.
func TestDCPV1314StampRefusesRacyEntries(t *testing.T) {
	supportedStampPlatform(t)
	dir := stampRepository(t)
	saved := stampRacyWindow
	t.Cleanup(func() { stampRacyWindow = saved })
	stampRacyWindow = time.Hour
	if got := checkoutStamp([]string{dir}); got != "" {
		t.Fatalf("stamp %q over entries written inside the window", got)
	}
}

// TestDCPV1315WarmCallsSkipGitWhileStampUnchanged: once a stamp is paired
// with an observation and a pin verification, warm calls run neither while
// the checkouts are unchanged, and any change runs both again (DCP-V1-316).
func TestDCPV1315WarmCallsSkipGitWhileStampUnchanged(t *testing.T) {
	supportedStampPlatform(t)
	shortenStampWindow(t)
	observed, verified := 0, 0
	savedObserve, savedVerify := observeCorpus, verifyPinned
	t.Cleanup(func() { observeCorpus, verifyPinned = savedObserve, savedVerify })
	observeCorpus = func(ctx context.Context, root string, a *doccorpus.Artifact) (string, error) {
		observed++
		return savedObserve(ctx, root, a)
	}
	verifyPinned = func(ctx context.Context, root string, m doccorpus.Manifest, revisions []string) error {
		verified++
		return savedVerify(ctx, root, m, revisions)
	}
	root, extra := multiRepositoryFixture(t, [3]int{3, 3, 3})
	r, e := New(root, "corpus.json")
	if e != nil {
		t.Fatal(e)
	}
	call := func(wantFreshness, wantState string) Timing {
		t.Helper()
		timing, state, _ := freshnessCall(t, r, "corvint.docs_get", `{"id":"adapter:lib2-1"}`)
		if timing.Freshness != wantFreshness || state != wantState {
			t.Fatalf("freshness=%q state=%q, want %q %q (%+v)", timing.Freshness, state, wantFreshness, wantState, timing)
		}
		return timing
	}
	expectGit := func(name string, wantObserved, wantVerified bool) {
		t.Helper()
		if (observed > 0) != wantObserved || (verified > 0) != wantVerified {
			t.Fatalf("%s: observed %d verified %d, want observe=%v verify=%v", name, observed, verified, wantObserved, wantVerified)
		}
		observed, verified = 0, 0
	}
	call("computed", "fresh")
	call("reused", "fresh") // pairs the stamp with an observation and a verification
	expectGit("pairing call", true, true)
	for range 4 {
		call("reused", "fresh")
	}
	expectGit("warm calls with unchanged checkouts", false, false)

	page := filepath.Join(extra[1], "guide2", "p00001.md")
	original, err := os.ReadFile(page)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(page, []byte("# changed and longer\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	call("computed", "stale")
	expectGit("edited additional-repository page", true, true)
	call("reused", "stale")
	expectGit("unchanged stale checkouts", false, false)
	if err := os.WriteFile(page, original, 0o600); err != nil {
		t.Fatal(err)
	}
	call("computed", "fresh")
	expectGit("restored page", true, true)

	if err := os.WriteFile(filepath.Join(root, "untracked.txt"), []byte("x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	call("computed", "fresh")
	expectGit("untracked primary file", true, true)

	if err := os.WriteFile(filepath.Join(extra[0], "note.txt"), []byte("x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	stampGit(t, extra[0], "add", "note.txt")
	stampGit(t, extra[0], "commit", "-qm", "note")
	call("computed", "fresh")
	expectGit("commit in an additional repository", true, true)
	call("reused", "fresh")
	expectGit("settled checkouts", false, false)
}
