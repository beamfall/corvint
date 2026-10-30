package projectpath

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// AHI-014, URE-V0-001: filesystem aliases and unresolved paths share one contract.
func TestRelativeAliasesAndUncertainty(t *testing.T) {
	root := t.TempDir()
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(t.TempDir(), "root")
	if err := os.Symlink(root, alias); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "file"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{"dir", "dir/sub", "..name"} {
		if err := os.Mkdir(filepath.Join(root, dir), 0700); err != nil {
			t.Fatal(err)
		}
	}
	for _, length := range []int{40, 41} {
		for link := 1; link <= length; link++ {
			target := fmt.Sprintf("chain%d-%d", length, link+1)
			if link == length {
				target = "missing"
			}
			if err := os.Symlink(target, filepath.Join(root, fmt.Sprintf("chain%d-%d", length, link))); err != nil {
				t.Fatal(err)
			}
		}
	}
	for name, target := range map[string]string{
		"dangling":         "missing/child",
		"through-dangling": "dangling",
		"dir-link":         "dir",
		"cycle":            "cycle",
		"escape":           t.TempDir(),
		"sub-link":         "dir/sub",
		"physical":         "sub-link/../missing",
	} {
		if err := os.Symlink(target, filepath.Join(root, name)); err != nil {
			t.Fatal(err)
		}
	}
	for _, test := range []struct {
		name string
		root string
		path string
		want string
	}{
		{"absolute alias", alias, filepath.Join(alias, "missing/child"), "missing/child"},
		{"resolved path", alias, filepath.Join(resolvedRoot, "missing/child"), "missing/child"},
		{"relative path", alias, "missing/child", "missing/child"},
		{"platform alias", resolvedRoot, filepath.Join(root, "missing/child"), "missing/child"},
		{"existing path", alias, "file", "file"},
		{"contained dangling leaf", root, "dangling", "missing/child"},
		{"contained dangling ancestor", root, "dangling/deep", "missing/child/deep"},
		{"dangling chain", root, "through-dangling", "missing/child"},
		{"resolved parent traversal", root, "dir-link/../missing", "missing"},
		{"unverified parent traversal", root, "missing/../other", ""},
		{"not a directory", root, "file/child", ""},
		{"link cycle", root, "cycle/child", ""},
		{"unresolved root", filepath.Join(root, "missing"), "child", ""},
		{"root itself", alias, resolvedRoot, ""},
		{"empty path", root, "", ""},
		{"parent of root", root, "..", ""},
		{"escaping link", root, "escape/child", ""},
		{"dot-dot prefixed entry", root, "..name", "..name"},
		{"dangling parent traversal", root, "dangling/..", ""},
		{"dangling link with trailing separator", root, "dangling/", ""},
		{"dangling target resolves parent traversal physically", root, "physical", "dir/missing"},
		{"dangling chain at the 40-expansion bound", root, "chain40-1", "missing"},
		{"dangling chain past the bound", root, "chain41-1", ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, ok := Relative(test.root, test.path)
			if ok != (test.want != "") || got != test.want {
				t.Fatalf("Relative(%q, %q) = %q, %v; want %q", test.root, test.path, got, ok, test.want)
			}
		})
	}
}
