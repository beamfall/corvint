// SPDX-License-Identifier: Apache-2.0
package dogfoodflow

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// LCP-V0-040: immutable shape proof precedes any parent-bound continuation.
func TestLocalCompletionSealedBinding(t *testing.T) {
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	writeTree(t, root, map[string]string{"source": "unchanged\n", ".corvint/change.cem.json": "map\n"})
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-qm", "bind")
	bind := testGit(t, root, "rev-parse", "HEAD")
	archive := ".corvint/changes/" + bind + ".cem.json"
	if err := os.MkdirAll(filepath.Dir(filepath.Join(root, archive)), 0755); err != nil {
		t.Fatal(err)
	}
	testGit(t, root, "mv", ".corvint/change.cem.json", archive)
	testGit(t, root, "commit", "-qm", "seal")
	seal := testGit(t, root, "rev-parse", "HEAD")
	positive, err := ResolveSealBinding(context.Background(), root, seal)
	if err != nil || positive.EvidenceTarget != bind || positive.MapPath != archive {
		t.Fatalf("positive: %#v %v", positive, err)
	}
	// Each injected commit starts from the proven parent; no fixture failure is
	// mistaken for a refused seal.
	for _, tc := range []struct {
		name   string
		mutate func()
	}{
		{"extra-source", func() { writeTree(t, root, map[string]string{"source": "changed\n"}) }},
		{"changed-map", func() { writeTree(t, root, map[string]string{archive: "different\n"}) }},
		{"wrong-name", func() { testGit(t, root, "mv", archive, ".corvint/changes/wrong.cem.json") }},
		{"symlink-map", func() {
			if err := os.Remove(filepath.Join(root, archive)); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink("../../source", filepath.Join(root, archive)); err != nil {
				t.Fatal(err)
			}
		}},
		{"mode-change", func() {
			if err := os.Chmod(filepath.Join(root, archive), 0755); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			testGit(t, root, "checkout", "--detach", "-q", seal)
			tc.mutate()
			testGit(t, root, "add", ".")
			// Amend in this disposable fixture so the candidate still has bind as parent.
			testGit(t, root, "commit", "--amend", "-qm", tc.name)
			candidate := testGit(t, root, "rev-parse", "HEAD")
			if tc.name == "mode-change" && !strings.HasPrefix(testGit(t, root, "ls-tree", candidate, "--", archive), "100755 ") {
				t.Fatal("mode injection not reached")
			}
			if candidate == seal {
				t.Fatal("injection did not change commit")
			}
			if _, err := ResolveSealBinding(context.Background(), root, candidate); err == nil {
				t.Fatal("invalid seal accepted")
			}
		})
	}
	tree := testGit(t, root, "rev-parse", seal+"^{tree}")
	merge := testGit(t, root, "commit-tree", tree, "-p", bind, "-p", seal, "-m", "forged merge")
	if _, err := ResolveSealBinding(context.Background(), root, merge); err == nil {
		t.Fatal("multi-parent seal accepted")
	}
	testGit(t, root, "checkout", "--detach", "-q", seal)
	testGit(t, root, "commit", "--allow-empty", "-qm", "descendant")
	if _, err := ResolveSealBinding(context.Background(), root, testGit(t, root, "rev-parse", "HEAD")); err == nil {
		t.Fatal("descendant accepted")
	}
}
