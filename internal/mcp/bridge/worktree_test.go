package bridge

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMCPLinkedWorktreeBinding(t *testing.T) {
	t.Run("MCPV0-001 linked roots bind their own clean and dirty state", func(t *testing.T) {
		root := makeRepository(t)
		linked := filepath.Join(t.TempDir(), "linked")
		gitOutput(t, root, "worktree", "add", "-qb", "linked", linked, "HEAD")
		gitOutput(t, linked, "commit", "--allow-empty", "-qm", "linked head")
		registry, err := New(linked)
		if err != nil {
			t.Fatal(err)
		}
		result, callErr := registry.Call(context.Background(), ToolStatus, []byte(`{}`))
		if callErr != nil || result.Repository == nil {
			t.Fatalf("%+v %v", result, callErr)
		}
		if result.Repository.CommitRevision != strings.TrimSpace(string(gitOutput(t, linked, "rev-parse", "HEAD"))) || result.Repository.WorktreeState != "CLEAN" {
			t.Fatalf("wrong worktree binding: %+v", result)
		}
		writeFile(t, filepath.Join(linked, "untracked.go"), "package untracked\n")
		dirty, callErr := registry.Call(context.Background(), ToolStatus, []byte(`{}`))
		if callErr != nil || dirty.Repository == nil || dirty.Repository.WorktreeState != "MIXED" || dirty.Repository.DirtyPathCount != 1 || dirty.Repository.DirtyPathsSHA256 == result.Repository.DirtyPathsSHA256 {
			t.Fatalf("dirty binding: %+v %v", dirty, callErr)
		}
		marker := filepath.Join(linked, ".git")
		writeFile(t, marker, "gitdir: "+filepath.Join(root, ".git")+"\n")
		refused, callErr := registry.Call(context.Background(), ToolStatus, []byte(`{}`))
		if callErr != nil || refused.Abstention.Reason != "ROOT_IDENTITY_CHANGED" {
			t.Fatalf("retarget accepted: %+v %v", refused, callErr)
		}
	})
}

func TestMCPForgedWorktreeRejected(t *testing.T) {
	root := makeRepository(t)
	forged := t.TempDir()
	if err := os.WriteFile(filepath.Join(forged, ".git"), []byte("gitdir: "+filepath.Join(root, ".git")+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := New(forged); err == nil {
		t.Fatal("forged worktree accepted")
	}
}

func TestMCPOpaqueGitlinkStatus(t *testing.T) {
	root := makeRepository(t)
	oid := strings.TrimSpace(string(gitOutput(t, root, "rev-parse", "HEAD")))
	gitOutput(t, root, "update-index", "--add", "--cacheinfo", "160000,"+oid+",vendored")
	gitOutput(t, root, "commit", "-qm", "opaque gitlink")
	if err := os.Mkdir(filepath.Join(root, "vendored"), 0700); err != nil {
		t.Fatal(err)
	}
	registry, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	result, callErr := registry.Call(context.Background(), ToolStatus, []byte(`{}`))
	if callErr != nil || result.Repository == nil || result.Repository.WorktreeState != "CLEAN" {
		t.Fatalf("gitlink binding=%+v %v", result, callErr)
	}
}
