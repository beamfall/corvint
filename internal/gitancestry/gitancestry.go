// Package gitancestry holds the one guard every Git ancestry decision shares: Git honours an
// info/grafts file even with replace objects disabled, and a shallow checkout rewrites parents the
// same way, so merge-base, rev-list and --is-ancestor answers cannot be trusted in either case
// (AFU-V1-068, EEP-V0-028).
package gitancestry

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Runner runs one read-only Git command in the checkout and returns its standard output.
type Runner func(ctx context.Context, args ...string) ([]byte, error)

// Rewritten names why the checkout's commit ancestry cannot be trusted, or returns "". An
// unreadable answer is never trusted.
func Rewritten(ctx context.Context, git Runner) string {
	common, err := git(ctx, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return "checkout Git directory unreadable"
	}
	if _, err := os.Lstat(filepath.Join(strings.TrimSpace(string(common)), "info", "grafts")); !errors.Is(err, fs.ErrNotExist) {
		return "checkout rewrites ancestry with info/grafts"
	}
	shallow, err := git(ctx, "rev-parse", "--is-shallow-repository")
	if err != nil || strings.TrimSpace(string(shallow)) != "false" {
		return "checkout is shallow"
	}
	return ""
}

// RewrittenAt is Rewritten for a caller that already holds the checkout's resolved common Git
// directory and scrubs GIT_GRAFT_FILE and GIT_SHALLOW_FILE from its Git environment: it reads the
// same info/grafts and shallow files without spawning Git. Any entry at either path, or an
// unreadable one, is never trusted.
func RewrittenAt(commonDir string) string {
	if commonDir == "" {
		return "checkout Git directory unreadable"
	}
	if _, err := os.Lstat(filepath.Join(commonDir, "info", "grafts")); !errors.Is(err, fs.ErrNotExist) {
		return "checkout rewrites ancestry with info/grafts"
	}
	if _, err := os.Lstat(filepath.Join(commonDir, "shallow")); !errors.Is(err, fs.ErrNotExist) {
		return "checkout is shallow"
	}
	return ""
}
