package store

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// MaxPreflightSpecFiles bounds the spec files one preflight reads at a
// commit (TOL-V0-025).
const MaxPreflightSpecFiles = 4096

// PreflightBlobBytes is the largest file preflight reads at a commit; a
// larger spec file is skipped with a warning and a larger plan refused.
const PreflightBlobBytes = knowHowSymbolMaxBlobBytes

// maxPreflightTreeEntryBytes bounds one path of the tree listing.
const maxPreflightTreeEntryBytes = 64 << 10

// preflightGitConfig disables repository hooks and the file-system monitor
// for every Git command preflight runs, so the repository's configuration
// starts no program outside a gate's timeout and output cap (TOL-V0-027).
var preflightGitConfig = []string{"-c", "core.hooksPath=" + os.DevNull, "-c", "core.fsmonitor=false"}

func preflightGit(root string, args ...string) ([]byte, error) {
	return gitOutput(root, append(append([]string{}, preflightGitConfig...), args...)...)
}

// TreePathsAtCommit lists the blob paths of commit's tree in root, under
// prefixes when any are given, for which keep returns true, with one
// streamed `git ls-tree`. The prefixes are literal pathspecs, so Git lists
// nothing outside them. It reads Git objects only, so it needs no checkout
// of the commit and writes nothing. More than MaxPreflightSpecFiles kept
// paths, or one path longer than 64 KiB, refuses LIMIT_EXCEEDED while
// reading, before the rest of the listing is read.
func TreePathsAtCommit(root, commit string, prefixes []string, keep func(string) bool) (paths []string, err error) {
	args := append(append([]string{}, preflightGitConfig...), "-c", "credential.helper=", "ls-tree", "-r", "-z", "--name-only", "--full-tree", commit, "--")
	for _, p := range prefixes {
		args = append(args, path.Clean(p))
	}
	c := exec.Command("git", args...)
	c.Dir = root
	c.Env = append(gitEnvironment(), "GIT_LITERAL_PATHSPECS=1")
	stdout, err := c.StdoutPipe()
	if err != nil {
		return nil, gitObservationFailed(err)
	}
	if err := c.Start(); err != nil {
		return nil, gitObservationFailed(err)
	}
	defer func() {
		if err != nil {
			_ = c.Process.Kill()
			_ = c.Wait()
		}
	}()
	r := bufio.NewReaderSize(stdout, maxPreflightTreeEntryBytes)
	for {
		entry, rerr := r.ReadSlice(0)
		switch {
		case rerr == bufio.ErrBufferFull:
			return nil, wire.Errorf(wire.CodeLimitExceeded, "--path", "a path at %s is longer than %d bytes", commit, maxPreflightTreeEntryBytes)
		case rerr == io.EOF && len(entry) == 0:
			if werr := c.Wait(); werr != nil {
				return nil, gitObservationFailed(werr)
			}
			return paths, nil
		case rerr != nil:
			return nil, wire.Errorf(wire.CodeUnsupported, "git", "git answered short: %v", rerr)
		}
		p := string(entry[:len(entry)-1])
		if !keep(p) {
			continue
		}
		if len(paths) == MaxPreflightSpecFiles {
			return nil, wire.Errorf(wire.CodeLimitExceeded, "--path", "more than %d spec files at %s; narrow them with --path", MaxPreflightSpecFiles, commit)
		}
		paths = append(paths, p)
	}
}

// PreflightGateRun is what one --deep gate observed. Changed names how a
// gate left the worktree when it is no longer a clean checkout of the
// commit, else "".
type PreflightGateRun struct {
	GateID   string
	Passed   bool
	Class    string
	ExitCode string
	Changed  string
	Output   []byte
}

// PreflightGateDefinitions resolves each gate id against policy with the
// lease runner's rules: a COMMAND gate in the worktree with an expected exit
// code, no reducer, shared resource or inputs (TOL-V0-027). An undeclared
// gate refuses GATE_UNKNOWN, any other UNSUPPORTED.
func PreflightGateDefinitions(policy *intent.Policy, gateIDs []string) ([]*intent.GateDefinition, error) {
	defs := make([]*intent.GateDefinition, 0, len(gateIDs))
	for _, id := range gateIDs {
		def, err := commandGate(policy, id)
		if err != nil {
			return nil, err
		}
		defs = append(defs, def)
	}
	return defs, nil
}

// PreflightGates runs defs, in order, in one temporary detached worktree of
// commit created under the system temporary directory, with each gate's
// declared environment, timeout and output cap. After each gate the
// worktree must still be a clean checkout of commit, as the lease runner
// requires; a gate that changed it does not pass and no later gate runs.
// The worktree is removed before return on every path, so the caller's
// checkout and its uncommitted state are never what runs (TOL-V0-027).
// When ctx ends (the caller's interrupt), the running gate's process group
// is killed and waited for, the worktree removed, and ctx's error returned.
// The only repository effect is that worktree's transient administrative
// entry, which the removal deletes; a failed removal is an error naming the
// repair.
func PreflightGates(ctx context.Context, root, commit string, defs []*intent.GateDefinition) (runs []PreflightGateRun, err error) {
	tree, err := resolvePreflightObject(root, commit+"^{tree}")
	if err != nil {
		return nil, err
	}
	dir, err := os.MkdirTemp("", "corvint-tasks-preflight-")
	if err != nil {
		return nil, wire.Errorf(wire.CodeUnsupported, "--deep", "cannot create a temporary directory: %v", err)
	}
	wt := filepath.Join(dir, "worktree")
	defer func() {
		_, rerr := preflightGit(root, "worktree", "remove", "--force", wt)
		_ = os.RemoveAll(dir)
		if rerr != nil && err == nil {
			err = wire.Errorf(wire.CodeUnsupported, "--deep", "the temporary worktree %s could not be removed; run `git worktree prune`", wt)
		}
	}()
	if _, err := preflightGit(root, "worktree", "add", "--detach", "--quiet", wt, commit); err != nil {
		return nil, err
	}
	for _, def := range defs {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		env, _ := gateEnvironment(def.Env)
		out := &cappedOutput{}
		run := execute(ctx, def, wt, env, out)
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		r := PreflightGateRun{GateID: def.GateID, Class: run.class, Output: bytes.Clone(out.buf.Bytes())}
		if run.exitCode != nil {
			r.ExitCode = string(*run.exitCode)
		}
		r.Changed = preflightWorktreeChange(wt, commit, tree)
		r.Passed = run.class == "EXIT" && run.exitCode.Int() == def.ExpectedExit.Int() && r.Changed == ""
		runs = append(runs, r)
		if r.Changed != "" {
			break
		}
	}
	return runs, nil
}

// preflightWorktreeChange is "" when wt is still a clean checkout of
// commit, else how it differs.
func preflightWorktreeChange(wt, commit, tree string) string {
	head, err := resolvePreflightObject(wt, "HEAD")
	if err != nil {
		return "its HEAD is unreadable"
	}
	if head != commit {
		return "its HEAD moved to " + head
	}
	if t, err := resolvePreflightObject(wt, "HEAD^{tree}"); err != nil || t != tree {
		return "its HEAD tree is not the commit's"
	}
	status, err := preflightGit(wt, "status", "--porcelain", "--untracked-files=all")
	if err != nil {
		return "its status is unreadable"
	}
	if len(status) > 0 {
		return "it has uncommitted or untracked changes"
	}
	return ""
}

func resolvePreflightObject(root, rev string) (string, error) {
	out, err := preflightGit(root, "rev-parse", "--verify", "--end-of-options", rev)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
