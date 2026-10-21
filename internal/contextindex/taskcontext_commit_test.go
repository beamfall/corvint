package contextindex

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TCP-V0-150 (V1-2127): `revision` stays the indexed tree id (HEAD's tree,
// also on a dirty worktree); the packet adds the HEAD commit as `commit`.
// TCP-V0-151 (V1-1285): a source-dirty worktree is disclosed in `worktree`.
func TestTaskContextCarriesHeadCommitBesideTreeRevision(t *testing.T) {
	root := impactRepositoryWithFiles(t, map[string]string{
		"go.mod":  "module example.test/commit\n\ngo 1.27.0\n",
		"main.go": "package main\n\nfunc MintToken() {}\n",
	})
	head := strings.TrimSpace(testGit(t, root, "rev-parse", "HEAD"))
	tree := strings.TrimSpace(testGit(t, root, "rev-parse", "HEAD^{tree}"))
	check := func(name string) {
		t.Helper()
		index, err := Build(context.Background(), root)
		if err != nil {
			t.Fatal(err)
		}
		packet, err := TaskContext(context.Background(), index, "change MintToken", "", 10)
		if err != nil {
			t.Fatal(err)
		}
		worktree, present := packet["worktree"].(map[string]any)
		if present != (name == "dirty") {
			t.Fatalf("%s: worktree member present %v: %v", name, present, packet["worktree"])
		}
		if present && (worktree["state"] != "MIXED" || worktree["dirty_path_count"] != 1 || len(worktree["dirty_paths"].([]string)) != 1 || worktree["dirty_paths"].([]string)[0] != "main.go") {
			t.Fatalf("dirty worktree disclosure: %v", worktree)
		}
		lookup, err := LookupDefinitions(index, "MintToken", 10)
		if err != nil {
			t.Fatal(err)
		}
		for kind, got := range map[string]map[string]any{"task": packet, "defs": lookup} {
			if got["commit"] != head {
				t.Fatalf("%s %s: commit = %v, want HEAD %s", name, kind, got["commit"], head)
			}
			if got["revision"] != tree {
				t.Fatalf("%s %s: revision = %v, HEAD tree %s, HEAD commit %s", name, kind, got["revision"], tree, head)
			}
		}
	}
	check("clean")
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n\nfunc MintToken() { _ = 1 }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	check("dirty")
}
