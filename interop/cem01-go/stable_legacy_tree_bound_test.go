package main

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// legacyTreeCommit commits a flat root tree of n entries that all name one
// blob, built through the index so no work-tree files are written.
func legacyTreeCommit(t *testing.T, dir string, n int) string {
	t.Helper()
	blob := strings.TrimSpace(gitRunInput(t, dir, "x\n", "hash-object", "-w", "--stdin"))
	var index strings.Builder
	for i := 0; i < n; i++ {
		fmt.Fprintf(&index, "100644 %s\tf%05d\n", blob, i)
	}
	gitRun(t, dir, "read-tree", "--empty")
	gitRunInput(t, dir, index.String(), "update-index", "--index-info")
	tree := strings.TrimSpace(gitRun(t, dir, "write-tree"))
	return strings.TrimSpace(gitRunInput(t, dir, "", "commit-tree", tree, "-m", fmt.Sprintf("%d entries", n)))
}

func gitRunInput(t *testing.T, dir, input string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "maintenance.auto=false", "-c", "gc.auto=0", "-C", dir}, args...)...)
	cmd.Env = append(cmd.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null",
		"GIT_AUTHOR_NAME=CEM Test", "GIT_AUTHOR_EMAIL=cem@example.invalid",
		"GIT_COMMITTER_NAME=CEM Test", "GIT_COMMITTER_EMAIL=cem@example.invalid")
	cmd.Stdin = strings.NewReader(input)
	b, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return string(b)
}

// CEM-V1-352 (V1-1472): the legacy portable `verify-stable` route keeps its
// declared narrower envelope of 4096 tree entries per snapshot. A snapshot of
// exactly 4096 entries reads; one more refuses unsupported-resource-limit, so
// a repository above the bound is refused, never partly read.
func TestCEMV1352LegacySnapshotTreeEntryBound(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	gitRun(t, dir, "init", "-q")
	at := legacyTreeCommit(t, dir, 4096)
	over := legacyTreeCommit(t, dir, 4097)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	v := &verifier{ctx: ctx, repo: dir, blobs: map[string][]byte{}, trees: map[string]*treeEntry{}}
	entries, e := v.snapshot03(at)
	if e != nil || len(entries) != 4096 {
		t.Fatalf("4096 entries: %d entries, err %v", len(entries), e)
	}
	v = &verifier{ctx: ctx, repo: dir, blobs: map[string][]byte{}, trees: map[string]*treeEntry{}}
	entries, e = v.snapshot03(over)
	if e == nil || e.code != "unsupported-resource-limit" || !e.operational || entries != nil {
		t.Fatalf("4097 entries: %d entries, err %#v", len(entries), e)
	}
}
