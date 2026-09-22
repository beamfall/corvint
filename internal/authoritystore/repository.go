package authoritystore

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
)

// repositoryBinding retains directory handles across evaluation. Neither event
// payload cwd nor a global enrollment chooses the invoking repository.
type repositoryBinding struct {
	repo          *gitauth.Repository
	cwd           string
	paths         []string
	handles       []*os.File
	configuration []repositoryMetadata
}

func bindCurrentRepository(ctx context.Context, root RootDocument) (*repositoryBinding, error) {
	binding := &repositoryBinding{}
	success := false
	defer func() {
		if !success {
			binding.close()
		}
	}()
	actual, err := os.Open(".")
	if err != nil {
		return nil, errUnavailable
	}
	binding.handles = append(binding.handles, actual)
	cwd, err := os.Getwd()
	if err != nil {
		return nil, errUnavailable
	}
	cwd, err = filepath.EvalSymlinks(cwd)
	if err != nil {
		return nil, errUnavailable
	}
	binding.cwd = cwd
	binding.paths = append(binding.paths, cwd)
	// A local core.worktree value must not redirect discovery across repositories.
	// Require the nearest physical .git boundary and Git's answer to agree.
	discovered := cwd
	for depth := 0; ; depth++ {
		if depth > 256 {
			return nil, errUnavailable
		}
		if _, err = os.Lstat(filepath.Join(discovered, ".git")); err == nil {
			break
		} else if !os.IsNotExist(err) {
			return nil, errUnavailable
		}
		parent := filepath.Dir(discovered)
		if parent == discovered {
			return nil, errUnavailable
		}
		discovered = parent
	}
	if discovered != root.RepositoryRoot {
		return nil, errUnavailable
	}
	binding.repo, err = gitauth.Open(discovered, gitrun.NewDefaultBudget())
	if err != nil || binding.repo.Root != root.RepositoryRoot {
		return nil, errUnavailable
	}
	for _, path := range []string{binding.repo.Root, binding.repo.GitDir, binding.repo.CommonDir} {
		handle, err := openBindingDirectory(path)
		if err != nil {
			return nil, errUnavailable
		}
		binding.paths = append(binding.paths, path)
		binding.handles = append(binding.handles, handle)
	}
	if err = binding.captureConfiguration(); err != nil {
		return nil, err
	}
	if err = binding.unchanged(ctx, root); err != nil {
		return nil, errUnavailable
	}
	success = true
	return binding, nil
}

func (b *repositoryBinding) close() {
	for _, handle := range b.handles {
		handle.Close()
	}
}

func (b *repositoryBinding) unchanged(ctx context.Context, root RootDocument) error {
	if ctx.Err() != nil {
		return errUnavailable
	}
	actual, err := os.Stat(".")
	if err != nil {
		return errUnavailable
	}
	held, err := b.handles[0].Stat()
	if err != nil || !os.SameFile(actual, held) {
		return errUnavailable
	}
	cwd, err := os.Getwd()
	if err != nil {
		return errUnavailable
	}
	cwd, err = filepath.EvalSymlinks(cwd)
	if err != nil || cwd != b.cwd {
		return errUnavailable
	}
	for index, path := range b.paths {
		current, err := os.Stat(path)
		if err != nil {
			return errUnavailable
		}
		held, err := b.handles[index].Stat()
		if err != nil || !current.IsDir() || !os.SameFile(current, held) {
			return errUnavailable
		}
	}
	// Revalidate reciprocal Git metadata, including a replaced linked-worktree
	// marker. Held directory handles prevent a renamed replacement from matching.
	repo, err := gitauth.Open(root.RepositoryRoot, gitrun.NewDefaultBudget())
	if err != nil || repo.Root != b.repo.Root || repo.GitDir != b.repo.GitDir || repo.CommonDir != b.repo.CommonDir {
		return errUnavailable
	}
	if err = b.configurationUnchanged(); err != nil {
		return err
	}
	if b.repo.ObjectFormat != "" {
		if err = repo.LoadObjectFormat(ctx); err != nil || repo.ObjectFormat != b.repo.ObjectFormat {
			return errUnavailable
		}
	}
	// A nested repository introduced during evaluation must not inherit admission.
	for path := cwd; path != root.RepositoryRoot; path = filepath.Dir(path) {
		if path == filepath.Dir(path) {
			return errUnavailable
		}
		if _, err := os.Lstat(filepath.Join(path, ".git")); !os.IsNotExist(err) {
			return errUnavailable
		}
	}
	return verifyGitRoot(ctx, root, cwd)
}

// Re-discover through the already verified protected Git image with no caller
// Git/PWD environment. Local worktree redirection is rejected, including drift.
func verifyGitRoot(ctx context.Context, root RootDocument, cwd string) error {
	output, err := gitrun.Run(ctx, gitrun.NewBudget(1, time.Second), gitrun.Options{
		Binary: root.Git.Path, Dir: cwd, StdoutLimit: 4096, PerOpTimeout: time.Second,
		Env: []string{"LANG=C", "LC_ALL=C", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=" + os.DevNull, "GIT_CONFIG_SYSTEM=" + os.DevNull, "GIT_OPTIONAL_LOCKS=0", "GIT_TERMINAL_PROMPT=0", "GIT_NO_LAZY_FETCH=1", "GIT_NO_REPLACE_OBJECTS=1"},
	}, "--no-optional-locks", "-c", "core.fsmonitor=false", "-c", "core.hooksPath="+os.DevNull, "rev-parse", "--show-toplevel")
	if err != nil {
		return errUnavailable
	}
	gitRoot := strings.TrimSuffix(string(output), "\n")
	if gitRoot != root.RepositoryRoot {
		return errUnavailable
	}
	return nil
}
