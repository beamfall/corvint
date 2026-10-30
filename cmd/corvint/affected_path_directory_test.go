package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// AFP-V0-231: a tracked directory (with or without a trailing slash) expands to its tracked
// files; an untracked directory, a submodule and non-canonical forms are refused, never dropped.
func TestAFPV0231PathDirectoryExpansion(t *testing.T) {
	root := materializationFixture(t)
	for name, raw := range map[string]string{"dir/a.txt": "a\n", "dir/sub/b.txt": "b\n", "other.txt": "o\n"} {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	materializationGit(t, root, "add", ".")
	materializationGit(t, root, "commit", "-qm", "dirs")
	if err := os.MkdirAll(filepath.Join(root, "untracked"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(root, "dir", "new.txt"), []byte("n\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := parseAffectedInvocation([]string{"--root", root, "affected", "--path", "dir"}); err == nil || !strings.Contains(err.Error(), "dir/new.txt") {
		t.Fatalf("untracked file under a directory must be named, not dropped: %v", err)
	}
	if err := os.Remove(filepath.Join(root, "dir", "new.txt")); err != nil {
		t.Fatal(err)
	}

	want := []string{"dir/a.txt", "dir/sub/b.txt"}
	for _, value := range []string{"dir", "dir/"} {
		invocation, _, err := parseAffectedInvocation([]string{"--root", root, "affected", "--path", value})
		if err != nil || !reflect.DeepEqual(invocation.PathList, want) {
			t.Fatalf("%q: %v %v", value, invocation.PathList, err)
		}
	}
	invocation, _, err := parseAffectedInvocation([]string{"--root", root, "affected", "--path", "dir/sub/", "--path", "other.txt", "--path", "dir/a.txt"})
	if err != nil || !reflect.DeepEqual(invocation.PathList, []string{"dir/a.txt", "dir/sub/b.txt", "other.txt"}) {
		t.Fatalf("mixed: %v %v", invocation.PathList, err)
	}
	if invocation, _, err = parseAffectedInvocation([]string{"--root", root, "affected", "--path", "tracked"}); err != nil || !reflect.DeepEqual(invocation.PathList, []string{"tracked"}) {
		t.Fatalf("file: %v %v", invocation.PathList, err)
	}

	for name, value := range map[string]string{
		"untracked directory": "untracked",
		"untracked slash":     "untracked/",
		"dot prefix":          "./dir",
		"double slash":        "dir//",
		"parent":              "../dir",
		"absolute":            "/dir",
	} {
		if _, _, err := parseAffectedInvocation([]string{"--root", root, "affected", "--path", value}); err == nil {
			t.Fatalf("%s: accepted %q", name, value)
		}
	}
	_, _, err = parseAffectedInvocation([]string{"--root", root, "affected", "--path", "untracked"})
	if err == nil || !strings.Contains(err.Error(), "tracked at HEAD as `dir` or `dir/`") {
		t.Fatalf("untracked refusal must name the accepted forms: %v", err)
	}
}
