package worksource

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/workqueue"
)

func fixture(t *testing.T) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	write(t, root, "a.txt", []byte("alpha\n"), 0644)
	write(t, root, "directory/b.txt", []byte("beta\n"), 0644)
	runGit(t, root, "init", "-q")
	runGit(t, root, "add", ".")
	commitFixture(t, root)
	return root
}
func write(t *testing.T, root, path string, raw []byte, mode os.FileMode) {
	t.Helper()
	destination := filepath.Join(root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(destination), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destination, raw, mode); err != nil {
		t.Fatal(err)
	}
}
func runGit(t *testing.T, root string, args ...string) []byte {
	t.Helper()
	source, err := newSource()
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	source.Root = root
	raw, err := source.Git(context.Background(), 32<<20, args...)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
func commitFixture(t *testing.T, root string) {
	runGit(t, root, "-c", "user.name=Source test", "-c", "user.email=source@example.invalid", "-c", "commit.gpgsign=false", "commit", "-qm", "fixture")
}

// WQO-V0-005 / VPO-V0-007,008,010: unsupported sources fail before callers
// receive a source with which to launch their adapter.
func TestWorkSourceRejectsBeforeAdapterLaunch(t *testing.T) {
	t.Run("WQO-V0-005", func(t *testing.T) {
		cases := map[string]func(*testing.T, string){
			"dirty-tracked": func(t *testing.T, r string) { write(t, r, "a.txt", []byte("dirty\n"), 0644) },
			"untracked":     func(t *testing.T, r string) { write(t, r, "unknown", []byte("x"), 0644) },
			"index-drift": func(t *testing.T, r string) {
				write(t, r, "a.txt", []byte("dirty\n"), 0644)
				runGit(t, r, "add", "a.txt")
			},
			"skip-worktree": func(t *testing.T, r string) {
				runGit(t, r, "update-index", "--skip-worktree", "a.txt")
				write(t, r, "a.txt", []byte("dirty\n"), 0644)
			},
			"assume-unchanged": func(t *testing.T, r string) {
				runGit(t, r, "update-index", "--assume-unchanged", "a.txt")
				write(t, r, "a.txt", []byte("dirty\n"), 0644)
			},
			"sparse":      func(t *testing.T, r string) { runGit(t, r, "config", "core.sparseCheckout", "true") },
			"split-index": func(t *testing.T, r string) { runGit(t, r, "update-index", "--split-index") },
			"replace": func(t *testing.T, r string) {
				head := strings.TrimSpace(string(runGit(t, r, "rev-parse", "HEAD")))
				runGit(t, r, "update-ref", "refs/replace/"+head, head)
			},
			"grafts":     func(t *testing.T, r string) { write(t, r, ".git/info/grafts", []byte(""), 0644) },
			"alternates": func(t *testing.T, r string) { write(t, r, ".git/objects/info/alternates", []byte(""), 0644) },
			"intent-to-add": func(t *testing.T, r string) {
				write(t, r, "new", []byte("new"), 0644)
				runGit(t, r, "add", "-N", "new")
			},
			"mode-drift": func(t *testing.T, r string) {
				runGit(t, r, "config", "core.filemode", "false")
				if err := os.Chmod(filepath.Join(r, "a.txt"), 0755); err != nil {
					t.Fatal(err)
				}
			},
			"symlink-escape": func(t *testing.T, r string) {
				if err := os.Symlink("../../outside", filepath.Join(r, "link")); err != nil {
					t.Fatal(err)
				}
				runGit(t, r, "add", "link")
				commitFixture(t, r)
			},
			"symlink-dangling": func(t *testing.T, r string) {
				if err := os.Symlink("missing", filepath.Join(r, "link")); err != nil {
					t.Fatal(err)
				}
				runGit(t, r, "add", "link")
				commitFixture(t, r)
			},
			"symlink-cycle": func(t *testing.T, r string) {
				if err := os.Symlink("link", filepath.Join(r, "link")); err != nil {
					t.Fatal(err)
				}
				runGit(t, r, "add", "link")
				commitFixture(t, r)
			},
			"submodule": func(t *testing.T, r string) {
				head := strings.TrimSpace(string(runGit(t, r, "rev-parse", "HEAD")))
				runGit(t, r, "update-index", "--add", "--cacheinfo", "160000,"+head+",module")
				commitFixture(t, r)
			},
			"included-config": func(t *testing.T, r string) { runGit(t, r, "config", "include.path", "/dev/null") },
		}
		for name, mutate := range cases {
			t.Run(name, func(t *testing.T) {
				root := fixture(t)
				mutate(t, root)
				launched := false
				source, err := Acquire(context.Background(), root)
				if err == nil {
					defer source.Close()
					launched = true
				}
				if err == nil || launched {
					t.Fatal("unqualified source admitted before adapter launch")
				}
			})
		}
	})
}

// VPO-V0-003,008: independently fixed canonical preimage, raw byte order and
// blob IDs; an index stat-cache hit cannot hide same-size restored-mtime edits.
func TestWorkSourceMaterializationGolden(t *testing.T) {
	root := fixture(t)
	source, err := Acquire(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	a := runGit(t, root, "rev-parse", "HEAD:a.txt")
	b := runGit(t, root, "rev-parse", "HEAD:directory/b.txt")
	aSHA, bSHA := sha256.Sum256([]byte("alpha\n")), sha256.Sum256([]byte("beta\n"))
	body := fmt.Sprintf(`[{"blobOid":"%s","mode":"100644","path":"a.txt","rawSha256":"%x"},{"blobOid":"%s","mode":"100644","path":"directory/b.txt","rawSha256":"%x"}]`, strings.TrimSpace(string(a)), aSHA, strings.TrimSpace(string(b)), bSHA)
	preimage := append([]byte("verification-materialization\x00verification-materialization/0\x00"), []byte(body)...)
	want := sha256.Sum256(preimage)
	if source.Identity.MaterializationSHA256 != hex.EncodeToString(want[:]) {
		t.Fatalf("wrong canonical commitment: %s; preimage %q", source.Identity.MaterializationSHA256, preimage)
	}
	info, err := os.Stat(filepath.Join(root, "a.txt"))
	if err != nil {
		t.Fatal(err)
	}
	write(t, root, "a.txt", []byte("omega\n"), 0644)
	if err := os.Chtimes(filepath.Join(root, "a.txt"), info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	if err := source.VerifyMaterialization(context.Background(), root); err == nil {
		t.Fatal("same-size restored-mtime edit was accepted")
	}
}

// VPO-V0-010: PATH/HOME/config poisoning cannot select another Git binary or
// repository; explicit alternate-index/rebinding requests are refused.
func TestWorkSourcePinsGitAndEnvironment(t *testing.T) {
	root := fixture(t)
	poison := t.TempDir()
	write(t, poison, "git", []byte("#!/bin/sh\nexit 91\n"), 0755)
	t.Setenv("PATH", poison)
	t.Setenv("HOME", poison)
	t.Setenv("TMPDIR", poison)
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "core.worktree")
	t.Setenv("GIT_CONFIG_VALUE_0", poison)
	source, err := Acquire(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	if !filepath.IsAbs(source.GitPath) || strings.HasPrefix(source.GitPath, poison) {
		t.Fatal("ambient Git selected")
	}
	for _, value := range source.GitEnvironment {
		if strings.Contains(value, poison) {
			t.Fatal("ambient environment retained")
		}
	}
	for _, name := range []string{"GIT_INDEX_FILE", "GIT_DIR", "GIT_WORK_TREE", "GIT_OBJECT_DIRECTORY"} {
		t.Run(name, func(t *testing.T) {
			t.Setenv(name, "/dev/null")
			if source, err := Acquire(context.Background(), root); err == nil {
				source.Close()
				t.Fatal("ambient rebinding accepted")
			}
		})
	}
}

// VPO-V0-010: Apple's /usr/bin/git shim is replaced by the xcode-select
// developer Git; any other Git, or an unusable link, is left unchanged.
func TestWorkSourceBypassesAppleGitShim(t *testing.T) {
	developer := t.TempDir()
	write(t, developer, "usr/bin/git", []byte("#!/bin/sh\n"), 0755)
	resolved, err := filepath.EvalSymlinks(filepath.Join(developer, "usr/bin/git"))
	if err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "xcode_select_link")
	if err := os.Symlink(developer, link); err != nil {
		t.Fatal(err)
	}
	dangling := filepath.Join(t.TempDir(), "xcode_select_link")
	if err := os.Symlink(filepath.Join(developer, "missing"), dangling); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ executable, link, want string }{
		{"/usr/bin/git", link, resolved},
		{"/opt/homebrew/Cellar/git/bin/git", link, "/opt/homebrew/Cellar/git/bin/git"},
		{"/usr/bin/git", dangling, "/usr/bin/git"},
		{"/usr/bin/git", filepath.Join(developer, "absent"), "/usr/bin/git"},
	} {
		if got := bypassAppleGitShim(tc.executable, tc.link); got != tc.want {
			t.Fatalf("bypassAppleGitShim(%q, %q) = %q, want %q", tc.executable, tc.link, got, tc.want)
		}
	}
}

// VPO-V0-008: contained tracked symlinks and exact tree/blob/commit objects have
// the same identity when exported into standalone private metadata.
func TestWorkSourceContainedLinksAndPrivateObjects(t *testing.T) {
	root := fixture(t)
	if err := os.Symlink("../a.txt", filepath.Join(root, "directory/link")); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", "directory/link")
	commitFixture(t, root)
	source, err := Acquire(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	destination := t.TempDir()
	runGit(t, destination, "init", "-q")
	if err := source.ExportObjects(context.Background(), filepath.Join(destination, ".git")); err != nil {
		t.Fatal(err)
	}
	raw := runGit(t, destination, "cat-file", "commit", source.Identity.Commit)
	kind, pinned, err := source.Object(context.Background(), source.Identity.Commit)
	if err != nil || kind != "commit" || !bytes.Equal(raw, pinned) {
		t.Fatal("private commit parity failed")
	}
	if err := source.ExportObjects(context.Background(), source.GitDir); err == nil {
		t.Fatal("caller Git export accepted")
	}
	home := source.temporary
	if err := source.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(home); !os.IsNotExist(err) {
		t.Fatal("source temporary storage retained")
	}
}

// VPO-V0-007,008,010: malformed output cannot be mistaken for an empty/partial tree.
func TestWorkSourceRejectsIncompleteTree(t *testing.T) {
	for _, raw := range []string{"100644 blob " + strings.Repeat("a", 40) + "\ta", "160000 commit " + strings.Repeat("a", 40) + "\ta\x00", "100644 blob " + strings.Repeat("a", 40) + "\t../a\x00", "100644 blob " + strings.Repeat("a", 40) + "\ta\xff\x00"} {
		if _, err := parseTree([]byte(raw), "sha1"); err == nil {
			t.Fatalf("accepted malformed tree %q", raw)
		}
	}
}

// VPO-V0-010: all acquisition output is bounded; cancellation closes pipes even
// when a normally exited leader leaves a descendant holding them.
func TestWorkSourceGitBoundsAndPipeOwnership(t *testing.T) {
	oldWaitDelay := gitCommandWaitDelay
	gitCommandWaitDelay = 100 * time.Millisecond
	t.Cleanup(func() { gitCommandWaitDelay = oldWaitDelay })
	source, err := newSource()
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	source.Root = t.TempDir()
	source.GitPath = filepath.Join(source.Root, "git-fixture")
	// Children stay in the owned group; the deferred group kill is the cleanup.
	write(t, source.Root, "git-fixture", []byte("#!/bin/sh\nsleep 30 &\nprintf x\n"), 0755)
	started := time.Now()
	if _, err := source.Git(context.Background(), 100, "ignored"); err == nil {
		t.Fatal("pipe-holder was accepted as complete")
	}
	if time.Since(started) > 3*time.Second {
		t.Fatal("pipe drain exceeded bound")
	}
	write(t, source.Root, "git-fixture", []byte("#!/bin/sh\nprintf overflow\n"), 0755)
	if _, err := source.Git(context.Background(), 2, "ignored"); err == nil {
		t.Fatal("overflow accepted")
	}
	write(t, source.Root, "git-fixture", []byte("#!/bin/sh\nprintf warning >&2\n"), 0755)
	if _, err := source.Git(context.Background(), 100, "ignored"); err == nil {
		t.Fatal("stderr accepted")
	}
}

// WQO-V0-005/015/032: source Git command and pipe-drain bounds are finite hang
// detectors. Slow commands expose the detector cause instead of source facts.
func TestWorkSourceGitHangDetectors(t *testing.T) {
	if gitCommandTimeout != 10*time.Minute || gitCommandWaitDelay != time.Minute {
		t.Fatalf("source Git bounds are not the specified hang detectors: command=%s wait=%s", gitCommandTimeout, gitCommandWaitDelay)
	}
	oldTimeout, oldWaitDelay := gitCommandTimeout, gitCommandWaitDelay
	gitCommandWaitDelay = 100 * time.Millisecond
	t.Cleanup(func() { gitCommandTimeout, gitCommandWaitDelay = oldTimeout, oldWaitDelay })

	source, err := newSource()
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	source.Root = t.TempDir()
	source.GitPath = filepath.Join(source.Root, "git-fixture")
	for _, tc := range []struct {
		name, script string
		timeout      time.Duration
		cause        error
	}{
		{"command", "#!/bin/sh\nsleep 30\n", 100 * time.Millisecond, context.DeadlineExceeded},
		// The command bound sits well above WaitDelay so a descheduled shell
		// cannot reach the deadline first; only the orphaned pipe holder can
		// produce the drain error.
		{"pipe-drain", "#!/bin/sh\nsleep 30 &\nprintf x\n", 2 * time.Second, exec.ErrWaitDelay},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gitCommandTimeout = tc.timeout
			write(t, source.Root, "git-fixture", []byte(tc.script), 0755)
			started := time.Now()
			_, err := source.Git(context.Background(), 100, "ignored")
			if !errors.Is(err, tc.cause) {
				t.Fatalf("slow Git error = %v, want %v", err, tc.cause)
			}
			if time.Since(started) > 3*time.Second {
				t.Fatal("injected slow Git exceeded test hang detector")
			}
		})
	}
}

// WQO-V0-005 / VPO-V0-010: acquiring source cannot refresh an index, create
// caller object/cache files, or mutate any other caller Git metadata byte.
func TestWorkSourceNoCallerGitWrites(t *testing.T) {
	root := fixture(t)
	before := gitManifest(t, filepath.Join(root, ".git"))
	source, err := Acquire(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	source.Close()
	after := gitManifest(t, filepath.Join(root, ".git"))
	if !bytes.Equal(before, after) {
		t.Fatal("source acquisition changed caller Git metadata")
	}
}
func gitManifest(t *testing.T, root string) []byte {
	t.Helper()
	var manifest bytes.Buffer
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		fmt.Fprintf(&manifest, "%s %v\n", relative, info.Mode())
		if info.Mode().IsRegular() {
			raw, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			sum := sha256.Sum256(raw)
			fmt.Fprintf(&manifest, "%x\n", sum)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return manifest.Bytes()
}

// VPO-V0-007,008,010: closing facts catch HEAD, index flags, and link swaps
// after the initially verified bytes; no closing failure yields a launchable Source.
func TestWorkSourceRejectsAcquisitionDrift(t *testing.T) {
	for _, kind := range []string{"head", "index", "symlink"} {
		t.Run(kind, func(t *testing.T) {
			root := fixture(t)
			if err := os.Symlink("../a.txt", filepath.Join(root, "directory/link")); err != nil {
				t.Fatal(err)
			}
			runGit(t, root, "add", "directory/link")
			commitFixture(t, root)
			initial := strings.TrimSpace(string(runGit(t, root, "rev-parse", "HEAD")))
			write(t, root, "a.txt", []byte("other\n"), 0644)
			runGit(t, root, "add", "a.txt")
			commitFixture(t, root)
			next := strings.TrimSpace(string(runGit(t, root, "rev-parse", "HEAD")))
			runGit(t, root, "reset", "--hard", initial)
			mutation := map[string]string{
				"head":    "/usr/bin/git update-ref HEAD " + next,
				"index":   "/usr/bin/git update-index --assume-unchanged a.txt",
				"symlink": "rm directory/link; ln -s b.txt directory/link",
			}[kind]
			script := []byte("#!/bin/sh\ncase \"$*\" in\n *'cat-file commit '*) /usr/bin/git \"$@\"; " + mutation + " ;;\n *) exec /usr/bin/git \"$@\" ;;\nesac\n")
			directory := t.TempDir()
			write(t, directory, "git", script, 0755)
			source, err := newSource()
			if err != nil {
				t.Fatal(err)
			}
			defer source.Close()
			source.GitPath = filepath.Join(directory, "git")
			if err := source.acquire(context.Background(), root); err == nil {
				t.Fatal("source drift accepted")
			}
		})
	}
}

// VPO-V0-007: conflicts and non-commit/unresolvable HEAD are refused.
func TestWorkSourceRejectsUnmergedAndNonCommit(t *testing.T) {
	t.Run("unmerged", func(t *testing.T) {
		root := fixture(t)
		initial := strings.TrimSpace(string(runGit(t, root, "rev-parse", "HEAD")))
		runGit(t, root, "checkout", "-qb", "conflicting")
		write(t, root, "a.txt", []byte("other\n"), 0644)
		runGit(t, root, "add", "a.txt")
		commitFixture(t, root)
		runGit(t, root, "checkout", "-q", "--detach", initial)
		write(t, root, "a.txt", []byte("third\n"), 0644)
		runGit(t, root, "add", "a.txt")
		commitFixture(t, root)
		source, err := newSource()
		if err != nil {
			t.Fatal(err)
		}
		defer source.Close()
		source.Root = root
		if _, err := source.Git(context.Background(), 4<<20, "-c", "user.name=Source test", "-c", "user.email=source@example.invalid", "merge", "conflicting"); err == nil {
			t.Fatal("fixture did not conflict")
		}
		if got, err := Acquire(context.Background(), root); err == nil {
			got.Close()
			t.Fatal("unmerged source accepted")
		}
	})
	for _, kind := range []string{"tree", "unresolvable"} {
		t.Run(kind, func(t *testing.T) {
			root := fixture(t)
			target := strings.Repeat("a", 40)
			if kind == "tree" {
				target = strings.TrimSpace(string(runGit(t, root, "rev-parse", "HEAD^{tree}")))
			}
			write(t, root, ".git/HEAD", []byte(target+"\n"), 0644)
			if got, err := Acquire(context.Background(), root); err == nil {
				got.Close()
				t.Fatal("non-commit source accepted")
			}
		})
	}
}

// VPO-V0-010 / WQO-V0-005: local clean/process filters must be rejected before
// status can execute repository-configured commands against the caller tree.
func TestWorkSourceRejectsFiltersBeforeGitStatus(t *testing.T) {
	for _, kind := range []string{"clean", "process"} {
		t.Run(kind, func(t *testing.T) {
			root := fixture(t)
			write(t, root, ".gitattributes", []byte("a.txt filter=sentinel\n"), 0644)
			runGit(t, root, "add", ".gitattributes")
			commitFixture(t, root)
			directory := t.TempDir()
			script := filepath.Join(directory, "filter")
			write(t, directory, "filter", []byte("#!/bin/sh\ntouch filter-launched\ncat\n"), 0755)
			runGit(t, root, "config", "filter.sentinel."+kind, script)
			changed := time.Now().Add(time.Hour)
			if err := os.Chtimes(filepath.Join(root, "a.txt"), changed, changed); err != nil {
				t.Fatal(err)
			}
			if got, err := Acquire(context.Background(), root); err == nil {
				got.Close()
				t.Fatal("executable filter accepted")
			}
			if _, err := os.Stat(filepath.Join(root, "filter-launched")); !os.IsNotExist(err) {
				t.Fatal("filter executed during source acquisition")
			}
		})
	}
}

func TestWorkSourceStatusRefusesFilterInstalledAfterQualification(t *testing.T) {
	t.Run("EAF-V0-007", func(t *testing.T) {
		root := fixture(t)
		source, err := newSource()
		if err != nil {
			t.Fatal(err)
		}
		defer source.Close()
		source.Root, source.GitDir, source.CommonDir = root, filepath.Join(root, ".git"), filepath.Join(root, ".git")
		if err := source.unsupportedState(context.Background()); err != nil {
			t.Fatal(err)
		}
		write(t, root, ".gitattributes", []byte("*.txt filter=hostile\n"), 0o600)
		marker := filepath.Join(t.TempDir(), "filter-executed")
		runGit(t, root, "config", "filter.hostile.clean", "touch "+marker)
		write(t, root, "a.txt", []byte("bravo\n"), 0o600)
		old := time.Unix(1700000000, 0)
		if err := os.Chtimes(filepath.Join(root, "a.txt"), old, old); err != nil {
			t.Fatal(err)
		}
		_, _, err = source.indexState(context.Background())
		if _, statErr := os.Stat(marker); !os.IsNotExist(statErr) {
			t.Fatalf("filter installed after config check executed: %v", statErr)
		}
		if err == nil {
			t.Fatal("unsafe status accepted after config qualification")
		}
	})
}

// VPO-V0-010: the explicit producer seam nests acquisition storage in an owner
// parent, including initial root resolution; ordinary Acquire remains isolated.
func TestWorkSourceExplicitScratchOwnership(t *testing.T) {
	root := fixture(t)
	parent, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(parent, 0700); err != nil {
		t.Fatal(err)
	}
	resolved, err := ResolveRootWithScratch(context.Background(), root, parent)
	if err != nil || resolved != root {
		t.Fatal("owned root resolution failed", err)
	}
	source, err := AcquireWithScratch(context.Background(), root, parent)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(source.temporary, parent+string(os.PathSeparator)) {
		t.Fatal("acquisition escaped owned scratch")
	}
	if err := source.Close(); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(parent)
	if err != nil || len(entries) != 0 {
		t.Fatal("source scratch cleanup incomplete")
	}
}

// VPO-V0-008: use actual inode mappings to reject aliases, including filesystem
// Unicode normalization collisions that a case-fold string heuristic misses.
func TestWorkSourceRejectsFilesystemAliases(t *testing.T) {
	root := fixture(t)
	write(t, root, "é/one", []byte("same\n"), 0644)
	write(t, root, "e\u0301/two", []byte("same\n"), 0644)
	first, err := os.Stat(filepath.Join(root, "é"))
	if err != nil {
		t.Fatal(err)
	}
	second, err := os.Stat(filepath.Join(root, "e\u0301"))
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(first, second) {
		t.Skip("filesystem preserves distinct Unicode directory spellings")
	}
	oid := objectID("sha1", "blob", []byte("same\n"))
	source := &Source{Identity: workqueue.RepositorySource{ObjectFormat: "sha1"}, Entries: []Entry{{Path: "é/one", Mode: "100644", BlobOID: oid}, {Path: "e\u0301/two", Mode: "100644", BlobOID: oid}}}
	if err := source.readEntries(context.Background(), root, true); err == nil {
		t.Fatal("filesystem-normalized directory alias accepted")
	}
}

// WQO-V0-004,005 / VPO-V0-010: a forged scratch marker cannot authorize
// acquisition writes in an external linked-worktree common Git directory.
func TestWorkSourceRejectsScratchInLinkedCommonBeforeWrites(t *testing.T) {
	original := fixture(t)
	linked := t.TempDir()
	runGit(t, original, "worktree", "add", "--quiet", "--detach", linked, "HEAD")
	linked, err := filepath.EvalSymlinks(linked)
	if err != nil {
		t.Fatal(err)
	}
	common := filepath.Join(original, ".git")
	scratch := filepath.Join(common, "forged-scratch")
	if err := os.Mkdir(scratch, 0700); err != nil {
		t.Fatal(err)
	}
	before := gitManifest(t, common)
	if source, err := AcquireWithScratch(context.Background(), linked, scratch); err == nil {
		source.Close()
		t.Fatal("common Git scratch admitted")
	}
	if _, err := ResolveRootWithScratch(context.Background(), filepath.Join(linked, "directory"), scratch); err == nil {
		t.Fatal("nested cwd bypassed common Git scratch refusal")
	}
	after := gitManifest(t, common)
	if !bytes.Equal(before, after) {
		t.Fatal("scratch refusal wrote common Git metadata")
	}
}
