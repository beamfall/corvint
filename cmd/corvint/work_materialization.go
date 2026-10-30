package main

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/execfile"
	"github.com/Beamfall/corvint/internal/worksource"
)

// workMaterialization owns a single observation's target and scratch storage.
// Its Git metadata is private derived execution state, outside the tree identity.
// Directory permissions provide isolation, not an enforced read-only sandbox.
type workMaterialization struct {
	root, target, home, tmp string
	environment             []string
	source                  *worksource.Source
	gitManifest             string
}

// workRunParentKey carries the directory that holds run roots. Only tests set it, so a
// test can inspect the run roots it allocated without globbing the host's shared /tmp.
type workRunParentKey struct{}

// workRunParent is the fixed /tmp or the checked CORVINT_WORK_RUN_PARENT override, never
// ambient TMPDIR; an unsafe override is refused (WQO-V0-094).
func workRunParent(ctx context.Context) (string, error) {
	if parent, ok := ctx.Value(workRunParentKey{}).(string); ok {
		return parent, nil
	}
	return worksource.ScratchParent()
}

func newWorkMaterialization(ctx context.Context, source *worksource.Source) (_ *workMaterialization, err error) {
	parent, err := workRunParent(ctx)
	if err != nil {
		return nil, err
	}
	root, err := os.MkdirTemp(parent, "corvint-work-run-")
	if err != nil {
		return nil, err
	}
	canonical, resolveErr := filepath.EvalSymlinks(root)
	if resolveErr != nil {
		_ = os.RemoveAll(root)
		return nil, resolveErr
	}
	root = canonical
	run := &workMaterialization{root: root, target: filepath.Join(root, "target"), home: filepath.Join(root, "home"), tmp: filepath.Join(root, "tmp"), source: source}
	defer func() {
		if err != nil {
			_ = run.Close()
		}
	}()
	for _, path := range []string{run.target, run.home, run.tmp} {
		if err = os.Mkdir(path, 0700); err != nil {
			return nil, err
		}
	}
	run.environment, err = workChildEnvironment(run.home, run.tmp)
	if err != nil {
		return nil, err
	}
	if err = run.writeEntries(); err != nil {
		return nil, err
	}
	if _, err = run.git(ctx, "init", "--quiet", "--template=", "--object-format="+source.Identity.ObjectFormat, "."); err != nil {
		return nil, err
	}
	if err = source.ExportObjects(ctx, filepath.Join(run.target, ".git")); err != nil {
		return nil, err
	}
	if _, err = run.git(ctx, "update-ref", "--no-deref", "HEAD", source.Identity.Commit); err != nil {
		return nil, err
	}
	if _, err = run.git(ctx, "read-tree", source.Identity.Tree); err != nil {
		return nil, err
	}
	budget, entries := int64(workManifestBytes), 0
	run.gitManifest, err = workManifestRoot(ctx, filepath.Join(run.target, ".git"), &budget, &entries)
	if err != nil {
		return nil, err
	}
	// This marker is outside the target and permits only this tuple's build reuse.
	owner := filepath.Join(run.tmp, "corvint-work-queue-owner")
	if err = os.Mkdir(owner, 0700); err != nil {
		return nil, err
	}
	if err = os.WriteFile(filepath.Join(owner, "tuple"), []byte(run.target+"\n"+source.Identity.Commit+"\n"), 0600); err != nil {
		return nil, err
	}
	if err = run.Verify(ctx); err != nil {
		return nil, err
	}
	return run, nil
}

func workChildEnvironment(home, tmp string) ([]string, error) {
	path, err := worksource.PlatformPath()
	if err != nil {
		return nil, err
	}
	environment := []string{"PATH=" + path, "LANG=C", "LC_ALL=C", "TZ=UTC", "NO_COLOR=1", "GIT_CONFIG_NOSYSTEM=1", "GIT_TERMINAL_PROMPT=0", "GIT_OPTIONAL_LOCKS=0", "HOME=" + home, "TMPDIR=" + tmp, "GOTOOLCHAIN=local"}
	// The adapter's per-user compiler cache must outlive the run-private HOME, or every
	// capture compiles the producer closure cold (WQO-V0-096, V1-3063). The script still
	// applies its own ownership, mode and outside-repository checks to this root.
	if cache, err := workUserCacheDir(); err == nil && filepath.IsAbs(cache) && !strings.ContainsAny(cache, "\x00\n") {
		environment = append(environment, "XDG_CACHE_HOME="+filepath.Clean(cache))
	}
	return environment, nil
}

// workUserCacheDir names the invoking user's cache directory for the adapter's compiler
// cache; tests point it at a package-run directory.
var workUserCacheDir = os.UserCacheDir

// Close retires the Git groups a killed producer left running in its
// acquisition scratch, then removes the run root. When retirement is not
// confirmed the run root, with the records needed to recover, is kept and the
// error names it (WQO-V0-052).
func (run *workMaterialization) Close() error {
	if err := worksource.RetireGitGroups(run.tmp, workGitRetirementBound); err != nil {
		return fmt.Errorf("work run scratch %s kept: %w", run.root, err)
	}
	return os.RemoveAll(run.root)
}

// workGitRetirementBound is a hang detector for Git groups exiting after SIGKILL, not a budget;
// tests shorten it.
var workGitRetirementBound = time.Minute

func (run *workMaterialization) git(ctx context.Context, args ...string) ([]byte, error) {
	private := &worksource.Source{Root: run.target, GitPath: run.source.GitPath, GitEnvironment: append([]string(nil), run.environment...)}
	private.GitEnvironment = append(private.GitEnvironment, "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null", "GIT_NO_LAZY_FETCH=1", "GIT_NO_REPLACE_OBJECTS=1")
	return private.Git(ctx, 1<<20, args...)
}

func (run *workMaterialization) writeEntries() error {
	root, err := os.OpenRoot(run.target)
	if err != nil {
		return err
	}
	defer root.Close()
	// Qualification rejects symlink parent components; create links only after files.
	for _, entry := range run.source.Entries {
		if entry.Mode == "120000" {
			continue
		}
		if err := workMakeParents(root, entry.Path); err != nil {
			return err
		}
		mode := fs.FileMode(0644)
		if entry.Mode == "100755" {
			mode = 0755
		}
		write := func() error { return workWriteEntry(root, entry.Path, entry.Raw, mode) }
		if mode == 0755 {
			// The runner execs materialized executables from this process while
			// other goroutines fork; a fork between open and close would inherit
			// the write descriptor and fail that start with ETXTBSY (WQO-V0-093).
			err = execfile.Guard(write)
		} else {
			err = write()
		}
		if err != nil {
			return err
		}
	}
	for _, entry := range run.source.Entries {
		if entry.Mode != "120000" {
			continue
		}
		if err := workMakeParents(root, entry.Path); err != nil {
			return err
		}
		if err := root.Symlink(string(entry.Raw), entry.Path); err != nil {
			return err
		}
	}
	return nil
}

// workWriteEntry creates path under root exclusively and writes raw to it. The
// descriptor is closed before it returns, so a caller may hold the fork lock.
func workWriteEntry(root *os.Root, path string, raw []byte, mode fs.FileMode) error {
	file, err := root.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	_, writeErr := file.Write(raw)
	closeErr := file.Close()
	if writeErr != nil {
		return writeErr
	}
	return closeErr
}

func workMakeParents(root *os.Root, path string) error {
	parent := filepath.Dir(path)
	if parent == "." {
		return nil
	}
	current := ""
	for _, part := range strings.Split(parent, string(filepath.Separator)) {
		current = filepath.Join(current, part)
		if err := root.Mkdir(current, 0755); err != nil && !os.IsExist(err) {
			return err
		}
		info, err := root.Lstat(current)
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return fmt.Errorf("non-directory materialization parent")
		}
	}
	return nil
}

func (run *workMaterialization) Verify(ctx context.Context) error {
	if err := run.source.VerifyMaterialization(ctx, run.target); err != nil {
		return err
	}
	root, err := os.OpenRoot(run.target)
	if err != nil {
		return err
	}
	defer root.Close()
	expected := make(map[string]worksource.Entry, len(run.source.Entries))
	directories := map[string]bool{".": true}
	for _, entry := range run.source.Entries {
		expected[entry.Path] = entry
		for parent := filepath.Dir(entry.Path); parent != "."; parent = filepath.Dir(parent) {
			directories[parent] = true
		}
	}
	err = fs.WalkDir(root.FS(), ".", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == ".git" {
			return fs.SkipDir
		}
		if entry.IsDir() {
			if !directories[path] {
				return fmt.Errorf("unexpected materialized directory %q", path)
			}
			return nil
		}
		if _, ok := expected[path]; !ok {
			return fmt.Errorf("unexpected materialized entry %q", path)
		}
		delete(expected, path)
		return nil
	})
	if err != nil {
		return err
	}
	if len(expected) != 0 {
		return fmt.Errorf("missing materialized entries")
	}
	budget, entries := int64(workManifestBytes), 0
	metadata, err := workManifestRoot(ctx, filepath.Join(run.target, ".git"), &budget, &entries)
	if err != nil {
		return err
	}
	if metadata != run.gitManifest {
		return fmt.Errorf("private Git metadata changed")
	}
	identity, err := run.git(ctx, "rev-parse", "HEAD", "HEAD^{tree}")
	if err != nil {
		return err
	}
	if string(identity) != run.source.Identity.Commit+"\n"+run.source.Identity.Tree+"\n" {
		return fmt.Errorf("private Git target changed")
	}
	status, err := run.git(ctx, "status", "--porcelain=v2", "--untracked-files=all")
	if err != nil {
		return err
	}
	if len(status) != 0 {
		return fmt.Errorf("private Git materialization changed")
	}
	return nil
}
