//go:build darwin || linux

package gitauth

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
)

// TestOpenStableSymlinkedPathIsRefusedUpFront pins V1-2519: a repository
// root, or a gitfile target, reached through a symbolic link is refused by
// OpenStable with repository-object-unavailable and a detail naming the
// link, never admitted and then refused by Validate as a boundary change
// (CEM-V1-960).
func TestOpenStableSymlinkedPathIsRefusedUpFront_V12519(t *testing.T) {
	t.Parallel()
	l := newStableLayout(t)
	link := filepath.Join(l.base, "link")
	if err := os.Symlink(l.base, link); err != nil {
		t.Fatal(err)
	}
	leaf := filepath.Join(l.base, "repo-link")
	if err := os.Symlink(l.repo, leaf); err != nil {
		t.Fatal(err)
	}
	viaLink := stableLayout{base: l.base, repo: l.repo, worktree: filepath.Join(l.base, "wt2")}
	viaLink.admin = filepath.Join(l.git, "worktrees", "wt2")
	for _, directory := range []string{viaLink.admin, viaLink.worktree} {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	viaLink.write(t, filepath.Join(viaLink.worktree, ".git"), "gitdir: "+filepath.Join(link, "repo", ".git", "worktrees", "wt2")+"\n")
	viaLink.write(t, filepath.Join(viaLink.admin, "gitdir"), viaLink.worktree+"/.git\n")
	viaLink.write(t, filepath.Join(viaLink.admin, "commondir"), "../..\n")
	for name, root := range map[string]string{
		"ancestor link":      filepath.Join(link, "repo"),
		"root link":          leaf,
		"gitfile names link": viaLink.worktree,
	} {
		_, boundary, err := openStableForTest(t, root, nil)
		if err == nil {
			err = boundary.Validate()
			t.Errorf("%s: OpenStable admitted a symlinked path; Validate = %v", name, err)
			continue
		}
		if cemcode.CodeOf(err) != cemcode.RepositoryObjectUnavailable || !strings.Contains(err.Error(), "is reached through the symbolic link "+link) && !strings.Contains(err.Error(), "is reached through the symbolic link "+leaf) {
			t.Errorf("%s: code = %q (%v), want %q naming the link", name, cemcode.CodeOf(err), err, cemcode.RepositoryObjectUnavailable)
		}
	}
}

// TestStableBoundaryResolvedPathValidates_V12519 pins the accepted side: the
// same repository opened by its resolved path validates unchanged.
func TestStableBoundaryResolvedPathValidates_V12519(t *testing.T) {
	t.Parallel()
	l := newStableLayout(t)
	link := filepath.Join(l.base, "link")
	if err := os.Symlink(l.base, link); err != nil {
		t.Fatal(err)
	}
	resolved, err := filepath.EvalSymlinks(filepath.Join(link, "repo"))
	if err != nil {
		t.Fatal(err)
	}
	_, boundary, err := openStableForTest(t, resolved, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := boundary.Validate(); err != nil {
		t.Fatalf("resolved path: Validate = %v", err)
	}
}
