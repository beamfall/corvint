// SPDX-License-Identifier: AGPL-3.0-or-later
package lspstdio

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkspaceAdmissionGitlink(t *testing.T) {
	root, _ := filepath.EvalSymlinks(contextFixture(t))
	if _, err := admitWorkspace(context.Background(), root); err != nil {
		t.Fatal(err)
	}
	commit := strings.TrimSpace(string(fixtureGit(t, root, "rev-parse", "HEAD")))
	fixtureGit(t, root, "update-index", "--add", "--cacheinfo", "160000,"+commit+",submodule")
	if _, err := admitWorkspace(context.Background(), root); err == nil {
		t.Fatal("index gitlink admitted")
	}
	fixtureGit(t, root, "-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "gitlink")
	os.Mkdir(filepath.Join(root, "submodule"), 0700)
	if _, err := admitWorkspace(context.Background(), root); err == nil {
		t.Fatal("empty committed gitlink admitted")
	}
}

type countedWorkspaceReader struct{ asked int }

func (r *countedWorkspaceReader) ReadDir(n int) ([]os.DirEntry, error) {
	r.asked += n
	return make([]os.DirEntry, n), nil
}
func TestWorkspaceChunkBounds(t *testing.T) {
	r := &countedWorkspaceReader{}
	if _, err := readWorkspaceChunk(context.Background(), r, 3); err == nil || r.asked != 4 {
		t.Fatalf("overflow read %d: %v", r.asked, err)
	}
	ctx, stop := context.WithCancel(context.Background())
	stop()
	r.asked = 0
	if _, err := readWorkspaceChunk(ctx, r, 3); err == nil || r.asked != 0 {
		t.Fatal("cancelled read allocated")
	}
}
func TestWorkspaceDirectoryInstability(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "a"), []byte("a"), 0600)
	changed := false
	err := boundedWorkspaceWalk(context.Background(), root, 10, func(_ string, _ os.DirEntry, _ error) error {
		if !changed {
			changed = true
			return os.WriteFile(filepath.Join(root, "b"), []byte("b"), 0600)
		}
		return nil
	})
	if err == nil {
		t.Fatal("changed directory accepted")
	}
	if !changed || err == io.EOF {
		t.Fatal("control did not run")
	}
}
func TestWorkspaceGitControlBytes(t *testing.T) {
	for i := 0; i < 32; i++ {
		if !hasGitControl("path" + string(rune(i))) {
			t.Fatalf("control %d", i)
		}
	}
	if !hasGitControl("path\x7f") || hasGitControl("normal/path") {
		t.Fatal("control parser")
	}
}

func TestWorkspaceAdmissionLinkedRoot(t *testing.T) {
	root, _ := filepath.EvalSymlinks(contextFixture(t))
	parent, _ := filepath.EvalSymlinks(t.TempDir())
	linked := filepath.Join(parent, "linked")
	fixtureGit(t, root, "worktree", "add", "-qb", "admission-linked", linked, "HEAD")
	if _, err := admitWorkspace(context.Background(), linked); err != nil {
		t.Fatal(err)
	}
}
