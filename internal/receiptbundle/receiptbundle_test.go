package receiptbundle

import (
	"bytes"
	"context"
	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

const mapPath = wire.ExcludedCEMPath

type fixture struct {
	root, base, target string
}

func fixtureGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-c", "maintenance.auto=false", "-c", "gc.auto=0"}, args...)...)
	command.Dir = dir
	command.Env = append(os.Environ(),
		"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_SYSTEM="+os.DevNull,
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.invalid",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.invalid",
		"GIT_AUTHOR_DATE=2000-01-01T00:00:00+0000", "GIT_COMMITTER_DATE=2000-01-01T00:00:00+0000")
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	if err := command.Run(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, stderr.String())
	}
	return strings.TrimSpace(stdout.String())
}

func writeFixture(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// newFixture commits a base and a change: a repository whose worktree and Git
// directories the output check protects (RCB-V0-005).
func newFixture(t *testing.T) fixture {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	fixtureGit(t, root, "init", "-q", "-b", "main")
	writeFixture(t, filepath.Join(root, "a.txt"), "base\n")
	fixtureGit(t, root, "add", ".")
	fixtureGit(t, root, "commit", "-qm", "base")
	base := fixtureGit(t, root, "rev-parse", "HEAD")
	writeFixture(t, filepath.Join(root, "a.txt"), "target\n")
	fixtureGit(t, root, "commit", "-qam", "target")
	return fixture{root: root, base: base, target: fixtureGit(t, root, "rev-parse", "HEAD")}
}

func TestExportRefusesOutputsItMustNotWrite(t *testing.T) {
	f := newFixture(t)
	existing := t.TempDir()
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(f.root, link); err != nil {
		t.Fatal(err)
	}
	cases := map[string]string{
		"relative":         "bundle",
		"existing":         existing,
		"worktree":         filepath.Join(f.root, "bundle"),
		"git directory":    filepath.Join(f.root, ".git", "bundle"),
		"no parent":        filepath.Join(existing, "missing", "bundle"),
		"symlinked parent": filepath.Join(link, "bundle"),
	}
	for name, output := range cases {
		refuse(t, f.root, name, output)
	}
	if _, err := os.Stat(filepath.Join(f.root, "bundle")); !os.IsNotExist(err) {
		t.Fatalf("a refused output was created: %v", err)
	}
}

// refuse asserts that the output check shared by every export refuses output
// from root (RCB-V0-005, CEM-V1-064). It is driven directly because the only
// export left, the cem/1.0 one, verifies its source before it checks output.
func refuse(t *testing.T, root, name, output string) {
	t.Helper()
	repo, err := gitauth.Open(root, gitrun.NewDefaultBudget())
	if err != nil {
		t.Fatal(err)
	}
	parent, _, err := outputParent(repo, output)
	if parent != nil {
		parent.Close()
	}
	if cemcode.CodeOf(err) != cemcode.BundleOutputRefused {
		t.Fatalf("%s: %v, want %s", name, err, cemcode.BundleOutputRefused)
	}
}

// TestExportRefusesACaseVariantOfTheWorktree: the output check compares file
// identity, so a case variant of the worktree is refused where the volume
// folds case.
func TestExportRefusesACaseVariantOfTheWorktree(t *testing.T) {
	f := newFixture(t)
	variant := strings.ToUpper(f.root)
	original, _ := os.Stat(f.root)
	folded, err := os.Stat(variant)
	if variant == f.root || err != nil || !os.SameFile(original, folded) {
		t.Skip("the temporary volume is case-sensitive")
	}
	refuse(t, f.root, "case variant", filepath.Join(variant, "bundle"))
}

// TestExportRefusesEveryWorktreeAndGitDirectory: a Git directory outside the
// worktree, the primary worktree seen from a linked one, and a sibling linked
// worktree are all protected.
func TestExportRefusesEveryWorktreeAndGitDirectory(t *testing.T) {
	f := newFixture(t)
	bare := filepath.Join(t.TempDir(), "bare.git")
	fixtureGit(t, f.root, "clone", "-q", "--bare", f.root, bare)
	linked := filepath.Join(t.TempDir(), "linked")
	fixtureGit(t, bare, "worktree", "add", "-q", linked, "main")
	refuse(t, linked, "common directory outside the worktree", filepath.Join(bare, "bundle"))
	refuse(t, linked, "git directory outside the worktree", filepath.Join(bare, "worktrees", "linked", "bundle"))

	// core.worktree names the primary worktree of a Git directory kept elsewhere.
	primary := t.TempDir()
	fixtureGit(t, bare, "config", "core.worktree", primary)
	refuse(t, linked, "core.worktree", filepath.Join(primary, "bundle"))

	first := filepath.Join(t.TempDir(), "first")
	second := filepath.Join(t.TempDir(), "second")
	fixtureGit(t, f.root, "worktree", "add", "-q", "-b", "first", first)
	fixtureGit(t, f.root, "worktree", "add", "-q", "-b", "second", second)
	refuse(t, first, "primary worktree", filepath.Join(f.root, "bundle"))
	refuse(t, first, "sibling worktree", filepath.Join(second, "bundle"))
	refuse(t, f.root, "linked worktree", filepath.Join(first, "bundle"))
}

// TestExportRefusesRetiredMaps checks CEM-V1-210: a missing map is
// unavailable, a cem/0.x map refuses as unsupported-spec with the retired-spec
// fix, and neither leaves a bundle.
func TestExportRefusesRetiredMaps(t *testing.T) {
	f := newFixture(t)
	path := filepath.Join(f.root, mapPath)
	for name, c := range map[string]struct{ content, code string }{
		"missing": {"", cemcode.MapUnavailable},
		"cem/0.2": {`{"spec":"cem/0.2"}`, cemcode.UnsupportedSpec},
	} {
		os.Remove(path)
		if c.content != "" {
			writeFixture(t, path, c.content)
		}
		output := filepath.Join(t.TempDir(), "bundle")
		_, err := Export(context.Background(), f.root, Options{MapPath: mapPath, ExpectedBase: f.base, Target: f.target, Output: output})
		if cemcode.CodeOf(err) != c.code || (c.code == cemcode.UnsupportedSpec && !strings.Contains(err.Error(), wire.RetiredSpecFix)) {
			t.Fatalf("%s map: %v, want %s", name, err, c.code)
		}
		if _, statErr := os.Stat(output); !os.IsNotExist(statErr) {
			t.Fatalf("%s map left a bundle: %v", name, statErr)
		}
	}
}
