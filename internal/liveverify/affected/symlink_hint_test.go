package affected

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// AFP-V0-494 (GH #1751, V1-2794): an unowned dirty path that is a symbolic
// link carries the trailing-slash ignore hint; a regular unowned file carries
// none, as does a graph with no disk root, and selection and scope are identical either way.
func TestUnownedSymlinkCarriesIgnoreHint_AFPV0494(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "plain.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "target"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("target", filepath.Join(root, "node_modules")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	graph, err := Build(context.Background(), root, chain())
	if err != nil {
		t.Fatal(err)
	}
	dirty := []string{"core.x", "node_modules", "plain.txt"}
	plan := Select(graph, dirty)
	hints := map[string]string{}
	for _, unknown := range plan.Unknown {
		if unknown.Reason == UnknownUnownedDirtyPath {
			hints[unknown.Detail] = unknown.Hint
		}
	}
	if len(hints) != 2 || hints["node_modules"] != symlinkUnownedHint || hints["plain.txt"] != "" {
		t.Fatalf("only the symlink must carry the hint, with Detail the bare path: %+v", plan.Unknown)
	}
	graph.root = ""
	unhinted := Select(graph, dirty)
	for _, unknown := range unhinted.Unknown {
		if unknown.Hint != "" {
			t.Fatalf("a graph with no disk root must give no hint: %+v", unknown)
		}
	}
	if plan.Scope != ScopeUnknown || unhinted.Scope != plan.Scope || len(unhinted.Selected) != len(plan.Selected) || len(unhinted.Unknown) != len(plan.Unknown) {
		t.Fatalf("the hint must not change selection or scope: %+v vs %+v", plan, unhinted)
	}
}
