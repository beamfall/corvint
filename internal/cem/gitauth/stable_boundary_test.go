//go:build darwin || linux

package gitauth

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
)

func TestStableMetadataLineGrammar(t *testing.T) {
	cells := []struct {
		name, data string
		ok         bool
	}{
		{"no-final-LF", "../..", true},
		{"one-final-LF", "../..\n", true},
		{"interior-space-kept", "a b\n", true},
		{"empty", "", false},
		{"only-LF", "\n", false},
		{"two-final-LF", "../..\n\n", false},
		{"CRLF", "../..\r\n", false},
		{"bare-CR", "a\rb\n", false},
		{"NUL", "a\x00b\n", false},
		{"two-lines", "a\nb\n", false},
		{"invalid-UTF8", "a\xffb\n", false},
	}
	for _, cell := range cells {
		line, err := stableMetadataLine([]byte(cell.data))
		if (err == nil) != cell.ok {
			t.Errorf("%s: err = %v, want ok=%v", cell.name, err, cell.ok)
		}
		if cell.ok && line != strings.TrimSuffix(cell.data, "\n") {
			t.Errorf("%s: line = %q; nothing but one final LF may be removed", cell.name, line)
		}
		if !cell.ok && cemcode.CodeOf(err) != cemcode.RepositoryObjectUnavailable {
			t.Errorf("%s: code = %q", cell.name, cemcode.CodeOf(err))
		}
	}
}

// stableLayout is topology T-LINKED: R=B/repo, G=R/.git, A=G/worktrees/wt,
// W=B/wt, under a symlink-free base.
type stableLayout struct{ base, repo, git, admin, worktree string }

func newStableLayout(t *testing.T) stableLayout {
	t.Helper()
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	l := stableLayout{base: base, repo: filepath.Join(base, "repo")}
	l.git = filepath.Join(l.repo, ".git")
	l.admin = filepath.Join(l.git, "worktrees", "wt")
	l.worktree = filepath.Join(base, "wt")
	command := exec.Command("git", "init", "-q", l.repo)
	command.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1")
	if out, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	for _, directory := range []string{l.admin, l.worktree} {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	l.write(t, filepath.Join(l.worktree, ".git"), "gitdir: "+l.admin+"\n")
	l.write(t, filepath.Join(l.admin, "gitdir"), l.worktree+"/.git\n")
	l.write(t, filepath.Join(l.admin, "commondir"), "../..\n")
	return l
}

func (stableLayout) write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func openStableForTest(t *testing.T, root string, fault StableFault) (*Repository, *StableBoundary, error) {
	t.Helper()
	repository, boundary, err := OpenStable(root, nil, fault)
	if boundary != nil {
		t.Cleanup(boundary.Close)
	}
	return repository, boundary, err
}

func TestOpenStableAdmitsPrimaryAndLinked(t *testing.T) {
	l := newStableLayout(t)
	primary, boundary, err := openStableForTest(t, l.repo, nil)
	if err != nil {
		t.Fatal(err)
	}
	if primary.GitDir != l.git || primary.CommonDir != l.git {
		t.Fatalf("primary = %+v", primary)
	}
	if err := boundary.Validate(); err != nil {
		t.Fatalf("unchanged boundary: %v", err)
	}
	linked, boundary, err := openStableForTest(t, l.worktree, nil)
	if err != nil {
		t.Fatal(err)
	}
	if linked.GitDir != l.admin || linked.CommonDir != l.git {
		t.Fatalf("linked = %+v", linked)
	}
	if err := boundary.Validate(); err != nil {
		t.Fatalf("unchanged linked boundary: %v", err)
	}
}

func TestOpenStableRefusals(t *testing.T) {
	cases := []struct {
		name   string
		linked bool
		mutate func(t *testing.T, l stableLayout) (root string, fault StableFault)
		want   string
	}{
		{"commondir-empty", true, func(t *testing.T, l stableLayout) (string, StableFault) {
			l.write(t, filepath.Join(l.admin, "commondir"), "")
			return l.worktree, nil
		}, cemcode.RepositoryObjectUnavailable},
		{"commondir-absolute", true, func(t *testing.T, l stableLayout) (string, StableFault) {
			l.write(t, filepath.Join(l.admin, "commondir"), l.git+"\n")
			return l.worktree, nil
		}, cemcode.RepositoryObjectUnavailable},
		{"marker-CRLF", true, func(t *testing.T, l stableLayout) (string, StableFault) {
			l.write(t, filepath.Join(l.worktree, ".git"), "gitdir: "+l.admin+"\r\n")
			return l.worktree, nil
		}, cemcode.RepositoryObjectUnavailable},
		{"marker-4097-bytes", true, func(t *testing.T, l stableLayout) (string, StableFault) {
			prefix := "gitdir: " + l.admin
			content := prefix + strings.Repeat("/.", (4096-len(prefix))/2)
			for len(content) < 4096 {
				content += "/"
			}
			l.write(t, filepath.Join(l.worktree, ".git"), content+"\n")
			return l.worktree, nil
		}, cemcode.RepositoryObjectUnavailable},
		{"back-pointer-not-reciprocal", true, func(t *testing.T, l stableLayout) (string, StableFault) {
			l.write(t, filepath.Join(l.admin, "gitdir"), l.base+"/elsewhere/.git\n")
			return l.worktree, nil
		}, cemcode.RepositoryObjectUnavailable},
		{"commondir-EIO", true, func(t *testing.T, l stableLayout) (string, StableFault) {
			return l.worktree, faultOn(filepath.Join(l.admin, "commondir"), syscall.EIO)
		}, cemcode.RepositoryObjectUnavailable},
		{"objects-info-ELOOP", false, func(t *testing.T, l stableLayout) (string, StableFault) {
			return l.repo, faultOn(filepath.Join(l.git, "objects", "info"), syscall.ELOOP)
		}, cemcode.RepositoryObjectUnavailable},
		{"attributes-EACCES", false, func(t *testing.T, l stableLayout) (string, StableFault) {
			return l.repo, faultOn(filepath.Join(l.git, "info", "attributes"), syscall.EACCES)
		}, cemcode.RepositoryObjectUnavailable},
		{"root-through-symlink", false, func(t *testing.T, l stableLayout) (string, StableFault) {
			if err := os.Symlink(l.base, filepath.Join(l.base, "link")); err != nil {
				t.Fatal(err)
			}
			return filepath.Join(l.base, "link", "repo"), nil
		}, cemcode.RepositoryObjectUnavailable},
		{"alternates-present", false, func(t *testing.T, l stableLayout) (string, StableFault) {
			l.write(t, filepath.Join(l.git, "objects", "info", "alternates"), "/elsewhere/objects\n")
			return l.repo, nil
		}, cemcode.UnsupportedObjectAlternates},
		{"http-alternates-empty", false, func(t *testing.T, l stableLayout) (string, StableFault) {
			l.write(t, filepath.Join(l.git, "objects", "info", "http-alternates"), "")
			return l.repo, nil
		}, cemcode.UnsupportedObjectAlternates},
		{"attributes-nonempty", false, func(t *testing.T, l stableLayout) (string, StableFault) {
			l.write(t, filepath.Join(l.git, "info", "attributes"), "* text\n")
			return l.repo, nil
		}, cemcode.UnsupportedRepositoryAttributes},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			l := newStableLayout(t)
			for _, directory := range []string{filepath.Join(l.git, "info"), filepath.Join(l.git, "objects", "info")} {
				if err := os.MkdirAll(directory, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			root, fault := c.mutate(t, l)
			_, _, err := openStableForTest(t, root, fault)
			if cemcode.CodeOf(err) != c.want {
				t.Fatalf("err = %v (code %q), want code %q", err, cemcode.CodeOf(err), c.want)
			}
		})
	}
}

func faultOn(path string, errno syscall.Errno) StableFault {
	return func(inspected string) error {
		if inspected == path {
			return &fs.PathError{Op: "lstat", Path: path, Err: errno}
		}
		return nil
	}
}

// Validate re-observes the whole admitted boundary: a replaced directory, a
// replaced metadata file and a newly appearing optional leaf each refuse.
func TestStableBoundaryValidateDetectsChange(t *testing.T) {
	cases := []struct {
		name   string
		change func(t *testing.T, l stableLayout)
	}{
		{"root-replaced", func(t *testing.T, l stableLayout) {
			if err := os.Rename(l.repo, l.repo+".moved"); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(l.repo, 0o755); err != nil {
				t.Fatal(err)
			}
		}},
		{"objects-replaced", func(t *testing.T, l stableLayout) {
			objects := filepath.Join(l.git, "objects")
			if err := os.Rename(objects, objects+".moved"); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(objects, 0o755); err != nil {
				t.Fatal(err)
			}
		}},
		{"absent-alternates-appears", func(t *testing.T, l stableLayout) {
			l.write(t, filepath.Join(l.git, "objects", "info", "alternates"), "")
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			l := newStableLayout(t)
			if err := os.MkdirAll(filepath.Join(l.git, "objects", "info"), 0o755); err != nil {
				t.Fatal(err)
			}
			_, boundary, err := openStableForTest(t, l.repo, nil)
			if err != nil {
				t.Fatal(err)
			}
			c.change(t, l)
			if err := boundary.Validate(); cemcode.CodeOf(err) != cemcode.RepositoryObjectUnavailable {
				t.Fatalf("Validate after %s = %v", c.name, err)
			}
		})
	}
}

// The legacy grammar is unchanged: Open still admits a root that Stable
// admission refuses, so no legacy command moves from working to refused.
func TestLegacyOpenUnchangedByStableAdmission(t *testing.T) {
	l := newStableLayout(t)
	if err := os.MkdirAll(filepath.Join(l.git, "objects", "info"), 0o755); err != nil {
		t.Fatal(err)
	}
	l.write(t, filepath.Join(l.git, "objects", "info", "http-alternates"), "")
	if _, _, err := openStableForTest(t, l.repo, nil); cemcode.CodeOf(err) != cemcode.UnsupportedObjectAlternates {
		t.Fatalf("stable admission = %v", err)
	}
	if _, err := Open(l.repo, gitrun.NewDefaultBudget()); err != nil {
		t.Fatalf("legacy Open refused a root it admitted before: %v", err)
	}
}
