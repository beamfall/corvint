package cli

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/rootalias"
)

// missingRootStderr is the startup line that makes the MCPV0-115 substitution visible in the
// host's server log; like every corvint-mcp stderr line it names no path.
const missingRootStderr = "corvint-mcp: --root does not exist; serving the Git worktree enclosing the launch directory instead"

// missingRootNoticeText is the closed, path-free notice every tool result of a substituted
// binding carries (MCPV0-115), in the MMR-V0-009 notice schema.
var missingRootNoticeText = func() string {
	raw, err := gokernel.CanonicalJSON(map[string]any{
		"schema": worktreeNoticeSchema, "code": "missing-root-launch-worktree", "mutates": false,
		"message": "The --root this server was started with does not exist, so it serves the Git worktree enclosing its launch directory instead. " +
			"Results describe that worktree. If it is not the intended repository, restart the server with an existing --root.",
	})
	if err != nil {
		panic(err)
	}
	return string(raw)
}()

// missingRootFallback applies MCPV0-115 (V1-3497). Only one plain, well-formed --root naming a
// path that does not exist, without --follow-ref or --corpus-artifact, is eligible; it is then
// replaced by the Git worktree enclosing the launch directory, which is observed once and still
// passes every MCPV0-001 check when it is opened. It returns that worktree and true; an ineligible
// argv, or one with no observable launch worktree, returns false and keeps the missing root's
// MCPV0-950 outcome (served unavailable, reason `--root does not exist`).
func missingRootFallback(roots []rootDeclaration, follows, corpus bool, getwd func() (string, error)) (root string, substituted bool) {
	if len(roots) != 1 || roots[0].alias != "" || follows || corpus || !rootalias.ValidRoot(roots[0].root) {
		return "", false
	}
	if _, err := os.Lstat(roots[0].root); !errors.Is(err, fs.ErrNotExist) {
		return "", false
	}
	launch, err := getwd()
	if err != nil {
		return "", false
	}
	return launchWorktreeCandidate(launch)
}

// launchWorktreeCandidate returns the nearest ancestor of directory, itself included, holding a
// .git entry of any kind, or the first ancestor whose .git lookup fails other than not-exist. It
// never skips such a marker to reach an outer repository: the caller opens the candidate with the
// MCPV0-001 checks, so a symlink marker or an unreadable entry is refused there as it would be for
// a direct --root.
func launchWorktreeCandidate(directory string) (string, bool) {
	resolved, err := filepath.EvalSymlinks(directory)
	if err != nil || !filepath.IsAbs(resolved) {
		return "", false
	}
	for current := resolved; ; {
		if _, err := os.Lstat(filepath.Join(current, ".git")); !errors.Is(err, fs.ErrNotExist) {
			return current, true
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", false
		}
		current = parent
	}
}
