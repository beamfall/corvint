//go:build darwin || linux

package gitstatus

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"
)

func testRun(ctx context.Context, root string, _ int, args ...string) ([]byte, error) {
	prefix := []string{"--no-optional-locks", "-c", "core.fsmonitor=false", "-c", "core.untrackedCache=false", "-c", "core.excludesFile=", "-c", "submodule.recurse=false", "-C", root}
	cmd := exec.CommandContext(ctx, "git", append(prefix, args...)...)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_SYSTEM="+os.DevNull, "GIT_OPTIONAL_LOCKS=0", "GIT_CEILING_DIRECTORIES="+filepath.Dir(root))
	return cmd.Output()
}

func fixture(t *testing.T, initArguments ...string) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	gitTest(t, root, append([]string{"init", "-q"}, initArguments...)...)
	gitTest(t, root, "config", "user.email", "test@example.invalid")
	gitTest(t, root, "config", "user.name", "Test")
	writeTest(t, filepath.Join(root, "value.go"), "package value\n")
	gitTest(t, root, "add", "value.go")
	gitTest(t, root, "commit", "-qm", "fixture")
	return root
}

func gitTest(t *testing.T, root string, args ...string) []byte {
	t.Helper()
	out, err := testRun(context.Background(), root, metadataLimit, args...)
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return out
}

func writeTest(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestPrivateStatusMatchesCleanDirtyStagedIgnoredAndLinkedWorktree(t *testing.T) {
	root := fixture(t)
	compare := func(root string) {
		t.Helper()
		args := []string{"status", "--porcelain=v1", "-z", "--untracked-files=all"}
		want := gitTest(t, root, args...)
		got, err := Status(context.Background(), root, metadataLimit, testRun, args...)
		if err != nil || !bytes.Equal(got, want) {
			t.Fatalf("status=%q want=%q error=%v", got, want, err)
		}
	}
	compare(root)
	gitTest(t, root, "config", "user.name", "First\tLast")
	gitTest(t, root, "config", "core.bare", "FALSE")
	compare(root)
	writeTest(t, filepath.Join(root, "value.go"), "package value\nconst X=1\n")
	writeTest(t, filepath.Join(root, ".gitignore"), "*.ignored\n")
	writeTest(t, filepath.Join(root, "file.ignored"), "ignored")
	writeTest(t, filepath.Join(root, "untracked"), "new")
	compare(root)
	gitTest(t, root, "add", "value.go")
	compare(root)
	gitTest(t, root, "mv", "value.go", "renamed.go")
	gitTest(t, root, "pack-refs", "--all")
	compare(root)
	linked := filepath.Join(filepath.Dir(root), "linked")
	gitTest(t, root, "worktree", "add", "-qb", "linked", linked, "HEAD")
	compare(linked)
	writeTest(t, filepath.Join(linked, "value.go"), "package value\nconst Linked=1\n")
	compare(linked)
}

// Git strips trailing CR and LF from a `.git` pointer file, so a CRLF pointer
// names a valid linked worktree that the private status must not refuse.
func TestPrivateStatusAcceptsCRLFWorktreePointers(t *testing.T) {
	root := fixture(t)
	linked := filepath.Join(filepath.Dir(root), "linked")
	gitTest(t, root, "worktree", "add", "-qb", "linked", linked, "HEAD")
	gitdir := strings.TrimSpace(string(gitTest(t, linked, "rev-parse", "--git-dir")))
	for _, path := range []string{filepath.Join(linked, ".git"), filepath.Join(gitdir, "commondir")} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		writeTest(t, path, strings.TrimSuffix(string(data), "\n")+"\r\n")
	}
	writeTest(t, filepath.Join(linked, "untracked"), "new")
	args := []string{"status", "--porcelain=v1", "-z", "--untracked-files=all"}
	want := gitTest(t, linked, args...)
	got, err := Status(context.Background(), linked, metadataLimit, testRun, args...)
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("status=%q want=%q error=%v", got, want, err)
	}
}

func TestStatusRefusesUnsupportedMetadataBeforeLiveStatus(t *testing.T) {
	// EAF-V0-011: each refusal names its feature, key or Git-relative file.
	reasons := map[string]string{
		"clean":                   "repository config sets filter.hostile.clean",
		"process":                 "repository config sets filter.hostile.process",
		"include":                 "repository config uses an include directive (include.*)",
		"includeIf":               "repository config uses a conditional include (includeIf.*)",
		"worktree":                "repository config sets core.worktree to a directory other than the checkout",
		"attributes":              "repository config sets core.attributesFile",
		"bare":                    "repository config sets core.bare",
		"split-index":             "index is a split index",
		"gitlink":                 "index records a submodule (gitlink)",
		"config-symlink":          "metadata file config is a symlink",
		"index-symlink":           "metadata file index is a symlink",
		"objects-symlink":         "metadata directory objects is missing or not a plain directory",
		"refs-symlink":            "metadata directory refs is missing or not a plain directory",
		"git-symlink":             ".git is a symlink",
		"config-internal-symlink": "metadata file config is a symlink",
		"reftable":                "repository config sets extensions.refStorage=reftable, which is not supported",
		"gitdir-pointer-crlf":     ".git gitdir: pointer is empty or contains line breaks or NUL",
		"commondir-crlf":          "commondir is empty or contains line breaks or NUL",
		"snapshot-budget":         "metadata file info/exclude exceeds the remaining 64 MiB metadata snapshot budget",
		"oversized-packed-refs":   "metadata file packed-refs exceeds 32 MiB",
		"exclude-fifo":            "metadata file info/exclude is a FIFO",
		"temp-in-repo":            "scratch directory (TMPDIR) is inside the repository or its Git directory",
	}
	// Decision 0383: each refusal also carries its closed class.
	classes := map[string]reasonClass{
		"clean": classGitFilter, "process": classGitFilter, "include": classConfigInclude, "includeIf": classConfigInclude,
		"worktree": classWorktreeConfig, "attributes": classAttributesFile, "bare": classWorktreeConfig,
		"split-index": classSplitIndex, "gitlink": classSubmodule, "config-symlink": classMetadataUnreadable,
		"index-symlink": classMetadataUnreadable, "objects-symlink": classMetadataDirectory, "refs-symlink": classMetadataDirectory,
		"git-symlink": classGitdirPointer, "config-internal-symlink": classMetadataUnreadable, "reftable": classRefStorage,
		"gitdir-pointer-crlf": classGitdirPointer, "commondir-crlf": classGitdirPointer, "snapshot-budget": classMetadataLimit,
		"oversized-packed-refs": classMetadataLimit, "exclude-fifo": classMetadataUnreadable, "temp-in-repo": classScratchDir,
	}
	for _, kind := range []string{"clean", "process", "include", "includeIf", "worktree", "attributes", "bare", "split-index", "gitlink", "config-symlink", "index-symlink", "objects-symlink", "refs-symlink", "git-symlink", "config-internal-symlink", "reftable", "gitdir-pointer-crlf", "commondir-crlf", "snapshot-budget", "oversized-packed-refs", "exclude-fifo", "temp-in-repo"} {
		t.Run(kind, func(t *testing.T) {
			var initArgs []string
			if kind == "reftable" {
				initArgs = []string{"--ref-format=reftable"}
			}
			root := fixture(t, initArgs...)
			switch kind {
			case "clean", "process":
				gitTest(t, root, "config", "filter.hostile."+kind, "exit 42")
			case "include":
				gitTest(t, root, "config", "include.path", filepath.Join(t.TempDir(), "missing"))
			case "includeIf":
				gitTest(t, root, "config", "includeIf.gitdir:/.path", filepath.Join(t.TempDir(), "missing"))
			case "worktree":
				gitTest(t, root, "config", "core.worktree", t.TempDir())
			case "attributes":
				gitTest(t, root, "config", "core.attributesFile", filepath.Join(t.TempDir(), "outside"))
			case "bare":
				gitTest(t, root, "config", "core.bare", "true")
			case "split-index":
				gitTest(t, root, "update-index", "--split-index")
			case "gitlink":
				oid := strings.TrimSpace(string(gitTest(t, root, "rev-parse", "HEAD")))
				gitTest(t, root, "update-index", "--add", "--cacheinfo", "160000,"+oid+",child")
			case "config-symlink", "index-symlink", "objects-symlink", "refs-symlink":
				name := strings.TrimSuffix(kind, "-symlink")
				path := filepath.Join(root, ".git", name)
				target := filepath.Join(t.TempDir(), name)
				if err := os.Rename(path, target); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(target, path); err != nil {
					t.Fatal(err)
				}
			case "git-symlink":
				path := filepath.Join(root, ".git")
				target := filepath.Join(t.TempDir(), ".git")
				if err := os.Rename(path, target); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(target, path); err != nil {
					t.Fatal(err)
				}
			case "config-internal-symlink":
				path := filepath.Join(root, ".git", "config")
				if err := os.Rename(path, path+".real"); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink("config.real", path); err != nil {
					t.Fatal(err)
				}
			case "gitdir-pointer-crlf":
				// The gitdir a `.git` pointer file names may carry a literal CR
				// byte in a real directory name; only interior CR/LF in the raw
				// pointer content itself must be refused, not merely trailing.
				actual := filepath.Join(t.TempDir(), "real\rgit")
				if err := os.Rename(filepath.Join(root, ".git"), actual); err != nil {
					t.Fatal(err)
				}
				writeTest(t, filepath.Join(root, ".git"), "gitdir: "+actual+"\n")
			case "commondir-crlf":
				// Split gitdir from a common directory reached only through a
				// symlink alias whose name carries an interior CR/LF, so the
				// alias resolves cleanly while the raw commondir bytes do not.
				admin := filepath.Join(t.TempDir(), "admin")
				if err := os.Rename(filepath.Join(root, ".git"), admin); err != nil {
					t.Fatal(err)
				}
				writeTest(t, filepath.Join(root, ".git"), "gitdir: "+admin+"\n")
				alias := filepath.Join(t.TempDir(), "alias\rreal")
				if err := os.Symlink(admin, alias); err != nil {
					t.Fatal(err)
				}
				writeTest(t, filepath.Join(admin, "commondir"), alias+"\n")
			case "snapshot-budget":
				// packed-refs and shallow alone stay under the per-file 32 MiB
				// cap; the 64 MiB total across all captured files is what must
				// refuse the final, still individually-small, info/exclude read.
				for _, name := range []string{"packed-refs", "shallow"} {
					path := filepath.Join(root, ".git", name)
					if err := os.WriteFile(path, nil, 0o600); err != nil {
						t.Fatal(err)
					}
					if err := os.Truncate(path, 25<<20); err != nil {
						t.Fatal(err)
					}
				}
				if err := os.Truncate(filepath.Join(root, ".git", "info", "exclude"), 20<<20); err != nil {
					t.Fatal(err)
				}
			case "oversized-packed-refs":
				path := filepath.Join(root, ".git", "packed-refs")
				if err := os.WriteFile(path, nil, 0o600); err != nil {
					t.Fatal(err)
				}
				if err := os.Truncate(path, metadataLimit+1); err != nil {
					t.Fatal(err)
				}
			case "exclude-fifo":
				path := filepath.Join(root, ".git", "info", "exclude")
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				if err := syscall.Mkfifo(path, 0o600); err != nil {
					t.Fatal(err)
				}
			case "temp-in-repo":
				t.Setenv("TMPDIR", root)
			}
			called := false
			run := func(ctx context.Context, dir string, limit int, args ...string) ([]byte, error) {
				for _, arg := range args {
					if arg == "status" {
						called = true
					}
				}
				return testRun(ctx, dir, limit, args...)
			}
			_, err := Status(context.Background(), root, metadataLimit, run, "status", "--porcelain=v1", "-z")
			if err == nil {
				t.Fatal("unsafe repository accepted")
			}
			if called {
				t.Fatal("unsafe metadata reached status")
			}
			if got := RefusalClass(err); got != string(classes[kind]) {
				t.Fatalf("refusal class=%q want %q: %v", got, classes[kind], err)
			}
			message := RefusalMessage(err)
			if !errors.Is(err, errUnsafe) || message != "Git status cannot safely observe repository metadata: "+reasons[kind] {
				t.Fatalf("refusal lost its specific reason: %v", err)
			}
			if strings.Contains(message, root) || strings.Contains(message, "exit 42") || strings.Contains(message, os.TempDir()) {
				t.Fatalf("refusal leaked a config value or outside path: %s", message)
			}
		})
	}
}

// A runner that adds no command-line fsmonitor override (companionrelease's
// does not) must still never execute a repository-configured fsmonitor hook.
func TestStatusNeverExecutesRepositoryConfiguredFSMonitor(t *testing.T) {
	root := fixture(t)
	marker := filepath.Join(t.TempDir(), "fsmonitor-ran")
	hook := filepath.Join(t.TempDir(), "hostile-fsmonitor.sh")
	writeTest(t, hook, "#!/bin/sh\ntouch '"+marker+"'\nexit 1\n")
	if err := os.Chmod(hook, 0o700); err != nil {
		t.Fatal(err)
	}
	gitTest(t, root, "config", "core.fsmonitor", hook)
	run := func(ctx context.Context, dir string, _ int, args ...string) ([]byte, error) {
		cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_SYSTEM="+os.DevNull)
		return cmd.Output()
	}
	_, _ = Status(context.Background(), root, metadataLimit, run, "status", "--porcelain=v1", "-z")
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("repository-configured fsmonitor executed: %v", err)
	}
}

func TestConfigReplacementDuringStatusCannotExecuteDriverOrRedirectRoot(t *testing.T) {
	root := fixture(t)
	marker := filepath.Join(t.TempDir(), "marker")
	outside := t.TempDir()
	writeTest(t, filepath.Join(root, ".gitattributes"), "*.go filter=hostile\n")
	writeTest(t, filepath.Join(root, "value.go"), "package value\nconst Dirty=1\n")
	run := func(ctx context.Context, dir string, limit int, args ...string) ([]byte, error) {
		for _, arg := range args {
			if arg == "status" {
				gitTest(t, root, "config", "filter.hostile.clean", "touch '"+marker+"'; cat")
				gitTest(t, root, "config", "core.worktree", outside)
			}
		}
		return testRun(ctx, dir, limit, args...)
	}
	_, err := Status(context.Background(), root, metadataLimit, run, "status", "--porcelain=v1", "-z")
	if err == nil {
		t.Fatal("config drift accepted")
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("driver executed: %v", err)
	}
}

func TestPinnedMetadataReaderDetectsParentReplacement(t *testing.T) {
	for _, kind := range []string{"directory", "symlink"} {
		t.Run(kind, func(t *testing.T) {
			root := fixture(t)
			gitdir := filepath.Join(root, ".git")
			backup := filepath.Join(t.TempDir(), "original")
			replacement := filepath.Join(t.TempDir(), "replacement")
			if err := os.CopyFS(replacement, os.DirFS(gitdir)); err != nil {
				t.Fatal(err)
			}
			run := func(ctx context.Context, dir string, limit int, args ...string) ([]byte, error) {
				for _, arg := range args {
					if arg != "status" {
						continue
					}
					if err := os.Rename(gitdir, backup); err != nil {
						t.Fatal(err)
					}
					var err error
					if kind == "symlink" {
						err = os.Symlink(backup, gitdir)
					} else {
						err = os.Rename(replacement, gitdir)
					}
					if err != nil {
						t.Fatal(err)
					}
				}
				return testRun(ctx, dir, limit, args...)
			}
			if _, err := Status(context.Background(), root, metadataLimit, run, "status", "--porcelain=v1", "-z"); err == nil {
				t.Fatal("parent replacement hidden by pinned metadata handles")
			}
		})
	}
}

// A parent the caller can search but not read (mode 0711 owned by someone
// else, 0311 here) is how Git itself sees many home and shared directories, so
// status reads through it (EAF-V0-012). Replacing that parent is still refused
// even when the repository inside it moves along unchanged, so only the
// parent's own pinned identity can catch it.
func TestStatusReadsThroughSearchOnlyParent(t *testing.T) {
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	parent := filepath.Join(base, "search-only")
	swap := filepath.Join(base, "swap")
	original := filepath.Join(base, "original")
	for _, dir := range []string{parent, swap} {
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	root := filepath.Join(parent, "repo")
	gitTest(t, base, "init", "-q", root)
	writeTest(t, filepath.Join(root, "value.go"), "package value\n")
	gitTest(t, root, "add", "value.go")
	gitTest(t, root, "-c", "user.email=test@example.invalid", "-c", "user.name=Test", "commit", "-qm", "fixture")
	if err := os.Chmod(parent, 0o311); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{parent, original} {
		t.Cleanup(func() { os.Chmod(dir, 0o755) })
	}
	if file, err := os.Open(parent); err == nil {
		file.Close()
		t.Skip("directory read permission is not enforced for this user")
	}
	args := []string{"status", "--porcelain=v1", "-z", "--untracked-files=all"}
	want := gitTest(t, root, args...)
	got, err := Status(context.Background(), root, metadataLimit, testRun, args...)
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("status=%q want=%q error=%v", got, want, err)
	}
	run := func(ctx context.Context, dir string, limit int, args ...string) ([]byte, error) {
		for _, arg := range args {
			if arg != "status" {
				continue
			}
			for _, move := range [][2]string{{root, filepath.Join(swap, "repo")}, {parent, original}, {swap, parent}} {
				if err := os.Rename(move[0], move[1]); err != nil {
					t.Fatal(err)
				}
			}
		}
		return testRun(ctx, dir, limit, args...)
	}
	_, err = Status(context.Background(), root, metadataLimit, run, args...)
	if !errors.Is(err, errUnsafe) || !strings.Contains(err.Error(), "replaced") || RefusalClass(err) != string(classMetadataDrift) {
		t.Fatalf("search-only parent replacement: %v class=%q", err, RefusalClass(err))
	}
}

func TestPrivateMetadataIsRemovedAfterCancellation(t *testing.T) {
	root := fixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var temporary string
	run := func(_ context.Context, directory string, _ int, _ ...string) ([]byte, error) {
		temporary = directory
		cancel()
		return nil, ctx.Err()
	}
	if _, err := Status(ctx, root, metadataLimit, run, "status", "--porcelain=v1", "-z"); err != context.Canceled {
		t.Fatalf("cancellation error=%v", err)
	}
	if temporary == "" {
		t.Fatal("test did not reach private metadata")
	}
	if _, err := os.Stat(temporary); !os.IsNotExist(err) {
		t.Fatalf("private metadata survived: %v", err)
	}
}

func TestStatusSeparatesMetadataProbeAndExecutionErrors(t *testing.T) {
	root := fixture(t)
	config := filepath.Join(root, ".git", "config")
	raw, err := os.ReadFile(config)
	if err != nil {
		t.Fatal(err)
	}
	writeTest(t, config, string(raw)+"# use private Git probes\n")
	for _, stage := range []string{"config", "ls-files", "rev-parse", "status"} {
		for _, cause := range []error{errors.New("probe diagnostics"), context.Canceled, context.DeadlineExceeded} {
			t.Run(stage+"/"+cause.Error(), func(t *testing.T) {
				called := false
				run := func(_ context.Context, _ string, _ int, args ...string) ([]byte, error) {
					if slices.Contains(args, stage) {
						called = true
						return nil, cause
					}
					return nil, nil
				}
				_, err := Status(context.Background(), root, metadataLimit, run, "status", "--porcelain=v1", "-z")
				if !called || !errors.Is(err, cause) || err.Error() != cause.Error() {
					t.Fatalf("stage=%s called=%v error=%v want=%v", stage, called, err, cause)
				}
				var probe *MetadataProbeError
				wantProbe := stage != "status" && cause != context.Canceled && cause != context.DeadlineExceeded
				if errors.As(err, &probe) != wantProbe {
					t.Fatalf("stage=%s error=%#v metadata probe=%v", stage, err, wantProbe)
				}
				if !wantProbe && err != cause {
					t.Fatalf("ordinary execution or cancellation error identity changed: %#v", err)
				}
			})
		}
	}
}

func TestStatusRejectsScratchDirectoryAliases(t *testing.T) {
	original := fixture(t)
	root := filepath.Join(t.TempDir(), "CorvintRepository")
	if err := os.Rename(original, root); err != nil {
		t.Fatal(err)
	}
	linked := filepath.Join(t.TempDir(), "LinkedRepository")
	gitTest(t, root, "worktree", "add", "-qb", "linked", linked, "HEAD")
	linkedGit := strings.TrimSpace(string(gitTest(t, linked, "rev-parse", "--absolute-git-dir")))
	for _, scope := range []struct{ root, target string }{
		{root, root}, {root, filepath.Join(root, ".git")},
		{linked, linked}, {linked, linkedGit}, {linked, filepath.Join(root, ".git")},
	} {
		alias := filepath.Join(filepath.Dir(scope.target), strings.ToUpper(filepath.Base(scope.target)))
		actual, err := os.Stat(scope.target)
		if err != nil {
			t.Fatal(err)
		}
		aliased, err := os.Stat(alias)
		if err != nil || !os.SameFile(actual, aliased) || alias == scope.target {
			t.Skip("filesystem does not expose a distinct case alias")
		}
		for _, parent := range []string{alias, filepath.Join(alias, "scratch-parent")} {
			if err := os.MkdirAll(parent, 0o700); err != nil {
				t.Fatal(err)
			}
			called := false
			run := func(context.Context, string, int, ...string) ([]byte, error) {
				called = true
				return nil, nil
			}
			_, err := StatusIn(context.Background(), scope.root, parent, metadataLimit, run, "status", "--porcelain=v1", "-z")
			if !errors.Is(err, errUnsafe) || called {
				t.Fatalf("scratch alias %q reached Git=%v error=%v", parent, called, err)
			}
			entries, err := os.ReadDir(parent)
			if err != nil {
				t.Fatal(err)
			}
			for _, entry := range entries {
				if strings.HasPrefix(entry.Name(), "corvint-git-status-") {
					t.Fatalf("scratch alias retained private metadata: %s", entry.Name())
				}
			}
		}
	}
}

func TestStatusAcceptsDeepExternalScratchParent(t *testing.T) {
	root := fixture(t)
	parent := filepath.Join(t.TempDir(), strings.Repeat("d/", 129))
	if err := os.MkdirAll(parent, 0o700); err != nil {
		t.Fatal(err)
	}
	result, err := StatusIn(context.Background(), root, parent, metadataLimit, testRun, "status", "--porcelain=v1", "-z")
	if err != nil || len(result) != 0 {
		t.Fatalf("external deep scratch status=%q error=%v", result, err)
	}
	entries, err := os.ReadDir(parent)
	if err != nil || len(entries) != 0 {
		t.Fatalf("external deep scratch retained private metadata: %v error=%v", entries, err)
	}
}

func TestConfigParserRejectsAmbiguousMultilineAndControlRecords(t *testing.T) {
	for _, raw := range []string{
		"filter.hidden\n.clean\ntouch marker\x00",
		"user.name\nfirst\nsecond\x00",
		"user.\tname\nvalue\x00",
		"filter.hidden.clean\ntouch marker\x00",
		"user.name\x7f\nvalue\x00",
	} {
		if class, reason := unsafeConfig([]byte(raw), "/repo", "/repo/.git"); reason == "" || class == classUnclassified {
			t.Fatalf("accepted unsafe config record %q", raw)
		}
	}
	if _, reason := unsafeConfig([]byte("user.name\nFirst\tLast\x00"), "/repo", "/repo/.git"); reason != "" {
		t.Fatal("inert tabbed config value refused")
	}
	if _, reason := unsafeConfig([]byte("filter.hostile.clean\n\x00"), "/repo", "/repo/.git"); reason != "" {
		t.Fatal("empty filter driver value refused")
	}
}

// within is the sole containment guard between scratch metadata and the
// repository it observes: a candidate exactly one level above the parent
// must never count as inside it, only genuine descendants may.
func TestWithinRejectsGrandparentAsAncestor(t *testing.T) {
	if within("/a/b", "/a") {
		t.Fatal("path one level above parent treated as within it")
	}
	if !within("/a/b", "/a/b/c") {
		t.Fatal("actual descendant not treated as within")
	}
}

func TestCancelledIsolationWaitDoesNotReadMetadataOrStartGit(t *testing.T) {
	for range cap(isolationSlot) {
		isolationSlot <- struct{}{}
	}
	held := true
	defer func() {
		if held {
			for range cap(isolationSlot) {
				<-isolationSlot
			}
		}
	}()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	run := func(context.Context, string, int, ...string) ([]byte, error) {
		t.Fatal("cancelled waiter started Git")
		return nil, nil
	}
	if _, err := Status(ctx, "/nonexistent", metadataLimit, run, "status"); err != context.Canceled {
		t.Fatalf("queued cancellation=%v", err)
	}
	for range cap(isolationSlot) {
		<-isolationSlot
	}
	held = false
	if _, err := Status(context.Background(), fixture(t), metadataLimit, testRun, "status", "--porcelain=v1", "-z"); err != nil {
		t.Fatalf("next status after cancelled waiter=%v", err)
	}
}

func TestMetadataReaderRefusesFIFOWithoutBlockingAndHonorsRemainingBudget(t *testing.T) {
	directory, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	fifo := filepath.Join(directory, "fifo")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}
	result := make(chan error, 1)
	go func() { _, _, err := readRegular(fifo, 4096); result <- err }()
	select {
	case err := <-result:
		if err == nil {
			t.Fatal("FIFO admitted as metadata")
		}
	case <-time.After(time.Second):
		t.Fatal("metadata FIFO blocked")
	}
	file := filepath.Join(directory, "metadata")
	writeTest(t, file, "12345")
	if _, err := captureLimited(file, 4); err == nil {
		t.Fatal("remaining snapshot budget ignored")
	}
}

// A private index must retain Git's racy-clean boundary; copying bytes alone
// changes status even when neither repository metadata nor worktree bytes move.
func TestPrivateStatusPreservesIndexTimestamp(t *testing.T) {
	root := fixture(t)
	// Make an equal-size edit invisible to cached stat fields. Only Git's
	// original index timestamp forces hashing this racily clean entry.
	gitTest(t, root, "config", "core.trustctime", "false")
	gitTest(t, root, "config", "core.checkStat", "minimal")
	valuePath := filepath.Join(root, "value.go")
	stamp := time.Unix(1_700_000_000, 123_456_000)
	if err := os.Chtimes(valuePath, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	gitTest(t, root, "add", "value.go")
	writeTest(t, valuePath, "package other\n")
	if err := os.Chtimes(valuePath, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	indexPath := filepath.Join(root, ".git", "index")
	original, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(indexPath, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	originalInfo, err := os.Stat(indexPath)
	if err != nil {
		t.Fatal(err)
	}
	statusCalls := 0
	run := func(ctx context.Context, dir string, limit int, args ...string) ([]byte, error) {
		if slices.Contains(args, "status") {
			statusCalls++
			privateInfo, err := os.Stat(filepath.Join(dir, "metadata", "index"))
			if err != nil || !privateInfo.ModTime().Equal(originalInfo.ModTime()) {
				t.Fatalf("private index lost captured timestamp: info=%v error=%v", privateInfo, err)
			}
		}
		return testRun(ctx, dir, limit, args...)
	}
	args := []string{"status", "--porcelain=v1", "-z", "--untracked-files=all"}
	want := gitTest(t, root, args...)
	if string(want) != " M value.go\x00" {
		t.Fatalf("racy-clean control did not detect equal-size edit: %q", want)
	}
	for range 3 {
		got, err := Status(context.Background(), root, metadataLimit, run, args...)
		if err != nil || !bytes.Equal(got, want) {
			t.Fatalf("status=%q want=%q error=%v", got, want, err)
		}
	}
	after, err := os.ReadFile(indexPath)
	if err != nil || !bytes.Equal(original, after) || statusCalls != 3 {
		t.Fatalf("live index changed or missing status calls: calls=%d error=%v", statusCalls, err)
	}
}

func TestPrivateStatusRejectsIndexTimestampDrift(t *testing.T) {
	root := fixture(t)
	indexPath := filepath.Join(root, ".git", "index")
	info, err := os.Stat(indexPath)
	if err != nil {
		t.Fatal(err)
	}
	run := func(ctx context.Context, dir string, limit int, args ...string) ([]byte, error) {
		if slices.Contains(args, "status") {
			stamp := info.ModTime().Add(-time.Hour)
			if err := os.Chtimes(indexPath, stamp, stamp); err != nil {
				t.Fatal(err)
			}
		}
		return testRun(ctx, dir, limit, args...)
	}
	if _, err := Status(context.Background(), root, metadataLimit, run, "status", "--porcelain=v1", "-z"); err == nil {
		t.Fatal("changed index racy-clean boundary admitted")
	}
}

func TestFilterKeyBoundsDriverName(t *testing.T) {
	for key, want := range map[string]string{
		"filter.lfs.process":                           "filter.lfs.process",
		"filter.a.b.clean":                             "filter.*.clean",
		"filter.clean":                                 "filter.*.clean",
		"filter.leak-\u009b31msecret.clean":            "filter.*.clean",
		"filter." + strings.Repeat("x", 33) + ".clean": "filter.*.clean",
	} {
		if got := filterKey(key); got != want {
			t.Errorf("filterKey(%q) = %q, want %q", key, got, want)
		}
	}
}
