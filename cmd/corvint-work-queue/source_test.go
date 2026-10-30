package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/worksource"
)

// WQO-V0-004,005 / VPO-V0-010: producer acquisition honors the script-owned
// tuple for cleanup nesting and refuses malformed/rebound ownership markers.
func TestProducerOwnedSourceScratch(t *testing.T) {
	root, _ := queueFixture(t, nil)
	root, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	source, err := worksource.Acquire(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	temporary := t.TempDir()
	if err := os.Chmod(temporary, 0700); err != nil {
		t.Fatal(err)
	}
	owner := filepath.Join(temporary, "corvint-work-queue-owner")
	if err := os.Mkdir(owner, 0700); err != nil {
		t.Fatal(err)
	}
	tuple := filepath.Join(owner, "tuple")
	raw := []byte(root + "\n" + source.Identity.Commit + "\n")
	if err := os.WriteFile(tuple, raw, 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TMPDIR", temporary)
	scratch, err := producerOwnedScratch()
	if err != nil || scratch.root != root || scratch.commit != source.Identity.Commit {
		t.Fatal("valid tuple rejected", err)
	}
	resolved, err := worksource.ResolveRootWithScratch(context.Background(), root, scratch.parent)
	if err != nil || resolved != root {
		t.Fatal("owned root resolution failed", err)
	}
	if err := os.Chmod(tuple, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := producerOwnedScratch(); err == nil {
		t.Fatal("non-private tuple accepted")
	}
	if err := os.Remove(tuple); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "docs/worklist.json"), tuple); err != nil {
		t.Fatal(err)
	}
	if _, err := producerOwnedScratch(); err == nil {
		t.Fatal("symlink tuple accepted")
	}
}

// WQO-V0-004,005: matching tuple text in external caller Git metadata remains
// untrusted and is rejected before either root discovery or acquisition writes.
func TestProducerRejectsForgedCommonScratch(t *testing.T) {
	original, _ := queueFixture(t, nil)
	linked := t.TempDir()
	if _, err := git(original, "worktree", "add", "--quiet", "--detach", linked, "HEAD"); err != nil {
		t.Fatal(err)
	}
	linked, err := filepath.EvalSymlinks(linked)
	if err != nil {
		t.Fatal(err)
	}
	source, err := worksource.Acquire(context.Background(), linked)
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	temporary := filepath.Join(source.CommonDir, "forged-source-owner")
	owner := filepath.Join(temporary, "corvint-work-queue-owner")
	if err := os.Mkdir(temporary, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(owner, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(owner, "tuple"), []byte(linked+"\n"+source.Identity.Commit+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TMPDIR", temporary)
	if _, err := producerOwnedScratch(); err == nil {
		t.Fatal("forged common Git tuple admitted")
	}
	entries, err := os.ReadDir(temporary)
	if err != nil || len(entries) != 1 || entries[0].Name() != "corvint-work-queue-owner" {
		t.Fatal("forged tuple validation created scratch")
	}
}
