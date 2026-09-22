package worksource

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// WQO-V0-004,005 / VPO-V0-010: case-preserving path resolution is not proof of
// distinct directories. An alias of external common metadata is rejected before
// either explicit scratch seam can create files.
func TestWorkSourceRejectsScratchAncestorAliasBeforeWrites(t *testing.T) {
	original := fixture(t)
	linked := t.TempDir()
	runGit(t, original, "worktree", "add", "--quiet", "--detach", linked, "HEAD")
	linked, err := filepath.EvalSymlinks(linked)
	if err != nil {
		t.Fatal(err)
	}
	common := filepath.Join(original, ".git")
	alias := filepath.Join(original, ".GIT")
	actual, err := os.Stat(common)
	if err != nil {
		t.Fatal(err)
	}
	alternate, err := os.Stat(alias)
	if err != nil || !os.SameFile(actual, alternate) {
		t.Skip("filesystem has no alternate-case directory alias")
	}
	scratch := filepath.Join(alias, "scratch-child")
	if err := os.Mkdir(scratch, 0700); err != nil {
		t.Fatal(err)
	}
	resolved, err := filepath.EvalSymlinks(scratch)
	if err != nil {
		t.Fatal(err)
	}
	if strings.HasPrefix(resolved, common+string(os.PathSeparator)) {
		t.Skip("platform normalizes alias spelling during EvalSymlinks")
	}
	before := gitManifest(t, common)
	if source, err := AcquireWithScratch(context.Background(), linked, scratch); err == nil {
		source.Close()
		t.Fatal("aliased common metadata scratch admitted")
	}
	if _, err := ResolveRootWithScratch(context.Background(), filepath.Join(linked, "directory"), scratch); err == nil {
		t.Fatal("aliased common metadata root-discovery scratch admitted")
	}
	if after := gitManifest(t, common); !bytes.Equal(before, after) {
		t.Fatal("aliased scratch refusal wrote caller metadata")
	}
}

// VPO-V0-010: relative spelling must be absolutized before walking ancestors;
// reaching "." is not proof that an external common Git root was not crossed.
func TestWorkSourceRejectsRelativeScratchInsideCommon(t *testing.T) {
	original := fixture(t)
	linked := t.TempDir()
	runGit(t, original, "worktree", "add", "--quiet", "--detach", linked, "HEAD")
	linked, err := filepath.EvalSymlinks(linked)
	if err != nil {
		t.Fatal(err)
	}
	common := filepath.Join(original, ".git")
	nested := filepath.Join(common, "nested-cwd")
	if err := os.MkdirAll(filepath.Join(nested, "scratch"), 0700); err != nil {
		t.Fatal(err)
	}
	before := gitManifest(t, common)
	t.Chdir(nested)
	if source, err := AcquireWithScratch(context.Background(), linked, "scratch"); err == nil {
		source.Close()
		t.Fatal("relative scratch below external common metadata admitted")
	}
	if after := gitManifest(t, common); !bytes.Equal(before, after) {
		t.Fatal("relative scratch refusal wrote caller metadata")
	}
}
