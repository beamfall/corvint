package store

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"

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
// commit, else "". Unchecked names why that check did not finish within
// the gate's timeout, else "".
type PreflightGateRun struct {
	GateID    string
	Passed    bool
	Class     string
	ExitCode  string
	Changed   string
	Unchecked string
	Output    []byte
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
		// execute stops watching the group once the gate exits, so a
		// descendant it left running would outlive it and every later
		// gate, interrupt and the worktree removal.
		killGateGroup(run.group)
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		r := PreflightGateRun{GateID: def.GateID, Class: run.class, Output: bytes.Clone(out.buf.Bytes())}
		if run.exitCode != nil {
			r.ExitCode = string(*run.exitCode)
		}
		r.Changed, r.Unchecked = preflightWorktreeChange(ctx, wt, commit, tree, time.Duration(def.TimeoutSeconds.Int())*time.Second)
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		r.Passed = run.class == "EXIT" && run.exitCode.Int() == def.ExpectedExit.Int() && r.Changed == "" && r.Unchecked == ""
		runs = append(runs, r)
		if r.Changed != "" || r.Unchecked != "" {
			break
		}
	}
	return runs, nil
}

// preflightWorktreeChange checks that wt is still a clean checkout of
// commit after a gate. changed is "" when it is, else how it differs.
// Reading the worktree can run repository programs, such as a clean filter
// on a file the gate touched, so every Git call here runs in its own
// process group, which is killed when ctx ends (the caller's interrupt) or
// the gate's declared timeout passes. unchecked names a check that did not
// finish within that timeout; the caller reads ctx itself (TOL-V0-027).
func preflightWorktreeChange(ctx context.Context, wt, commit, tree string, timeout time.Duration) (changed, unchecked string) {
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	late := func() string {
		if cctx.Err() != nil {
			return "its state was not read within the gate's timeout of " + timeout.String()
		}
		return ""
	}
	head, err := postGateResolve(cctx, wt, "HEAD")
	if err != nil {
		if l := late(); l != "" {
			return "", l
		}
		return "its HEAD is unreadable", ""
	}
	if head != commit {
		return "its HEAD moved to " + head, ""
	}
	t, err := postGateResolve(cctx, wt, "HEAD^{tree}")
	if l := late(); l != "" {
		return "", l
	}
	if err != nil || t != tree {
		return "its HEAD tree is not the commit's", ""
	}
	dirty, err := writesAnything(cctx, postGateGit(wt, "status", "--porcelain", "-z", "--untracked-files=all"))
	switch {
	case late() != "":
		return "", late()
	case err != nil:
		return "its status is unreadable", ""
	case dirty:
		return "it has uncommitted or untracked changes", ""
	}
	return "", ""
}

// postGateGit is a preflight Git command in wt.
func postGateGit(wt string, args ...string) *exec.Cmd {
	c := exec.Command("git", append(append(append([]string{}, preflightGitConfig...), "-c", "credential.helper="), args...)...)
	c.Dir = wt
	c.Env = gitEnvironment()
	return c
}

// boundedEvent, when a test sets it, observes the bounded post-gate calls:
// "retire" as retire starts, "cancel" as the cancel callback starts,
// "signal" before each kill of the process group and "reaped" once Wait
// has returned.
var boundedEvent func(string)

func noteBounded(event string) {
	if boundedEvent != nil {
		boundedEvent(event)
	}
}

// signalBounded kills c's process group.
func signalBounded(c *exec.Cmd) {
	noteBounded("signal")
	killGate(c)
}

// startBounded starts c in its own process group, whose id is c's pid, and
// kills that group when ctx ends. The caller calls retire exactly once,
// after it has read c's output to the end or abandoned the read, and never
// calls Wait itself. retire ends the cancel watch, waits for a cancel kill
// already under way, kills whatever remains of the group and only then
// waits for c. Every signal therefore lands before c is reaped: until then
// c's pid, and so the group's id, cannot be reused, and the kill cannot
// reach another group. Killing on every path, a clean exit included, means
// a child c left running, such as one a clean filter put in the
// background, does not outlive the call. A command that closes its output
// and then keeps running is killed and reads as failed, never as clean.
func startBounded(ctx context.Context, c *exec.Cmd) (retire func() error, err error) {
	containGate(c)
	c.WaitDelay = gateWaitDelay
	if err := c.Start(); err != nil {
		return nil, gitObservationFailed(err)
	}
	cancelled := make(chan struct{})
	stop := context.AfterFunc(ctx, func() {
		defer close(cancelled)
		noteBounded("cancel")
		signalBounded(c)
	})
	return func() error {
		noteBounded("retire")
		if !stop() {
			<-cancelled
		}
		signalBounded(c)
		err := c.Wait()
		noteBounded("reaped")
		return err
	}, nil
}

// maxPostGateRevBytes bounds a post-gate rev-parse's output, one object id
// and a newline.
const maxPostGateRevBytes = 1024

func postGateResolve(ctx context.Context, wt, rev string) (string, error) {
	c := postGateGit(wt, "rev-parse", "--verify", "--end-of-options", rev)
	stdout, err := c.StdoutPipe()
	if err != nil {
		return "", gitObservationFailed(err)
	}
	retire, err := startBounded(ctx, c)
	if err != nil {
		return "", err
	}
	out, rerr := io.ReadAll(io.LimitReader(stdout, maxPostGateRevBytes+1))
	werr := retire()
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if rerr == nil && len(out) > maxPostGateRevBytes {
		rerr = errors.New("rev-parse output exceeds its bound")
	}
	if werr != nil || rerr != nil {
		return "", gitObservationFailed(errors.Join(werr, rerr))
	}
	return strings.TrimSpace(string(out)), nil
}

// writesAnything runs c and reports whether it writes any output. It reads
// at most one byte: the first byte settles the answer, so c's process group
// is then killed rather than read to the end, and a gate that left many
// untracked files costs no more than one that left one. The group is also
// killed when ctx ends, which returns ctx's error, and on return in every
// case (startBounded). A command that writes nothing and fails is an error.
func writesAnything(ctx context.Context, c *exec.Cmd) (bool, error) {
	stdout, err := c.StdoutPipe()
	if err != nil {
		return false, gitObservationFailed(err)
	}
	retire, err := startBounded(ctx, c)
	if err != nil {
		return false, err
	}
	var first [1]byte
	n, rerr := io.ReadFull(stdout, first[:])
	werr := retire()
	if n == 1 {
		return true, nil
	}
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if werr != nil || rerr != io.EOF {
		return false, gitObservationFailed(errors.Join(werr, rerr))
	}
	return false, nil
}

func resolvePreflightObject(root, rev string) (string, error) {
	out, err := preflightGit(root, "rev-parse", "--verify", "--end-of-options", rev)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
