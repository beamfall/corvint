// SPDX-License-Identifier: Apache-2.0
package affected

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// AFP-V0-9960: only a regular file read at both sides is modified; every
// unreadable side, and a path present at neither side, is unknown.
func TestClassifyChange_AFPV09960(t *testing.T) {
	regular := ChangeEntry{Known: true, Present: true, Regular: true}
	other := ChangeEntry{Known: true, Present: true}
	absent := ChangeEntry{Known: true}
	unread := ChangeEntry{}
	for _, row := range []struct {
		name       string
		base, head ChangeEntry
		want       ChangeStatus
	}{
		{"added", absent, regular, ChangeAdded},
		{"added symlink", absent, other, ChangeAdded},
		{"deleted", regular, absent, ChangeDeleted},
		{"modified", regular, regular, ChangeModified},
		{"file to directory", regular, other, ChangeTypeChanged},
		{"symlink to file", other, regular, ChangeTypeChanged},
		{"changed symlink", other, other, ChangeTypeChanged},
		{"absent both", absent, absent, ChangeUnknown},
		{"base unreadable", unread, regular, ChangeUnknown},
		{"head unreadable", regular, unread, ChangeUnknown},
	} {
		if got := ClassifyChange(row.base, row.head); got != row.want {
			t.Errorf("%s: %s, want %s", row.name, got, row.want)
		}
	}
	for _, status := range []ChangeStatus{ChangeAdded, ChangeDeleted, ChangeTypeChanged, ChangeUnknown, "", "renamed"} {
		if !ListingCanChange(status) {
			t.Errorf("ListingCanChange(%q) = false; only modified cannot change a listing", status)
		}
	}
	if ListingCanChange(ChangeModified) {
		t.Error("ListingCanChange(modified) = true")
	}
}

// AFP-V0-9960: ChangeStatuses reads the base from Git and the head from the
// tree: added, deleted, renamed (deleted plus added), type-changed (file to
// directory, a changed symlink), modified and untracked paths, and no base.
func TestChangeStatuses_AFPV09960(t *testing.T) {
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git not installed")
	}
	root := t.TempDir()
	recordGit(t, root, "init", "-q")
	for _, name := range []string{"mod.txt", "gone.txt", "old.txt", "kind", "target-a", "target-b"} {
		recordWrite(t, root, name, name+"\n")
	}
	links := os.Symlink("target-a", filepath.Join(root, "link")) == nil
	recordGit(t, root, "add", "-A")
	recordGit(t, root, "commit", "-q", "-m", "base")
	base := recordGit(t, root, "rev-parse", "HEAD")
	recordWrite(t, root, "mod.txt", "edited\n")
	recordWrite(t, root, "new.txt", "old.txt\n")
	for _, name := range []string{"gone.txt", "old.txt", "kind"} {
		if err := os.Remove(filepath.Join(root, name)); err != nil {
			t.Fatal(err)
		}
	}
	recordWrite(t, root, "kind/inner", "x\n")
	recordWrite(t, root, "committed.txt", "x\n")
	recordGit(t, root, "add", "-A")
	recordGit(t, root, "commit", "-q", "-m", "head")
	recordWrite(t, root, "untracked.txt", "x\n")
	if links {
		if err := os.Remove(filepath.Join(root, "link")); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink("target-b", filepath.Join(root, "link")); err != nil {
			t.Fatal(err)
		}
	}
	paths := []string{"committed.txt", "gone.txt", "kind", "kind/inner", "link", "mod.txt", "never.txt", "new.txt", "old.txt", "untracked.txt"}
	got := ChangeStatuses(context.Background(), git, root, root, base, paths)
	want := map[string]ChangeStatus{
		"committed.txt": ChangeAdded,
		"gone.txt":      ChangeDeleted,
		"kind":          ChangeTypeChanged,
		"kind/inner":    ChangeAdded,
		"mod.txt":       ChangeModified,
		"new.txt":       ChangeAdded,
		"old.txt":       ChangeDeleted,
		"untracked.txt": ChangeAdded,
	}
	if links {
		want["link"] = ChangeTypeChanged
	}
	for _, relative := range paths {
		graph := (&Graph{}).WithChangeStatus(got)
		expected, ok := want[relative]
		if !ok {
			expected = ChangeUnknown
		}
		if status := graph.ChangeStatusOf(relative); status != expected {
			t.Errorf("%s: %s, want %s", relative, status, expected)
		}
	}
	if _, listed := got["never.txt"]; listed {
		t.Errorf("never.txt, absent at both sides, has a status: %v", got)
	}
	for name, status := range map[string]map[string]ChangeStatus{
		"no base":         ChangeStatuses(context.Background(), git, root, root, "", paths),
		"unreadable base": ChangeStatuses(context.Background(), git, root, root, "0000000000000000000000000000000000000000", paths),
	} {
		if status != nil {
			t.Errorf("%s: %v, want nil (every path unknown)", name, status)
		}
	}
	// A status value outside the five, and an absent map, read as unknown.
	graph := (&Graph{}).WithChangeStatus(map[string]ChangeStatus{"mod.txt": "renamed"})
	if status := graph.ChangeStatusOf("mod.txt"); status != ChangeUnknown {
		t.Errorf("an unrecognised status reads %s, want unknown", status)
	}
	if status := (&Graph{}).ChangeStatusOf("mod.txt"); status != ChangeUnknown {
		t.Errorf("a graph without status reads %s, want unknown", status)
	}
}
