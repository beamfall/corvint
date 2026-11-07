package companionrelease

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
)

// The current-export tests run the unchanged exportSource against a disposable
// repository holding the working tree's real bytes of the paths they consume,
// never against the enclosing checkout's .git (AFP-V0-023: a declared package's
// tests read below the repository root only their own directory and declared
// entries, and never .git). currentSourcePaths names those paths: the Tasks
// subset exactly as the production lists define it, the attribute and ignore
// rules that decide what `git add -A` tracks, and extensions/vscode/, which
// lies outside the subset, so tasksExport still has real paths to drop and the
// VSIX proof finds its extension source.
func currentSourcePaths() []string {
	paths := []string{".gitattributes", ".gitignore", "extensions/vscode/"}
	for name := range tasksExportFiles {
		paths = append(paths, name)
	}
	paths = append(paths, tasksExportPrefixes...)
	sort.Strings(paths)
	return paths
}

var currentSource struct {
	once   sync.Once
	parent string
	root   string
	err    error
}

// currentSourceFixture returns the one fixture repository this test binary
// shares between the current-export tests, building it on first use.
func currentSourceFixture(t *testing.T) string {
	t.Helper()
	currentSource.once.Do(func() {
		currentSource.parent, currentSource.root, currentSource.err = buildCurrentSourceFixture()
	})
	if currentSource.err != nil {
		t.Fatalf("current source fixture: %v", currentSource.err)
	}
	return currentSource.root
}

func TestMain(m *testing.M) {
	code := m.Run()
	if currentSource.parent != "" {
		_ = os.RemoveAll(currentSource.parent)
	}
	os.Exit(code)
}

func buildCurrentSourceFixture() (parent, root string, err error) {
	repository, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		return "", "", err
	}
	parent, err = os.MkdirTemp("", "companionrelease-source-")
	if err != nil {
		return "", "", err
	}
	root = filepath.Join(parent, "repo")
	home := filepath.Join(parent, "home")
	if err := os.Mkdir(home, 0o700); err != nil {
		return parent, "", err
	}
	// Only the fixture's own repository and an empty HOME configure git: no
	// system or global config, no template (so no hooks), fixed identities.
	env := append(closedGitEnv(home),
		"GIT_AUTHOR_NAME=fixture", "GIT_AUTHOR_EMAIL=fixture@example.invalid", "GIT_AUTHOR_DATE=2026-01-01T00:00:00Z",
		"GIT_COMMITTER_NAME=fixture", "GIT_COMMITTER_EMAIL=fixture@example.invalid", "GIT_COMMITTER_DATE=2026-01-01T00:00:00Z")
	git := func(stdin []byte, args ...string) ([]byte, error) {
		command := exec.Command("/usr/bin/git", append([]string{"-c", "core.hooksPath=/dev/null", "-c", "commit.gpgsign=false"}, args...)...)
		command.Dir, command.Env = root, env
		command.Stdin = bytes.NewReader(stdin)
		var stdout, stderr bytes.Buffer
		command.Stdout, command.Stderr = &stdout, &stderr
		if err := command.Run(); err != nil {
			return stdout.Bytes(), fmt.Errorf("git %v: %w: %s", args, err, stderr.Bytes())
		}
		return stdout.Bytes(), nil
	}
	if err := os.Mkdir(root, 0o700); err != nil {
		return parent, "", err
	}
	if _, err := git(nil, "init", "-q", "--template=", "--initial-branch=main"); err != nil {
		return parent, "", err
	}
	for _, path := range currentSourcePaths() {
		relative := filepath.FromSlash(strings.TrimSuffix(path, "/"))
		if err := os.MkdirAll(filepath.Join(root, filepath.Dir(relative)), 0o755); err != nil {
			return parent, "", err
		}
		if !strings.HasSuffix(path, "/") {
			if err := copySourceEntry(repository, root, relative); err != nil {
				return parent, "", err
			}
			continue
		}
		if err := os.Mkdir(filepath.Join(root, relative), 0o755); err != nil {
			return parent, "", err
		}
		if err := copySourceTree(git, repository, root, relative); err != nil {
			return parent, "", err
		}
	}
	if _, err := git(nil, "add", "-A"); err != nil {
		return parent, "", err
	}
	if _, err := git(nil, "commit", "-q", "--no-verify", "-m", "current source fixture"); err != nil {
		return parent, "", err
	}
	return parent, root, nil
}

// copySourceTree copies directory relative from repository to root, leaving
// out what the repository's own ignore rules (already copied into root)
// ignore, so an installed node_modules or build output is never copied; `git
// add -A` applies the same rules to whatever is copied.
func copySourceTree(git func([]byte, ...string) ([]byte, error), repository, root, relative string) error {
	entries, err := os.ReadDir(filepath.Join(repository, relative))
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.Name() == ".gitignore" {
			if err := copySourceEntry(repository, root, filepath.Join(relative, entry.Name())); err != nil {
				return err
			}
		}
	}
	var names bytes.Buffer
	for _, entry := range entries {
		child := filepath.Join(relative, entry.Name())
		if entry.IsDir() {
			// A directory rule matches only a directory that exists.
			if err := os.Mkdir(filepath.Join(root, child), 0o755); err != nil {
				return err
			}
		}
		names.WriteString(filepath.ToSlash(child))
		names.WriteByte(0)
	}
	ignored := map[string]bool{}
	output, err := git(names.Bytes(), "check-ignore", "-z", "--stdin")
	var exit *exec.ExitError
	if err != nil && !(errors.As(err, &exit) && exit.ExitCode() == 1) {
		return err
	}
	for _, name := range strings.Split(string(output), "\x00") {
		if name != "" {
			ignored[name] = true
		}
	}
	for _, entry := range entries {
		child := filepath.Join(relative, entry.Name())
		switch {
		case ignored[filepath.ToSlash(child)]:
			if entry.IsDir() {
				if err := os.Remove(filepath.Join(root, child)); err != nil {
					return err
				}
			}
		case entry.IsDir():
			if err := copySourceTree(git, repository, root, child); err != nil {
				return err
			}
		case entry.Name() != ".gitignore":
			if err := copySourceEntry(repository, root, child); err != nil {
				return err
			}
		}
	}
	return nil
}

// copySourceEntry copies one regular file with its permission bits, or one
// symbolic link as a link, so exportSource sees what the checkout holds.
func copySourceEntry(repository, root, relative string) error {
	from, to := filepath.Join(repository, relative), filepath.Join(root, relative)
	info, err := os.Lstat(from)
	if err != nil {
		return err
	}
	switch {
	case info.Mode()&fs.ModeSymlink != 0:
		target, err := os.Readlink(from)
		if err != nil {
			return err
		}
		return os.Symlink(target, to)
	case !info.Mode().IsRegular():
		return fmt.Errorf("%s is neither a regular file nor a symbolic link", relative)
	}
	source, err := os.Open(from)
	if err != nil {
		return err
	}
	defer source.Close()
	target, err := os.OpenFile(to, os.O_WRONLY|os.O_CREATE|os.O_EXCL, info.Mode().Perm())
	if err != nil {
		return err
	}
	if _, err := io.Copy(target, source); err != nil {
		target.Close()
		return err
	}
	return target.Close()
}
