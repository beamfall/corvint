package cli

import (
	"errors"
	"io/fs"
	"os"

	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/rootalias"
)

// missingRootStderr is the startup line that makes the MCPV0-115 substitution visible in the
// host's server log; like every corvint-mcp stderr line it names no path.
const missingRootStderr = "corvint-mcp: --root does not exist; serving the Git worktree enclosing the launch directory instead"

// missingRootRefusal is the MCPV0-115 refusal when no launch worktree can stand in.
const missingRootRefusal = "repository unavailable: --root does not exist and the launch directory is not in a Git worktree"

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
// passes every MCPV0-001 check when it is opened. It returns that worktree and true, or a refusal
// when no launch worktree exists; an ineligible argv returns neither and keeps its old outcome.
func missingRootFallback(roots []rootDeclaration, follows, corpus bool, getwd func() (string, error)) (root string, substituted bool, refusal string) {
	if len(roots) != 1 || roots[0].alias != "" || follows || corpus || !rootalias.ValidRoot(roots[0].root) {
		return "", false, ""
	}
	if _, err := os.Lstat(roots[0].root); !errors.Is(err, fs.ErrNotExist) {
		return "", false, ""
	}
	launch, err := getwd()
	if err != nil {
		return "", false, missingRootRefusal
	}
	top, ok := enclosingWorktree(launch)
	if !ok {
		return "", false, missingRootRefusal
	}
	return top, true, ""
}
