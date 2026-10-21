package contextindex

import (
	"reflect"
	"testing"
)

// TCP-V0-141 (V1-1285): a dirty worktree is disclosed beside the committed
// tree the packet reads; a clean one leaves coverage untouched.
func TestTaskContextDisclosesDirtyWorktreeAgainstCommittedTree(t *testing.T) {
	clean := map[string]any{"uncertainty": []any{"kept"}}
	worktreeCoverage(clean, &Index{CommitRevision: "c0", Revision: "t0"})
	if !reflect.DeepEqual(clean, map[string]any{"uncertainty": []any{"kept"}}) {
		t.Fatalf("clean worktree changed coverage: %v", clean)
	}
	empty := map[string]any{}
	worktreeCoverage(empty, &Index{})
	if _, present := empty["uncertainty"]; present {
		t.Fatalf("clean worktree added uncertainty: %v", empty)
	}
	dirty := map[string]any{"uncertainty": []any{"kept"}}
	worktreeCoverage(dirty, &Index{CommitRevision: "c1", Revision: "t1", DirtyPaths: []string{"a.go", "b.md"}})
	want := []any{"kept", `the worktree has 2 uncommitted paths; this packet reads commit c1 (tree t1, the "revision"), so uncommitted edits are not evidence`}
	if !reflect.DeepEqual(dirty["uncertainty"], want) {
		t.Fatalf("dirty uncertainty = %v, want %v", dirty["uncertainty"], want)
	}
}
