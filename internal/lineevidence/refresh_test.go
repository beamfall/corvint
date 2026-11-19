// SPDX-License-Identifier: Apache-2.0
package lineevidence

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/linecov"
	"github.com/Beamfall/corvint/internal/testinventory"
)

// LEV-V1-002/003: certify the whole Git state, not only regular-file blobs.
// Exercise each refusal before collection and immediately before publication.
func TestLEVV1003RefreshCleanGitState(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX modes and symlinks")
	}
	for _, kind := range []string{"mode", "staged-mode", "symlink", "staged-symlink", "gitlink", "staged-gitlink", "dirty-submodule", "staged-content", "untracked-link"} {
		for _, during := range []bool{false, true} {
			phase := "before"
			if during {
				phase = "during"
			}
			t.Run(kind+"/"+phase, func(t *testing.T) {
				root := t.TempDir()
				put := func(name, body string) {
					t.Helper()
					if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0644); err != nil {
						t.Fatal(err)
					}
				}
				put("go.mod", "module example\n\ngo 1.27.4\n")
				put("calc.go", "package example\nfunc Calc() {}\n")
				put("calc_test.go", "package example\n")
				put("runner.sh", "#!/bin/sh\nexit 0\n")
				if err := os.Symlink("calc.go", filepath.Join(root, "link")); err != nil {
					t.Fatal(err)
				}
				nested := filepath.Join(root, "nested")
				if err := os.Mkdir(nested, 0755); err != nil {
					t.Fatal(err)
				}
				put("nested/data", "initial\n")
				fixtureGit(t, nested, "init", "-q")
				fixtureGit(t, nested, "add", ".")
				fixtureGit(t, nested, "commit", "-qm", "nested")
				put(".gitmodules", "[submodule \"nested\"]\n path = nested\n url = ./unused\n")
				fixtureGit(t, root, "init", "-q")
				fixtureGit(t, root, "add", ".")
				fixtureGit(t, root, "commit", "-qm", "fixture")
				// Repository preferences must not hide these changes from refresh.
				fixtureGit(t, root, "config", "core.fileMode", "false")
				fixtureGit(t, root, "config", "submodule.nested.ignore", "all")
				git := testinventory.Git{Executable: "git", Root: root}
				ctx := context.Background()
				in, ix := fixture(t)
				store, err := Store(ctx, git)
				if err != nil {
					t.Fatal(err)
				}
				name, err := Write(store, in.Instance, ix)
				if err != nil {
					t.Fatal(err)
				}
				before, err := os.ReadFile(name)
				if err != nil {
					t.Fatal(err)
				}
				mutate := func() {
					switch kind {
					case "mode":
						if err := os.Chmod(filepath.Join(root, "runner.sh"), 0755); err != nil {
							t.Fatal(err)
						}
					case "staged-mode":
						fixtureGit(t, root, "update-index", "--chmod=+x", "runner.sh")
					case "symlink", "staged-symlink":
						if err := os.Remove(filepath.Join(root, "link")); err != nil {
							t.Fatal(err)
						}
						if err := os.Symlink("calc_test.go", filepath.Join(root, "link")); err != nil {
							t.Fatal(err)
						}
						if kind == "staged-symlink" {
							fixtureGit(t, root, "add", "link")
						}
					case "gitlink", "staged-gitlink", "dirty-submodule":
						put("nested/data", "changed\n")
						if kind != "dirty-submodule" {
							fixtureGit(t, nested, "add", ".")
							fixtureGit(t, nested, "commit", "-qm", "changed")
						}
						if kind == "staged-gitlink" {
							fixtureGit(t, root, "add", "nested")
						}
					case "staged-content":
						put("runner.sh", "staged\n")
						fixtureGit(t, root, "add", "runner.sh")
						put("runner.sh", "#!/bin/sh\nexit 0\n") // worktree equals HEAD; index does not
					case "untracked-link":
						if err := os.Symlink("absent", filepath.Join(root, "untracked")); err != nil {
							t.Fatal(err)
						}
					}
				}
				if !during {
					mutate()
				}
				calls := 0
				result, err := Refresh(ctx, Options{Git: git, Runner: "go", Collect: func(context.Context, testinventory.Git, testinventory.Declared, []testinventory.Declared, map[string]string, string) (*linecov.Index, error) {
					calls++
					if during {
						mutate()
					}
					replacement := *ix
					replacement.Runner = "new evidence"
					return &replacement, nil
				}})
				want := "line-evidence: refresh requires a clean tree"
				wantCalls := 0
				if during {
					want = "line-evidence: tree changed during collection"
					wantCalls = 1
				}
				if err == nil || err.Error() != want || len(result) != 0 || calls != wantCalls {
					t.Fatalf("result=%+v err=%v calls=%d", result, err, calls)
				}
				after, err := os.ReadFile(name)
				if err != nil || !bytes.Equal(before, after) {
					t.Fatalf("previous evidence replaced: %v", err)
				}
			})
		}
	}
}

// The cleanliness allowlist is exactly successful empty output. A future or
// malformed status record and an unsuccessful empty result both fail closed.
func TestLEVV1003RefreshUnknownStatus(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX Git stub")
	}
	for _, tc := range []struct {
		name, script string
		clean        bool
	}{
		{"empty", "exit 0\n", true},
		{"unknown-form", "printf 'FUTURE-STATUS\\000opaque\\000'\n", false},
		{"failed-empty", "exit 1\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			executable := filepath.Join(root, "git-stub")
			if err := os.WriteFile(executable, []byte("#!/bin/sh\n"+tc.script), 0700); err != nil {
				t.Fatal(err)
			}
			if got := refreshTreeClean(context.Background(), testinventory.Git{Executable: executable, Root: root}); got != tc.clean {
				t.Fatalf("clean=%t, want %t (%s)", got, tc.clean, strings.TrimSpace(tc.script))
			}
		})
	}
}
