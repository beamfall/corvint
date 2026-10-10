package store

import (
	"bytes"
	"context"
	"os"
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

// TreePathsAtCommit lists the blob paths of commit's tree in root for which
// keep returns true, with one `git ls-tree`. It reads Git objects only, so
// it needs no checkout of the commit and writes nothing. More than
// MaxPreflightSpecFiles kept paths refuse LIMIT_EXCEEDED.
func TreePathsAtCommit(root, commit string, keep func(string) bool) ([]string, error) {
	out, err := gitOutput(root, "ls-tree", "-r", "-z", "--name-only", "--full-tree", commit)
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, p := range strings.Split(string(out), "\x00") {
		if p == "" || !keep(p) {
			continue
		}
		if len(paths) == MaxPreflightSpecFiles {
			return nil, wire.Errorf(wire.CodeLimitExceeded, "--path", "more than %d spec files at %s; narrow them with --path", MaxPreflightSpecFiles, commit)
		}
		paths = append(paths, p)
	}
	return paths, nil
}

// PreflightGateRun is what one --deep gate observed.
type PreflightGateRun struct {
	GateID   string
	Passed   bool
	Class    string
	ExitCode string
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
// declared environment, timeout and output cap. The worktree is removed
// before return on every path, so the caller's checkout and its uncommitted
// state are never what runs (TOL-V0-027). The only repository effect is that
// worktree's transient administrative entry, which the removal deletes; a
// failed removal is an error naming the repair.
func PreflightGates(ctx context.Context, root, commit string, defs []*intent.GateDefinition) (runs []PreflightGateRun, err error) {
	dir, err := os.MkdirTemp("", "corvint-tasks-preflight-")
	if err != nil {
		return nil, wire.Errorf(wire.CodeUnsupported, "--deep", "cannot create a temporary directory: %v", err)
	}
	wt := filepath.Join(dir, "worktree")
	defer func() {
		_, rerr := gitOutput(root, "worktree", "remove", "--force", wt)
		_ = os.RemoveAll(dir)
		if rerr != nil && err == nil {
			err = wire.Errorf(wire.CodeUnsupported, "--deep", "the temporary worktree %s could not be removed; run `git worktree prune`", wt)
		}
	}()
	if _, err := gitOutput(root, "worktree", "add", "--detach", "--quiet", wt, commit); err != nil {
		return nil, err
	}
	for _, def := range defs {
		env, _ := gateEnvironment(def.Env)
		out := &cappedOutput{}
		run := execute(ctx, def, wt, env, out)
		r := PreflightGateRun{GateID: def.GateID, Class: run.class, Output: bytes.Clone(out.buf.Bytes())}
		if run.exitCode != nil {
			r.ExitCode = string(*run.exitCode)
		}
		r.Passed = run.class == "EXIT" && run.exitCode.Int() == def.ExpectedExit.Int()
		runs = append(runs, r)
	}
	return runs, nil
}
