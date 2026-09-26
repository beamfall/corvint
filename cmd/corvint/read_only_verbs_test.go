package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/wire"
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

// ledgerIgnoringRepository is the harness fixture with .corvint/ gitignored,
// which is the one condition under which harness event may append the
// self-observation ledger row.
func ledgerIgnoringRepository(t *testing.T) string {
	t.Helper()
	root := ledgerRepository(t)
	cemWrite(t, root, ".gitignore", ".corvint/\n.context-corvint/\n")
	cemGit(t, root, "add", ".gitignore")
	cemGit(t, root, "commit", "-qm", "ignore .corvint")
	return root
}

func runHarnessEventInProcess(t *testing.T, root, event, input string) int {
	t.Helper()
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	code := runContext(context.Background(), cliArguments(root, event), strings.NewReader(input), stdout, stderr)
	if code != 0 {
		t.Logf("harness event %s exited %d: %s", event, code, stderr)
	}
	return code
}

// TestReadOnlyVerbsWriteNothing is the filesystem tripwire for the verbs the
// root help lists as reading without mutating repository or trace state
// (init, adopt, query, impact, docs, harness, lrf; GPK-V0-007, AGENTS.md
// invariant 4). Each verb runs over a snapshot of the whole fixture tree,
// .git and .corvint included, and may change nothing except the documented
// ledgers under their documented conditions: harness event session-start
// appends one self-observation row when .corvint/ is gitignored, and the
// unplanned-read marker alone never makes a read verb record anything.
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
			name: "query compiles the authority-start receipt",
			setup: func(t *testing.T) (string, func() int) {
				root := queryCLIRepository(t)
				return root, func() int {
					code, _, _ := runCLI(t, "--root", root, "query", "--task", authorityStartPrompt, "--limit", "1")
					return code
				}
			},
		},
		{
			name: "query records nothing while the unplanned-read marker exists",
			setup: func(t *testing.T) (string, func() int) {
				root := queryCLIRepository(t)
				cemWrite(t, root, ".corvint/unplanned-reads.enabled", "")
				return root, func() int {
					code, _, _ := runCLI(t, "--root", root, "query", "--task", authorityStartPrompt, "--limit", "1")
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
		{
			name: "docs drafts the source-derived page",
			setup: func(t *testing.T) (string, func() int) {
				root := docsRepository(t)
				return root, func() int {
					code, _, _ := runDocsFixture(t, root, "draft", "")
					return code
				}
			},
		},
		{
			name: "harness event session-start appends only the self-observation row",
			setup: func(t *testing.T) (string, func() int) {
				root := ledgerIgnoringRepository(t)
				return root, func() int { return runHarnessEventInProcess(t, root, "session-start", `{"startSource":"startup"}`) }
			},
			allowed: []string{".corvint", ".corvint/self-observations.jsonl"},
		},
		{
			name: "harness event stop writes nothing",
			setup: func(t *testing.T) (string, func() int) {
				root := ledgerIgnoringRepository(t)
				return root, func() int { return runHarnessEventInProcess(t, root, "stop", `{}`) }
			},
		},
		{
			name: "lrf verifies the canonical-derived CEM/02 fixture",
			setup: func(t *testing.T) (string, func() int) {
				fixture := newLRFFixture(t, wire.Spec02)
				return fixture.root, func() int {
					code, _, _ := runCLI(t, "--root", fixture.root, "lrf", "--cem", fixture.mapPath,
						"--expected-base", fixture.base, "--target", fixture.target)
					return code
				}
			},
		},
		{
			name: "cem export writes only the bundle outside the worktree",
			setup: func(t *testing.T) (string, func() int) {
				fixture := newLRFFixture(t, wire.Spec02)
				bind := commitExportMap(t, fixture)
				output := filepath.Join(t.TempDir(), "bundle")
				return fixture.root, func() int {
					code, _, _ := runCLI(t, "--root", fixture.root, "cem", "export", "--map", fixture.mapPath,
						"--expected-base", fixture.base, "--target", bind, "--output", output)
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
