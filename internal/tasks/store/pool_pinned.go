package store

import (
	"context"
	"errors"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// poolCommandSecond is the unit of a health/cleanup timeoutSeconds bound; only
// package tests shorten it, to enforce a long bound without waiting it out.
var poolCommandSecond = time.Second

// poolCommandDir returns the directory a pool command runs in (PSR-V0-012).
// REPOSITORY keeps the queue checkout root. A pinned external worktree is used
// only when its path is not an alias, is a Git worktree top level, its HEAD is
// exactly the pinned revision and its inputs are clean under the same porcelain
// rule as REPOSITORY; otherwise it refuses with a named code and the caller runs
// nothing. Every Git observation is bounded by sweepGit.
func poolCommandDir(ctx context.Context, def *intent.PoolCommand, root string) (string, error) {
	if def == nil || def.Pinned == nil {
		return root, nil
	}
	pin := def.Pinned
	real, e := filepath.EvalSymlinks(pin.Path)
	if e != nil || real != pin.Path {
		return "", wire.Errorf(wire.CodeMissingEvidence, "cwd", "pinned path is absent or an alias")
	}
	if info, e := os.Stat(pin.Path); e != nil || !info.IsDir() {
		return "", wire.Errorf(wire.CodeMissingEvidence, "cwd", "pinned path is not a directory")
	}
	git := func(args ...string) (string, error) {
		raw, e := sweepGit(ctx, pin.Path, args...)
		var bounded sweepGitBounded
		if e != nil && !errors.As(e, &bounded) {
			e = wire.Errorf(wire.CodeMissingEvidence, "cwd", "pinned path is not a readable Git worktree")
		}
		return string(raw), e
	}
	top, e := git("rev-parse", "--show-toplevel")
	if e != nil {
		return "", e
	}
	if strings.TrimSpace(top) != pin.Path {
		return "", wire.Errorf(wire.CodeMissingEvidence, "cwd", "pinned path is not a worktree top level")
	}
	head, e := git("rev-parse", "--verify", "--end-of-options", "HEAD^{commit}")
	if e != nil {
		return "", e
	}
	if strings.TrimSpace(head) != pin.Revision {
		return "", wire.Errorf(wire.CodeStaleTree, "cwd", "pinned worktree HEAD differs from the pinned revision")
	}
	status, e := git("status", "--porcelain", "--untracked-files=all", "--", ".", ":(exclude).taskman")
	if e != nil {
		return "", e
	}
	if status != "" {
		return "", wire.Errorf(wire.CodeDirtyWorktree, "cwd", "pinned worktree inputs must be clean")
	}
	return pin.Path, nil
}

// poolMemberCommand returns the configured health or cleanup command for a
// queue-unique member, or nil.
func poolMemberCommand(p *intent.Policy, member, kind string) *intent.PoolCommand {
	for _, pool := range p.Pools {
		config, ok := pool.MemberConfig[member]
		if !ok {
			continue
		}
		if kind == "health" {
			return config.Health
		}
		return config.Cleanup
	}
	return nil
}
