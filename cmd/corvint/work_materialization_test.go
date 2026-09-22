package main

import (
	"context"
	"crypto/sha256"
	"fmt"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/workqueue"
	"github.com/Beamfall/corvint/internal/worksource"
)

func materializationGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "git", append([]string{"-C", root}, args...)...)
	command.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1", "GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.invalid", "GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.invalid")
	command.WaitDelay = time.Second
	workContain(command)
	defer workKillGroup(command)
	raw, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, raw)
	}
	return strings.TrimSpace(string(raw))
}

func materializationFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	materializationGit(t, root, "init", "-q")
	// A commit must not spawn detached auto maintenance: it outlives the fixture
	// command and its transient .git writes race the manifests.
	materializationGit(t, root, "config", "maintenance.auto", "false")
	materializationGit(t, root, "config", "gc.auto", "0")
	for name, raw := range map[string]string{"tracked": "pinned bytes\n", ".gitignore": "ignored\n"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(raw), 0644); err != nil {
			t.Fatal(err)
		}
	}
	materializationGit(t, root, "add", ".")
	materializationGit(t, root, "commit", "-qm", "fixture")
	return root
}

func materializationManifest(t *testing.T, root string) map[string]string {
	t.Helper()
	result := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		result[strings.TrimPrefix(path, root)] = fmt.Sprintf("%o:%x", info.Mode(), sha256.Sum256(raw))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

// WQO-V0-004/005 and VPO-V0-022: target files, Git metadata and scratch are private.
func TestWorkMaterializationNoCallerGitWrites(t *testing.T) {
	t.Parallel()
	caller := materializationFixture(t)
	if err := os.WriteFile(filepath.Join(caller, "ignored"), []byte("caller secret"), 0600); err != nil {
		t.Fatal(err)
	}
	before := materializationManifest(t, caller)
	source, err := worksource.Acquire(context.Background(), caller)
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	run, err := newWorkMaterialization(context.Background(), source)
	if err != nil {
		t.Fatal(err)
	}
	defer run.Close()
	if run.target == caller {
		t.Fatal("executing caller")
	}
	if _, err := os.Stat(filepath.Join(run.target, "ignored")); !os.IsNotExist(err) {
		t.Fatal("caller ignored data copied")
	}
	gitDir := materializationGit(t, run.target, "rev-parse", "--absolute-git-dir")
	if gitDir != filepath.Join(run.target, ".git") {
		t.Fatalf("shared Git metadata: %s", gitDir)
	}
	if !reflect.DeepEqual(before, materializationManifest(t, caller)) {
		t.Fatal("caller files or Git changed")
	}
	if err := os.WriteFile(filepath.Join(run.target, "tracked"), []byte("changed"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := run.Verify(context.Background()); err == nil {
		t.Fatal("changed target verified")
	}
	runRoot := run.root
	run.Close()
	if _, err := os.Stat(runRoot); !os.IsNotExist(err) {
		t.Fatal("run root remains")
	}
	if !reflect.DeepEqual(before, materializationManifest(t, caller)) {
		t.Fatal("failure cleanup changed caller")
	}
}

// VPO-V0-022: the adapter sees exactly the accepted names, without caller state.
func TestWorkMaterializationExactEnvironment(t *testing.T) {
	t.Parallel()
	environment, err := workChildEnvironment("/private/home", "/private/tmp")
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, item := range environment {
		name, _, _ := strings.Cut(item, "=")
		if names[name] {
			t.Fatal("duplicate environment")
		}
		names[name] = true
	}
	expected := []string{"PATH", "LANG", "LC_ALL", "TZ", "NO_COLOR", "GIT_CONFIG_NOSYSTEM", "GIT_TERMINAL_PROMPT", "GIT_OPTIONAL_LOCKS", "HOME", "TMPDIR", "GOTOOLCHAIN"}
	if len(names) != len(expected) {
		t.Fatalf("environment %v", names)
	}
	for _, name := range expected {
		if !names[name] {
			t.Fatalf("missing %s", name)
		}
	}
}

var workProductionSeed struct {
	once  sync.Once
	root  string
	ready bool
}

var workCompilerCache struct {
	once sync.Once
	root string
	err  error
}

// workCaptureSlots bounds how many parallel producer captures run at once. A
// capture's Git, bash and go build children run under finite hang detectors;
// twelve concurrent captures previously overran budget-sized bounds as
// SOURCE_UNQUALIFIED and ADAPTER_FAILED.
var workCaptureSlots = make(chan struct{}, 4)

// workCaptureSlot holds a slot until t ends. Call it only from a leaf test: a
// parent holding a slot while its parallel subtests wait can deadlock.
func workCaptureSlot(t *testing.T) {
	t.Helper()
	workCaptureSlots <- struct{}{}
	t.Cleanup(func() { <-workCaptureSlots })
}

func TestMain(m *testing.M) {
	code := m.Run()
	for name, root := range map[string]string{"work production seed": workProductionSeed.root, "work compiler cache": workCompilerCache.root, "work bound build": workBoundBuild.root} {
		if root == "" {
			continue
		}
		if err := os.RemoveAll(root); err != nil {
			fmt.Fprintln(os.Stderr, "remove "+name+":", err)
			code = 1
		}
	}
	os.Exit(code)
}

// workSharedCompilerPrelude gives fixtures that already substitute the adapter
// script one package-run cache for the run-private fallback (decision 0111).
// The script normally selects its safe per-user cache instead; only when that
// cache is unusable does this link become its GOCACHE. The script still builds
// the materialized target, and changed source inputs still recompile.
func workSharedCompilerPrelude(t *testing.T) string {
	t.Helper()
	workCompilerCache.once.Do(func() {
		workCompilerCache.root, workCompilerCache.err = os.MkdirTemp("", "corvint-work-compiler-cache-")
	})
	if workCompilerCache.err != nil {
		t.Fatal(workCompilerCache.err)
	}
	return `work_fixture_owner="${TMPDIR:-/tmp}/corvint-work-queue-owner"
if [ -f "$work_fixture_owner/tuple" ] && [ ! -e "$work_fixture_owner/build" ]; then
  mkdir "$work_fixture_owner/build" && ln -s '` + workCompilerCache.root + `' "$work_fixture_owner/build/cache"
fi`
}

// workProductionFixture pins the actual producer once, then gives each test a
// private checkout and Git object store. No runtime state is shared; the actual
// script's producer build reuses its content-keyed per-user Go cache (WQO-V0-005).
func workProductionFixture(t *testing.T) string {
	t.Helper()
	workProductionSeed.once.Do(func() {
		var err error
		workProductionSeed.root, err = os.MkdirTemp("", "corvint-work-production-seed-")
		if err != nil {
			t.Fatal(err)
		}
		buildWorkProductionSeed(t, workProductionSeed.root)
		workProductionSeed.ready = true
	})
	if !workProductionSeed.ready {
		t.Fatal("work production seed initialization failed")
	}
	caller := t.TempDir()
	materializationGit(t, caller, "clone", "--quiet", "--local", "--no-hardlinks", workProductionSeed.root, ".")
	materializationGit(t, caller, "remote", "remove", "origin")
	return caller
}

func buildWorkProductionSeed(t *testing.T, caller string) {
	t.Helper()
	repo, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	for _, directory := range repositoryImportClosure(t, repo, "cmd/corvint-work-queue") {
		entries, err := os.ReadDir(filepath.Join(repo, directory))
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range entries {
			name := entry.Name()
			if entry.IsDir() || (!strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, ".json")) || strings.HasSuffix(name, "_test.go") {
				continue
			}
			raw, err := os.ReadFile(filepath.Join(repo, directory, name))
			if err != nil {
				t.Fatal(err)
			}
			target := filepath.Join(caller, directory, name)
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(target, raw, 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
	for _, relative := range []string{"go.mod", "script/corvint-work-queue", ".corvint/work-queue-policy.json"} {
		raw, err := os.ReadFile(filepath.Join(repo, relative))
		if err != nil {
			t.Fatal(err)
		}
		target := filepath.Join(caller, relative)
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			t.Fatal(err)
		}
		mode := fs.FileMode(0644)
		if strings.HasPrefix(relative, "script/") {
			mode = 0755
		}
		if err := os.WriteFile(target, raw, mode); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(caller, "docs"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(caller, "docs/worklist.json"), []byte(`{"profile":"corvint-worklist/0","tickets":[]}`), 0644); err != nil {
		t.Fatal(err)
	}
	materializationGit(t, caller, "init", "-q")
	materializationGit(t, caller, "add", ".")
	materializationGit(t, caller, "commit", "-qm", "actual producer fixture")
	materializationGit(t, caller, "commit", "--allow-empty", "-qm", "pinned descendant without exported parent")
}

// repositoryImportClosure is the repository import closure of the main package
// directory, read from every non-test file so no build tag drops a dependency.
// A fixture seeded with only these packages builds the same binary, while the
// product's scans of that fixture stop reading the rest of internal/.
func repositoryImportClosure(t *testing.T, repo, main string) []string {
	t.Helper()
	const module = "github.com/Beamfall/corvint/"
	pending, seen := []string{main}, map[string]bool{main: true}
	for len(pending) != 0 {
		directory := pending[0]
		pending = pending[1:]
		files, err := filepath.Glob(filepath.Join(repo, directory, "*.go"))
		if err != nil {
			t.Fatal(err)
		}
		for _, file := range files {
			if strings.HasSuffix(file, "_test.go") {
				continue
			}
			parsed, err := parser.ParseFile(token.NewFileSet(), file, nil, parser.ImportsOnly)
			if err != nil {
				t.Fatal(err)
			}
			for _, spec := range parsed.Imports {
				dependency, ok := strings.CutPrefix(strings.Trim(spec.Path.Value, `"`), module)
				if ok && !seen[dependency] {
					seen[dependency] = true
					pending = append(pending, dependency)
				}
			}
		}
	}
	directories := make([]string, 0, len(seen))
	for directory := range seen {
		directories = append(directories, directory)
	}
	sort.Strings(directories)
	return directories
}

// WQO-V0-004/005: actual script and producer agree with the observer source.
func TestWorkSelfAdapterSourceParity(t *testing.T) {
	t.Parallel()
	workCaptureSlot(t)
	caller := workProductionFixture(t)
	before := materializationManifest(t, caller)
	source, err := worksource.Acquire(context.Background(), caller)
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	run, err := newWorkMaterialization(context.Background(), source)
	if err != nil {
		t.Fatal(err)
	}
	defer run.Close()
	var cached os.FileInfo
	for _, operation := range []string{"snapshot", "verify"} {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		command := exec.CommandContext(ctx, filepath.Join(run.target, "script/corvint-work-queue"), operation)
		command.Dir, command.Env = run.target, run.environment
		command.WaitDelay = time.Second
		workContain(command)
		command.Cancel = func() error { workKillGroup(command); return nil }
		raw, err := command.Output()
		workKillGroup(command)
		cancel()
		if err != nil {
			if exit, ok := err.(*exec.ExitError); ok {
				t.Fatalf("%s: %v: %s", operation, err, exit.Stderr)
			}
			t.Fatal(err)
		}
		var identity workqueue.RepositorySource
		if operation == "snapshot" {
			document, err := workqueue.ParseSnapshot(raw)
			if err != nil {
				t.Fatal(err)
			}
			identity = document.RepositorySource
		} else {
			document, err := workqueue.ParseCheckpoint(raw)
			if err != nil {
				t.Fatal(err)
			}
			identity = document.RepositorySource
		}
		if identity != source.Identity {
			t.Fatalf("%s source mismatch: %#v != %#v", operation, identity, source.Identity)
		}
		if err := run.Verify(context.Background()); err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(filepath.Join(run.tmp, "corvint-work-queue-owner/build/corvint-work-queue"))
		if err != nil {
			t.Fatal(err)
		}
		if cached != nil && (!os.SameFile(cached, info) || cached.ModTime() != info.ModTime()) {
			t.Fatal("same-run executable was rebuilt")
		}
		cached = info
	}
	if !reflect.DeepEqual(before, materializationManifest(t, caller)) {
		t.Fatal("actual script changed caller")
	}
	cache := filepath.Join(run.tmp, "corvint-work-queue-owner/build/corvint-work-queue")
	if _, err := os.Stat(cache); err != nil {
		t.Fatal("missing run-owned reusable build", err)
	}
}

// WQO-V0-004/005: private Git rebinding and added target entries fail before reuse.
func TestWorkMaterializationRejectsRebinding(t *testing.T) {
	t.Parallel()
	caller := materializationFixture(t)
	source, err := worksource.Acquire(context.Background(), caller)
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	mutations := map[string]func(*workMaterialization) error{
		"git-config": func(run *workMaterialization) error {
			return os.WriteFile(filepath.Join(run.target, ".git", "config"), []byte("[core]\nworktree = "+caller+"\n"), 0644)
		},
		"git-symlink": func(run *workMaterialization) error {
			if err := os.RemoveAll(filepath.Join(run.target, ".git")); err != nil {
				return err
			}
			return os.Symlink(filepath.Join(caller, ".git"), filepath.Join(run.target, ".git"))
		},
		"added-empty-directory": func(run *workMaterialization) error { return os.Mkdir(filepath.Join(run.target, "unexpected"), 0755) },
		"added-file": func(run *workMaterialization) error {
			return os.WriteFile(filepath.Join(run.target, "unexpected"), nil, 0644)
		},
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			run, err := newWorkMaterialization(context.Background(), source)
			if err != nil {
				t.Fatal(err)
			}
			defer run.Close()
			if err := mutate(run); err != nil {
				t.Fatal(err)
			}
			if err := run.Verify(context.Background()); err == nil {
				t.Fatal("modified execution target verified")
			}
		})
	}
}

// WQO-V0-004: cancellation and construction failure remove allocated run roots.
func TestWorkRunRootCleanup(t *testing.T) {
	t.Parallel()
	caller := materializationFixture(t)
	source, err := worksource.Acquire(context.Background(), caller)
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	for _, cancelled := range []bool{true, false} {
		t.Run(fmt.Sprint(cancelled), func(t *testing.T) {
			parent := t.TempDir()
			before, err := filepath.Glob(filepath.Join(parent, "corvint-work-run-*"))
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.WithValue(context.Background(), workRunParentKey{}, parent))
			defer cancel()
			malformed := *source
			if cancelled {
				cancel()
			} else {
				malformed.Identity.ObjectFormat = "invalid-object-format"
			}
			run, err := newWorkMaterialization(ctx, &malformed)
			if err == nil {
				run.Close()
				t.Fatal("expected construction failure")
			}
			after, err := filepath.Glob(filepath.Join(parent, "corvint-work-run-*"))
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(before, after) {
				t.Fatalf("construction leaked run root: before %v after %v", before, after)
			}
		})
	}
}

// WQO-V0-004/005: the standalone actual script owns producer scratch as well as
// compiler scratch and preserves source identity without an observer marker.
func TestWorkSelfAdapterStandaloneParity(t *testing.T) {
	t.Parallel()
	caller := workProductionFixture(t)
	source, err := worksource.Acquire(context.Background(), caller)
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	scratch := t.TempDir()
	environment, err := workChildEnvironment(t.TempDir(), scratch)
	if err != nil {
		t.Fatal(err)
	}
	before := materializationManifest(t, caller)
	// Parallel cold producer build: a hang detector, not a budget (decision 0082).
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, filepath.Join(caller, "script/corvint-work-queue"), "snapshot")
	command.Dir, command.Env = caller, environment
	command.WaitDelay = time.Second
	workContain(command)
	// Standalone script owns nested child groups: request its cleanup trap before
	// the independent context deadline's final group cleanup.
	command.Cancel = func() error {
		if command.Process != nil {
			return command.Process.Signal(os.Interrupt)
		}
		return nil
	}
	t.Cleanup(func() { workKillGroup(command) })
	raw, err := command.Output()
	workKillGroup(command)
	if err != nil {
		if exit, ok := err.(*exec.ExitError); ok {
			t.Fatalf("standalone actual script: %v: %s", err, exit.Stderr)
		}
		t.Fatal(err)
	}
	snapshot, err := workqueue.ParseSnapshot(raw)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.RepositorySource != source.Identity {
		t.Fatal("standalone producer source disagreement")
	}
	entries, err := os.ReadDir(scratch)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("standalone actual producer scratch remains: %v", entries)
	}
	if !reflect.DeepEqual(before, materializationManifest(t, caller)) {
		t.Fatal("standalone actual script changed caller")
	}
}

// WQO-V0-004/005/017: ambient TMPDIR and forged reuse tuples cannot put build
// artifacts in a caller tree, its Git directory, or a linked worktree's common Git.
func TestWorkScriptRejectsCallerScratch(t *testing.T) {
	t.Parallel()
	script, err := filepath.Abs(filepath.Join("..", "..", "script/corvint-work-queue"))
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"ambient-target", "forged-target", "forged-git", "forged-linked-common"} {
		t.Run(kind, func(t *testing.T) {
			primary := materializationFixture(t)
			caller := primary
			if kind == "forged-linked-common" {
				caller = filepath.Join(t.TempDir(), "linked")
				materializationGit(t, primary, "worktree", "add", "--detach", caller, "HEAD")
			}
			caller, err := filepath.EvalSymlinks(caller)
			if err != nil {
				t.Fatal(err)
			}
			common := filepath.Join(primary, ".git")
			temporary := filepath.Join(caller, "scratch")
			if kind == "forged-git" || kind == "forged-linked-common" {
				temporary = filepath.Join(common, "scratch")
			}
			if err := os.MkdirAll(temporary, 0700); err != nil {
				t.Fatal(err)
			}
			if kind != "ambient-target" {
				owner := filepath.Join(temporary, "corvint-work-queue-owner")
				if err := os.Mkdir(owner, 0700); err != nil {
					t.Fatal(err)
				}
				tuple := caller + "\n" + materializationGit(t, caller, "rev-parse", "HEAD") + "\n"
				if err := os.WriteFile(filepath.Join(owner, "tuple"), []byte(tuple), 0600); err != nil {
					t.Fatal(err)
				}
			}
			beforeCaller, beforeCommon := materializationManifest(t, caller), materializationManifest(t, common)
			fake := t.TempDir()
			witness := filepath.Join(fake, "build-storage")
			body := fmt.Sprintf("#!/bin/bash\nprintf '%%s\\n' \"$TMPDIR\" > %q\nexit 7\n", witness)
			if err := os.WriteFile(filepath.Join(fake, "go"), []byte(body), 0755); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute) // hang detector, not a budget (decision 0082)
			defer cancel()
			command := exec.CommandContext(ctx, script, "snapshot")
			command.Dir = caller
			command.Env = []string{"PATH=" + fake + ":/usr/bin:/bin:/usr/local/bin", "HOME=" + t.TempDir(), "TMPDIR=" + temporary}
			command.WaitDelay = time.Second
			workContain(command)
			t.Cleanup(func() { workKillGroup(command) })
			if err := command.Run(); err == nil {
				t.Fatal("poisoned scratch unexpectedly completed")
			}
			workKillGroup(command)
			afterCaller, afterCommon := materializationManifest(t, caller), materializationManifest(t, common)
			if !reflect.DeepEqual(beforeCaller, afterCaller) || !reflect.DeepEqual(beforeCommon, afterCommon) {
				t.Fatalf("script wrote caller or shared Git metadata:\ncaller %v -> %v\ncommon %v -> %v", beforeCaller, afterCaller, beforeCommon, afterCommon)
			}
			raw, err := os.ReadFile(witness)
			if kind != "ambient-target" {
				if !os.IsNotExist(err) {
					t.Fatal("forged shared marker launched compiler")
				}
				return
			}
			if err != nil {
				t.Fatal("standalone compiler did not run in safe scratch", err)
			}
			acquired := strings.TrimSpace(string(raw))
			fixed, err := filepath.EvalSymlinks("/tmp")
			if err != nil {
				t.Fatal(err)
			}
			if !strings.HasPrefix(acquired, fixed+"/corvint-work-queue.") {
				t.Fatalf("ambient scratch used: %s", acquired)
			}
			if _, err := os.Stat(filepath.Dir(acquired)); !os.IsNotExist(err) {
				t.Fatal("standalone build scratch remains")
			}
		})
	}
}

// WQO-V0-004/005: aliases of a protected directory retain its filesystem identity.
func TestWorkScriptRejectsScratchAliases(t *testing.T) {
	t.Parallel()
	script, err := filepath.Abs(filepath.Join("..", "..", "script/corvint-work-queue"))
	if err != nil {
		t.Fatal(err)
	}
	for _, names := range [][2]string{{"MixedCaseRepository", "MIXEDCASEREPOSITORY"}, {"caf\u00e9", "cafe\u0301"}} {
		t.Run(names[0], func(t *testing.T) {
			parent := t.TempDir()
			caller, alias := filepath.Join(parent, names[0]), filepath.Join(parent, names[1])
			if err := os.Mkdir(caller, 0700); err != nil {
				t.Fatal(err)
			}
			original, err := os.Stat(caller)
			if err != nil {
				t.Fatal(err)
			}
			equivalent, err := os.Stat(alias)
			if err != nil || !os.SameFile(original, equivalent) {
				t.Skip("filesystem does not alias these spellings")
			}
			materializationGit(t, caller, "init", "-q")
			materializationGit(t, caller, "commit", "--allow-empty", "-qm", "alias fixture")
			caller = materializationGit(t, caller, "rev-parse", "--show-toplevel")
			temporary := filepath.Join(alias, "scratch")
			owner := filepath.Join(temporary, "corvint-work-queue-owner")
			if err := os.MkdirAll(owner, 0700); err != nil {
				t.Fatal(err)
			}
			tuple := caller + "\n" + materializationGit(t, caller, "rev-parse", "HEAD") + "\n"
			if err := os.WriteFile(filepath.Join(owner, "tuple"), []byte(tuple), 0600); err != nil {
				t.Fatal(err)
			}
			before := materializationManifest(t, caller)
			fake := t.TempDir()
			marker := filepath.Join(fake, "compiled")
			body := fmt.Sprintf("#!/bin/bash\nprintf started > %q\nexit 7\n", marker)
			if err := os.WriteFile(filepath.Join(fake, "go"), []byte(body), 0755); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute) // hang detector, not a budget (decision 0082)
			defer cancel()
			command := exec.CommandContext(ctx, script, "snapshot")
			command.Dir = caller
			command.Env = []string{"PATH=" + fake + ":/usr/bin:/bin:/usr/local/bin", "HOME=" + t.TempDir(), "TMPDIR=" + temporary}
			command.WaitDelay = time.Second
			workContain(command)
			t.Cleanup(func() { workKillGroup(command) })
			if err := command.Run(); err == nil {
				t.Fatal("aliased marker accepted")
			}
			workKillGroup(command)
			if _, err := os.Stat(marker); !os.IsNotExist(err) {
				t.Fatal("aliased target scratch launched compiler")
			}
			if !reflect.DeepEqual(before, materializationManifest(t, caller)) {
				t.Fatal("aliased caller changed")
			}
		})
	}
}

// WQO-V0-004/005 and VPO-V0-022: script metadata preflight cannot inherit Git
// rebinding, PATH Git, or user/global configuration before scratch validation.
func TestWorkScriptPinsGitPreflight(t *testing.T) {
	t.Parallel()
	caller := materializationFixture(t)
	script, err := filepath.Abs(filepath.Join("..", "..", "script/corvint-work-queue"))
	if err != nil {
		t.Fatal(err)
	}
	fake := t.TempDir()
	gitMarker, goMarker := filepath.Join(fake, "git-started"), filepath.Join(fake, "compiler-storage")
	for name, body := range map[string]string{
		"git": fmt.Sprintf("#!/bin/bash\nprintf started > %q\nexit 99\n", gitMarker),
		"go":  fmt.Sprintf("#!/bin/bash\nprintf '%%s\\n' \"$TMPDIR\" > %q\nexit 7\n", goMarker),
	} {
		if err := os.WriteFile(filepath.Join(fake, name), []byte(body), 0755); err != nil {
			t.Fatal(err)
		}
	}
	home, scratch := t.TempDir(), t.TempDir()
	global := filepath.Join(home, ".gitconfig")
	if err := os.WriteFile(global, []byte("[core]\nworktree = "+fake+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	before := materializationManifest(t, caller)
	poisoned := []string{"GIT_DIR", "GIT_COMMON_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE", "GIT_OBJECT_DIRECTORY", "GIT_ALTERNATE_OBJECT_DIRECTORIES", "GIT_NAMESPACE", "GIT_SHALLOW_FILE", "GIT_REPLACE_REF_BASE", "GIT_CONFIG_COUNT", "GIT_CONFIG_PARAMETERS", ""}
	for _, name := range poisoned {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute) // hang detector, not a budget (decision 0082)
			defer cancel()
			command := exec.CommandContext(ctx, script, "snapshot")
			command.Dir = caller
			command.Env = []string{"PATH=" + fake + ":/usr/bin:/bin:/usr/local/bin", "HOME=" + home, "TMPDIR=" + scratch, "GIT_CONFIG_GLOBAL=" + global, "GIT_CONFIG_SYSTEM=" + global}
			if name != "" {
				command.Env = append(command.Env, name+"=")
			}
			command.WaitDelay = time.Second
			workContain(command)
			t.Cleanup(func() { workKillGroup(command) })
			if err := command.Run(); err == nil {
				t.Fatal("fixture unexpectedly completed")
			}
			workKillGroup(command)
			if _, err := os.Stat(gitMarker); !os.IsNotExist(err) {
				t.Fatal("ambient PATH Git executed")
			}
			if !reflect.DeepEqual(before, materializationManifest(t, caller)) {
				t.Fatal("preflight changed caller")
			}
			raw, err := os.ReadFile(goMarker)
			if name != "" {
				if !os.IsNotExist(err) {
					t.Fatal("rebound preflight reached compiler")
				}
				return
			}
			if err != nil {
				t.Fatal("fixed Git did not ignore poisoned global config", err)
			}
			if _, err := os.Stat(filepath.Dir(strings.TrimSpace(string(raw)))); !os.IsNotExist(err) {
				t.Fatal("compiler scratch remains")
			}
		})
	}
}
