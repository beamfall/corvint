package cli

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/mcp/rootbind"
	"github.com/Beamfall/corvint/internal/rootalias"
)

// rootSubstitution is how a substituted binding says so: one startup line in the host's server log
// and the closed notice every tool result carries. Like every corvint-mcp stderr line, neither
// names a path.
type rootSubstitution struct{ stderr, notice string }

// missingRootSubstitution is the MCPV0-115 substitution for a --root that does not exist, and
// nonWorktreeRootSubstitution the MCPV0-861 one for an existing --root outside any Git worktree.
var (
	missingRootSubstitution = rootSubstitution{
		stderr: "corvint-mcp: --root does not exist; serving the Git worktree enclosing the launch directory instead",
		notice: substitutionNotice("missing-root-launch-worktree",
			"The --root this server was started with does not exist, so it serves the Git worktree enclosing its launch directory instead. "+
				"Results describe that worktree. If it is not the intended repository, restart the server with an existing --root."),
	}
	nonWorktreeRootSubstitution = rootSubstitution{
		stderr: "corvint-mcp: --root is not inside a Git worktree; serving the Git worktree enclosing the launch directory instead",
		notice: substitutionNotice("non-worktree-root-launch-worktree",
			"The --root this server was started with is not inside a Git worktree, so it serves the Git worktree enclosing its launch directory instead. "+
				"Results describe that worktree. If it is not the intended repository, restart the server with a --root inside it."),
	}
)

// substitutionNotice renders a closed, path-free notice in the MMR-V0-009 notice schema.
func substitutionNotice(code, message string) string {
	raw, err := gokernel.CanonicalJSON(map[string]any{
		"schema": worktreeNoticeSchema, "code": code, "mutates": false, "message": message,
	})
	if err != nil {
		panic(err)
	}
	return string(raw)
}

// missingRootFallback applies MCPV0-115 (V1-3497) and MCPV0-861 (V1-3894). Only one plain,
// well-formed --root without --follow-ref or --corpus-artifact is eligible, and only when its
// MCPV0-950 reason is that it does not exist (and lstat agrees) or that it is an existing
// directory outside any Git worktree. It is then replaced by the Git worktree enclosing the launch
// directory, which is observed once and still passes every MCPV0-001 check when it is opened. It
// returns that worktree, the substitution to announce, and true; an ineligible argv, or one with
// no observable launch worktree, returns false and keeps the root's MCPV0-950 outcome (served
// unavailable with its reason).
func missingRootFallback(roots []rootDeclaration, reason string, follows, corpus bool, getwd func() (string, error)) (root string, substitution rootSubstitution, substituted bool) {
	if len(roots) != 1 || roots[0].alias != "" || follows || corpus || !rootalias.ValidRoot(roots[0].root) {
		return "", rootSubstitution{}, false
	}
	switch reason {
	case rootbind.ReasonMissing:
		if _, err := os.Lstat(roots[0].root); !errors.Is(err, fs.ErrNotExist) {
			return "", rootSubstitution{}, false
		}
		substitution = missingRootSubstitution
	case rootbind.ReasonNoWorktree:
		substitution = nonWorktreeRootSubstitution
	default:
		return "", rootSubstitution{}, false
	}
	launch, err := getwd()
	if err != nil {
		return "", rootSubstitution{}, false
	}
	root, substituted = launchWorktreeCandidate(launch)
	if !substituted {
		return "", rootSubstitution{}, false
	}
	return root, substitution, true
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
