package postmergeconnector

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/procgroup"
)

var oidPattern = regexp.MustCompile(`^[0-9a-f]{40}$`)
var idPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)
var pathPattern = regexp.MustCompile(`^[A-Za-z0-9._/-]{1,512}$`)

func validPath(p string) bool {
	if !pathPattern.MatchString(p) || strings.HasPrefix(p, "/") {
		return false
	}
	for _, part := range strings.Split(p, "/") {
		if part == "" || part == "." || part == ".." {
			return false
		}
	}
	return true
}
func gitBytes(ctx context.Context, root string, limit int, args ...string) ([]byte, error) {
	git, err := exec.LookPath("git")
	if err != nil {
		return nil, fmt.Errorf("git-executable-unavailable")
	}
	env := []string{"PATH=" + os.Getenv("PATH"), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null", "GIT_CONFIG_NOSYSTEM=1", "GIT_TERMINAL_PROMPT=0", "GIT_OPTIONAL_LOCKS=0", "GIT_NO_LAZY_FETCH=1", "GIT_NO_REPLACE_OBJECTS=1", "GIT_ALLOW_PROTOCOL=", "LANG=C", "LC_ALL=C"}
	o := procgroup.Run(ctx, procgroup.Spec{Argv: append([]string{git, "-c", "credential.helper="}, args...), Dir: root, Env: env, Timeout: 10 * time.Second, ShutdownTimeout: time.Second, OutputLimit: limit})
	if o.ExitStatus != 0 || o.Err != nil || o.OutputOverflow || o.TimedOut || o.Cancelled {
		return nil, fmt.Errorf("git-observation-unavailable")
	}
	return o.Stdout, nil
}

// ProtectedGitPaths includes detached worktree metadata outside the checkout.
// Mutable connector outputs must be excluded from each returned directory.
func ProtectedGitPaths(ctx context.Context, root string) ([]string, error) {
	if !filepath.IsAbs(root) || filepath.Clean(root) != root {
		return nil, fmt.Errorf("product-root-must-be-clean-absolute-directory")
	}
	paths := []string{}
	for _, flag := range []string{"--show-toplevel", "--absolute-git-dir", "--git-common-dir"} {
		b, err := gitBytes(ctx, root, 4096, "rev-parse", "--path-format=absolute", flag)
		if err != nil {
			return nil, err
		}
		paths = append(paths, strings.TrimSuffix(string(b), "\n"))
	}
	for i, path := range paths {
		resolved, err := filepath.EvalSymlinks(path)
		if err != nil || !filepath.IsAbs(resolved) {
			return nil, fmt.Errorf("protected-git-directory-unavailable")
		}
		info, err := os.Stat(resolved)
		if err != nil || !info.IsDir() {
			return nil, fmt.Errorf("protected-git-directory-unavailable")
		}
		paths[i] = resolved
	}
	return paths, nil
}
func verifyBinding(ctx context.Context, root string, b Binding) error {
	if !idPattern.MatchString(b.Forge) || !idPattern.MatchString(b.Repository) || !idPattern.MatchString(b.Change) || !oidPattern.MatchString(b.Base) || !oidPattern.MatchString(b.Merge) || b.Base == b.Merge {
		return fmt.Errorf("invalid-binding")
	}
	for _, id := range []string{b.Base, b.Merge} {
		out, err := gitBytes(ctx, root, 256, "rev-parse", "--verify", id+"^{commit}")
		if err != nil || strings.TrimSpace(string(out)) != id {
			return fmt.Errorf("commit-unavailable")
		}
	}
	_, err := gitBytes(ctx, root, 256, "merge-base", "--is-ancestor", b.Base, b.Merge)
	if err != nil {
		return fmt.Errorf("base-not-ancestor")
	}
	return nil
}
func ChangedFiles(ctx context.Context, root string, b Binding) ([]ChangedFile, error) {
	if err := verifyBinding(ctx, root, b); err != nil {
		return nil, err
	}
	out, err := gitBytes(ctx, root, MaxBytes, "diff", "--no-renames", "--name-status", "-z", b.Base, b.Merge, "--")
	if err != nil {
		return nil, err
	}
	fields := bytes.Split(out, []byte{0})
	fields = fields[:len(fields)-1]
	if len(fields)%2 != 0 {
		return nil, fmt.Errorf("changed-files-invalid")
	}
	files := []ChangedFile{}
	for i := 0; i < len(fields); i += 2 {
		p, status := string(fields[i+1]), string(fields[i])
		if !validPath(p) || !slices.Contains([]string{"A", "M", "D", "T"}, status) {
			return nil, fmt.Errorf("changed-path-unsupported")
		}
		files = append(files, ChangedFile{p, status})
	}
	slices.SortFunc(files, func(a, b ChangedFile) int { return strings.Compare(a.Path, b.Path) })
	return files, nil
}
func lineExists(ctx context.Context, root, merge, path string, line int) bool {
	if !validPath(path) || line < 1 {
		return false
	}
	out, err := gitBytes(ctx, root, 2048, "ls-tree", merge, "--", path)
	if err != nil || !(strings.HasPrefix(string(out), "100644 blob ") || strings.HasPrefix(string(out), "100755 blob ")) {
		return false
	}
	body, err := gitBytes(ctx, root, (1<<20)+1, "show", merge+":"+path)
	if err != nil || len(body) > 1<<20 || len(body) == 0 || bytes.Contains(body, []byte{0}) {
		return false
	}
	n := bytes.Count(body, []byte{'\n'})
	if body[len(body)-1] != '\n' {
		n++
	}
	return line <= n
}
