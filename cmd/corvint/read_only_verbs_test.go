package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// treeSnapshot maps every path below root, including .git and .corvint, to
// its kind, mode, content digest and modification time, so a verb that
// rewrites a file with identical bytes still trips the comparison.
func treeSnapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	snapshot := make(map[string]string)
	err := filepath.WalkDir(root, func(file string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if file == root {
			return nil
		}
		relative, err := filepath.Rel(root, file)
		if err != nil {
			return err
		}
		info, err := os.Lstat(file)
		if err != nil {
			return err
		}
		switch {
		case info.IsDir():
			snapshot[filepath.ToSlash(relative)] = fmt.Sprintf("dir %v", info.Mode())
		case info.Mode()&fs.ModeSymlink != 0:
			target, err := os.Readlink(file)
			if err != nil {
				return err
			}
			snapshot[filepath.ToSlash(relative)] = "link " + target
		default:
			data, err := os.ReadFile(file)
			if err != nil {
				return err
			}
			digest := sha256.Sum256(data)
			snapshot[filepath.ToSlash(relative)] = fmt.Sprintf("file %v %s %d", info.Mode(), hex.EncodeToString(digest[:]), info.ModTime().UnixNano())
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}

// snapshotDelta lists, sorted, every path whose snapshot entry was added,
// removed or changed.
func snapshotDelta(before, after map[string]string) []string {
	delta := make([]string, 0)
	for path, entry := range before {
		if after[path] != entry {
			delta = append(delta, path)
		}
	}
	for path := range after {
		if _, ok := before[path]; !ok {
			delta = append(delta, path)
		}
	}
	slices.Sort(delta)
	return delta
}

// TestReadOnlyVerbsWriteNothing is the filesystem tripwire for the verbs the
// root help lists as reading without mutating repository or trace state
// (init, adopt, impact, docs, lrf; GPK-V0-007, AGENTS.md invariant 4). Each
// verb runs over a snapshot of the whole fixture tree, .git and .corvint
// included, and may change nothing except the documented ledgers under their
// documented conditions.
func TestReadOnlyVerbsWriteNothing(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		setup    func(t *testing.T) (root string, invoke func() int)
		wantExit int
		allowed  []string
	}{
		{
			name: "init compiles the first inventory",
			setup: func(t *testing.T) (string, func() int) {
				root := newActivationFixture(t, "sha1")
				return root, func() int {
					code, _, _ := runCLI(t, activationArguments(root, "init", true)...)
					return code
				}
			},
		},
		{
			name: "adopt compiles the recoverable inventory",
			setup: func(t *testing.T) (string, func() int) {
				root := newActivationFixture(t, "sha1")
				return root, func() int {
					code, _, _ := runCLI(t, activationArguments(root, "adopt", false)...)
					return code
				}
			},
		},
		{
			name: "impact compiles path impact",
			setup: func(t *testing.T) (string, func() int) {
				root := impactCLIRepository(t)
				return root, func() int {
					code, _, _ := runCLI(t, "--root", root, "impact", "pkg/main.go", "--limit", "10")
					return code
				}
			},
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			root, invoke := testCase.setup(t)
			before := treeSnapshot(t, root)
			if exit := invoke(); exit != testCase.wantExit {
				t.Fatalf("exit = %d, want %d", exit, testCase.wantExit)
			}
			allowed := testCase.allowed
			if allowed == nil {
				allowed = []string{}
			}
			if delta := snapshotDelta(before, treeSnapshot(t, root)); !slices.Equal(delta, allowed) {
				t.Fatalf("verb changed %v, want exactly %v", delta, allowed)
			}
			if len(allowed) == 0 {
				return
			}
			ledger, err := os.ReadFile(filepath.Join(root, selfObservationsRelativePath))
			if err != nil || bytes.Count(ledger, []byte("\n")) != 1 {
				t.Fatalf("ledger = %q (%v), want exactly one appended row", ledger, err)
			}
		})
	}
}
