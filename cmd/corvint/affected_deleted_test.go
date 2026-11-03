package main

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// TestAffectedPlanMarksDeletedPaths_AFPV02206 pins V1-3864: a committed
// deletion and a worktree deletion are dirty paths the plan names in
// plan.deleted, in dirty order, and the compact form carries the same list;
// an edited file and an added one are not deleted.
func TestAffectedPlanMarksDeletedPaths_AFPV02206(t *testing.T) {
	root := t.TempDir()
	write := func(name, text string) {
		t.Helper()
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module example.test/d\n")
	write("a/a.go", "package a\n")
	write("b/b.go", "package b\n")
	write("c/c.go", "package c\n")
	write("c/c_test.go", "package c\n\nimport \"testing\"\n\nfunc TestC(t *testing.T) {}\n")
	commit := func(message string) {
		materializationGit(t, root, "add", "-A")
		materializationGit(t, root, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "-q", "-m", message)
	}
	materializationGit(t, root, "init", "-q")
	commit("base")
	base := strings.TrimSpace(materializationGit(t, root, "rev-parse", "HEAD"))
	if err := os.Remove(filepath.Join(root, "a", "a.go")); err != nil {
		t.Fatal(err)
	}
	write("d/d.go", "package d\n")
	commit("delete a, add d")
	if err := os.Remove(filepath.Join(root, "b", "b.go")); err != nil {
		t.Fatal(err)
	}
	write("c/c.go", "package c\n\nconst C = 1\n")
	receipt, err := compileAffected(context.Background(), affectedInvocation{Root: root, Base: base})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"a/a.go", "b/b.go"}; !slices.Equal(receipt.Plan.Deleted, want) {
		t.Fatalf("deleted = %v, want %v (dirty %v)", receipt.Plan.Deleted, want, receipt.Plan.Dirty)
	}
	compact, err := compactAffected(receipt)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(compact.Plan.Deleted, receipt.Plan.Deleted) {
		t.Fatalf("compact deleted = %v, want %v", compact.Plan.Deleted, receipt.Plan.Deleted)
	}
	if got := affectedDeletedPaths(root, []string{"c/c.go", "d/d.go"}); len(got) != 0 {
		t.Fatalf("present paths read as deleted: %v", got)
	}
	// A directory replaced by a file deletes every path below it, though
	// Lstat reports ENOTDIR rather than a missing file.
	write("e", "now a file\n")
	if got := affectedDeletedPaths(root, []string{"e/inner.go"}); !slices.Equal(got, []string{"e/inner.go"}) {
		t.Fatalf("path below a file = %v, want deleted", got)
	}
}
