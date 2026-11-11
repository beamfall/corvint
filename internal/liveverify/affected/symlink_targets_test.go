// SPDX-License-Identifier: Apache-2.0

package affected

import (
	"context"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"testing"
)

// AFP-V0-9965: TreeSymlinks lists the links of the measured commit with
// their targets read from the tree, and each dirty path that is a link at
// either side; symlinkTargets makes each dirty path that is a link, lies
// below one, or lies at or below a resolved target UNKNOWN in a graph with
// the shell plugin.
func TestSymlinks_AFPV09965(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("creating a symbolic link needs a privilege Windows test hosts do not grant")
	}
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git not installed")
	}
	root := t.TempDir()
	recordGit(t, root, "init", "-q")
	for _, name := range []string{"real/a.sh", "real/sub/b.sh", "other/c.sh", "plain/d.sh", "flip/e.sh"} {
		recordWrite(t, root, name, "true\n")
	}
	for link, target := range map[string]string{
		"link":     "real",                        // a tracked link
		"chain":    "link/sub",                    // through another link
		"climb":    "link/../other",               // `..` after a link climbs from the target
		"escape":   "../outside",                  // leaves the repository
		"absolute": filepath.Join(root, "plain"),  // absolute, inside the tree
		"foreign":  "/nonexistent-corvint-v15368", // absolute, outside
		"dangle":   "missing",                     // dangling
		"retarget": "real",                        // retargeted in the worktree
		"gone":     "real",                        // deleted in the worktree
		"unread":   "flip",                        // a regular file in the worktree, not passed dirty
		"loop/a":   "b",                           // a cycle
		"loop/b":   "a",
	} {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, link)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, filepath.Join(root, filepath.FromSlash(link))); err != nil {
			t.Fatal(err)
		}
	}
	recordGit(t, root, "add", "-A")
	recordGit(t, root, "commit", "-q", "-m", "base")
	base := recordGit(t, root, "rev-parse", "HEAD")
	for _, name := range []string{"retarget", "gone", "unread"} {
		if err := os.Remove(filepath.Join(root, name)); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink("other", filepath.Join(root, "retarget")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("plain", filepath.Join(root, "fresh")); err != nil { // untracked
		t.Fatal(err)
	}
	recordWrite(t, root, "unread", "now a file\n")
	dirty := []string{"fresh", "gone", "retarget"}
	links := TreeSymlinks(context.Background(), git, root, root, base, dirty)
	wantTargets := map[string]string{
		"link": "real", "chain": "link/sub", "climb": "link/../other", "escape": "../outside",
		"absolute": "plain", "foreign": "", "dangle": "missing", "unread": ".",
		"loop/a": "loop/b", "loop/b": "loop/a", "retarget": "other", "fresh": "plain",
	}
	if !links.Listed || !maps.Equal(links.Targets, wantTargets) {
		t.Errorf("targets %v listed %v, want %v", links.Targets, links.Listed, wantTargets)
	}
	if want := []string{"fresh", "gone", "retarget"}; !slices.Equal(links.Changed, want) {
		t.Errorf("changed %v, want %v", links.Changed, want)
	}
	for start, want := range map[string]string{
		"link": "real", "chain": "real/sub", "climb": "other", "absolute": "plain",
		"dangle": "missing", "loop/a": ".", "link/sub/x": "real/sub/x",
	} {
		if got, inside := resolveSymlink(start, links.Targets); !inside || got != want {
			t.Errorf("resolve %s: %q %v, want %q", start, got, inside, want)
		}
	}
	for _, start := range []string{"escape", "foreign", "../x"} {
		if got, inside := resolveSymlink(start, links.Targets); inside {
			t.Errorf("resolve %s: %q inside, want outside", start, got)
		}
	}
	shell := (&Graph{languages: []string{"shell"}}).WithSymlinks(links)
	// "unread" resolves to the root, so every path is UNKNOWN; without it,
	// only the paths a link reaches are.
	if got := shell.symlinkTargets([]string{"elsewhere.txt"}); len(got) != 1 {
		t.Errorf("an unreadable target does not cover every path: %v", got)
	}
	narrow := links
	narrow.Targets = maps.Clone(links.Targets)
	delete(narrow.Targets, "unread")
	delete(narrow.Targets, "loop/a")
	delete(narrow.Targets, "loop/b")
	delete(narrow.Targets, "climb")
	shell = (&Graph{languages: []string{"shell"}}).WithSymlinks(narrow)
	for path, want := range map[string]bool{
		"real/a.sh":     true,  // below link's target
		"real/sub/b.sh": true,  // below chain's target
		"plain/d.sh":    true,  // below an absolute target inside the tree
		"missing/x.sh":  true,  // below a dangling target
		"other/c.sh":    true,  // below retarget's new target
		"gone":          true,  // a deleted link
		"fresh":         true,  // an untracked link
		"link/a.sh":     true,  // a path through a link
		"flip/e.sh":     false, // reached by no link
		"realm/x.sh":    false, // a sibling of a target, not below it
	} {
		got := shell.symlinkTargets([]string{path})
		if covered := len(got) == 1 && got[0] == (Unknown{Reason: UnknownLanguageFrontier, Detail: FrontierShellSymlinkTarget + ":" + path}); covered != want {
			t.Errorf("%s: %v, want covered %v", path, got, want)
		}
	}
	// Not listed: no commit, an unreadable commit, no Git.
	for name, unlisted := range map[string]Symlinks{
		"no commit":         TreeSymlinks(context.Background(), git, root, root, "", dirty),
		"unreadable commit": TreeSymlinks(context.Background(), git, root, root, "0000000000000000000000000000000000000000", dirty),
		"no git executable": TreeSymlinks(context.Background(), "", root, root, base, dirty),
	} {
		if unlisted.Listed {
			t.Errorf("%s: listed", name)
		}
		got := (&Graph{languages: []string{"shell"}}).WithSymlinks(unlisted).symlinkTargets([]string{"flip/e.sh"})
		if len(got) != 1 || got[0].Detail != FrontierShellSymlinkInventoryUnknown {
			t.Errorf("%s: %v", name, got)
		}
	}
	// No list, no shell plugin, or no dirty path: no unknown.
	for name, got := range map[string][]Unknown{
		"no list":         (&Graph{languages: []string{"shell"}}).symlinkTargets([]string{"real/a.sh"}),
		"no shell plugin": (&Graph{languages: []string{"go"}}).WithSymlinks(narrow).symlinkTargets([]string{"real/a.sh"}),
		"no dirty path":   (&Graph{languages: []string{"shell"}}).WithSymlinks(Symlinks{}).symlinkTargets(nil),
	} {
		if len(got) != 0 {
			t.Errorf("%s: %v", name, got)
		}
	}
}
