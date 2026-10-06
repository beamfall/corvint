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

// poolPinStep, when set by a package test, runs after each pinned-cwd proof
// step and may fail it; it is nil in production.
var poolPinStep func(step string) error

// poolCommandDir returns the directory a pool command runs in (PSR-V0-012) and
// a launch guard that re-proves its identity immediately before process start.
// REPOSITORY keeps the queue checkout root and a nil guard. A pinned external
// worktree is used only when its path is a real directory (Lstat, no alias), a
// Git worktree top level, HEAD is exactly the pinned revision both before and
// after the clean-input status check, its inputs are clean under the same
// porcelain rule as REPOSITORY, and its directory identity is unchanged across
// the whole proof; otherwise it refuses with a named code and the caller runs
// nothing. Every Git observation is bounded by sweepGit. The proof is a
// sequence of non-atomic observations of a checkout Tasks does not own:
// concurrent mutation during or after them (in-place edits that keep HEAD and
// the directory identity, change-and-restore between reads, edits while the
// command runs) is outside the guarantee (PSR-V0-012 accepted bound).
func poolCommandDir(ctx context.Context, def *intent.PoolCommand, root string) (string, func() error, error) {
	if def == nil || def.Pinned == nil {
		return root, nil, nil
	}
	pin := def.Pinned
	step := func(name string) error {
		if poolPinStep == nil {
			return nil
		}
		return poolPinStep(name)
	}
	identity, e := pinnedDirectory(pin.Path)
	if e != nil {
		return "", nil, e
	}
	guard := func() error {
		now, e := pinnedDirectory(pin.Path)
		if e != nil || !os.SameFile(identity, now) {
			return wire.Errorf(wire.CodeStaleTree, "cwd", "pinned directory identity changed")
		}
		return nil
	}
	if e = step("identity"); e != nil {
		return "", nil, e
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
		return "", nil, e
	}
	if strings.TrimSpace(top) != pin.Path {
		return "", nil, wire.Errorf(wire.CodeMissingEvidence, "cwd", "pinned path is not a worktree top level")
	}
	head := func(name string) error {
		raw, e := git("rev-parse", "--verify", "--end-of-options", "HEAD^{commit}")
		if e != nil {
			return e
		}
		if strings.TrimSpace(raw) != pin.Revision {
			return wire.Errorf(wire.CodeStaleTree, "cwd", "pinned worktree HEAD differs from the pinned revision")
		}
		return step(name)
	}
	if e = head("head"); e != nil {
		return "", nil, e
	}
	status, e := git("status", "--porcelain", "--untracked-files=all", "--", ".", ":(exclude).taskman")
	if e != nil {
		return "", nil, e
	}
	if status != "" {
		return "", nil, wire.Errorf(wire.CodeDirtyWorktree, "cwd", "pinned worktree inputs must be clean")
	}
	if e = step("status"); e != nil {
		return "", nil, e
	}
	// A second HEAD read narrows, but cannot exclude, a revision change around
	// the status read.
	if e = head("head-again"); e != nil {
		return "", nil, e
	}
	if e = guard(); e != nil {
		return "", nil, e
	}
	if e = step("validated"); e != nil {
		return "", nil, e
	}
	return pin.Path, guard, nil
}

// pinnedDirectory is the Lstat identity of a real, non-alias pinned directory.
func pinnedDirectory(path string) (os.FileInfo, error) {
	info, e := os.Lstat(path)
	if e != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return nil, wire.Errorf(wire.CodeMissingEvidence, "cwd", "pinned path is absent, an alias or not a directory")
	}
	if real, e := filepath.EvalSymlinks(path); e != nil || real != path {
		return nil, wire.Errorf(wire.CodeMissingEvidence, "cwd", "pinned path is absent or an alias")
	}
	return info, nil
}

// poolPostClass classifies a finished command after its post-exit proof. Only
// shown drift (a non-bounded failure) becomes SOURCE_CHANGED; a bounded probe
// TIMEOUT/INTERRUPTED replaces any class that could satisfy the phase's
// success predicate (mayPass) and otherwise keeps the command's own terminal
// failure. Callers must also refuse to pass a phase whose proof failed. The
// post-exit proof is detection only.
func poolPostClass(class string, mayPass bool, e error) string {
	if e == nil {
		return class
	}
	var bounded sweepGitBounded
	if errors.As(e, &bounded) {
		if mayPass {
			return bounded.class
		}
		return class
	}
	return "SOURCE_CHANGED"
}

// executePool and executePoolCaptured run without a launch guard.
func executePool(ctx context.Context, def *intent.PoolCommand, root string, env []string) (string, bool, wire.Digest) {
	return executePoolGuarded(ctx, def, root, env, nil)
}
func executePoolCaptured(ctx context.Context, def *intent.PoolCommand, root string, env []string) poolCommandResult {
	return executePoolCapturedGuarded(ctx, def, root, env, nil)
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
