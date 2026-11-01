//go:build unix

package store

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func headRepo(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "repo")
	holdGit(t, root, "init", "-q", "-b", "main")
	holdGit(t, root, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "--allow-empty", "-m", "base")
	return root
}

func gitHead(t *testing.T, root string) string {
	t.Helper()
	out, err := gitOutput(root, "rev-parse", "--verify", "--end-of-options", "HEAD^{commit}")
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(out))
}

// PSR-V0-224: the file resolver names the commit Git resolves for HEAD in
// every files-backend layout it accepts (a loose branch, a packed branch, a
// detached HEAD, a linked worktree), and reports false for one it does not
// read exactly (a symlinked ref, an unborn branch, a reftable store).
func TestPSRV0224_HeadFilesResolveAsGit(t *testing.T) {
	t.Parallel()
	resolve := func(t *testing.T, root string) (string, bool) {
		t.Helper()
		l, ok := observeHeadLayout(root)
		if !ok {
			return "", false
		}
		return l.head()
	}
	t.Run("loose packed detached", func(t *testing.T) {
		root := headRepo(t)
		if oid, ok := resolve(t, root); !ok || oid != gitHead(t, root) {
			t.Fatalf("loose: %q %v, want %q", oid, ok, gitHead(t, root))
		}
		holdGit(t, root, "pack-refs", "--all")
		if _, err := os.Lstat(filepath.Join(root, ".git", "refs", "heads", "main")); !os.IsNotExist(err) {
			t.Fatalf("pack-refs left a loose main: %v", err)
		}
		if oid, ok := resolve(t, root); !ok || oid != gitHead(t, root) {
			t.Fatalf("packed: %q %v, want %q", oid, ok, gitHead(t, root))
		}
		holdGit(t, root, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "--allow-empty", "-m", "next")
		if oid, ok := resolve(t, root); !ok || oid != gitHead(t, root) {
			t.Fatalf("loose over packed: %q %v, want %q", oid, ok, gitHead(t, root))
		}
		holdGit(t, root, "checkout", "-q", "--detach", "HEAD~1")
		if oid, ok := resolve(t, root); !ok || oid != gitHead(t, root) {
			t.Fatalf("detached: %q %v, want %q", oid, ok, gitHead(t, root))
		}
	})
	t.Run("linked worktree", func(t *testing.T) {
		root := headRepo(t)
		linked := filepath.Join(filepath.Dir(root), "linked")
		holdGit(t, root, "worktree", "add", "-q", "-b", "side", linked)
		holdGit(t, linked, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "--allow-empty", "-m", "side")
		if oid, ok := resolve(t, linked); !ok || oid != gitHead(t, linked) || oid == gitHead(t, root) {
			t.Fatalf("linked: %q %v, want %q", oid, ok, gitHead(t, linked))
		}
	})
	t.Run("declines", func(t *testing.T) {
		root := headRepo(t)
		ref := filepath.Join(root, ".git", "refs", "heads", "main")
		b, err := os.ReadFile(ref)
		if err != nil {
			t.Fatal(err)
		}
		target := filepath.Join(root, ".git", "main-target")
		if err := os.WriteFile(target, b, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(ref); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, ref); err != nil {
			t.Fatal(err)
		}
		if oid, ok := resolve(t, root); ok {
			t.Fatalf("symlinked ref resolved to %q", oid)
		}
		unborn := filepath.Join(t.TempDir(), "unborn")
		holdGit(t, unborn, "init", "-q", "-b", "main")
		if oid, ok := resolve(t, unborn); ok {
			t.Fatalf("unborn branch resolved to %q", oid)
		}
		if reason := reftableUnsupported(t, "git"); reason != "" {
			t.Skip(reason)
		}
		reftable := filepath.Join(t.TempDir(), "reftable")
		holdGit(t, reftable, "init", "-q", "--ref-format=reftable", "-b", "main")
		holdGit(t, reftable, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "--allow-empty", "-m", "base")
		if _, ok := observeHeadLayout(reftable); ok {
			t.Fatal("a reftable store was accepted for file resolution")
		}
	})
}

// reftableUnsupported names why git cannot create a reftable store, or ""
// when it can. Git before 2.45 rejects --ref-format as an unknown option
// (exit 129); any other failure is a broken git, not a missing capability.
func reftableUnsupported(t *testing.T, git string) string {
	t.Helper()
	reason, err := probeReftable(t, git)
	if err != nil {
		t.Fatal(err)
	}
	return reason
}

func probeReftable(t *testing.T, git string) (string, error) {
	t.Helper()
	c := exec.Command(git, "init", "-q", "--ref-format=reftable", filepath.Join(t.TempDir(), "probe"))
	c.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1")
	out, err := c.CombinedOutput()
	if err == nil {
		return "", nil
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() == 129 && bytes.Contains(out, []byte("ref-format")) {
		line, _, _ := strings.Cut(strings.TrimSpace(string(out)), "\n")
		return "git lacks --ref-format=reftable (needs git 2.45 or later): " + line, nil
	}
	return "", fmt.Errorf("reftable probe: %v %s", err, out)
}

// PSR-V0-224: the reftable probe skips only for the unknown-option refusal of
// a git older than 2.45, reports support when init accepts the format, and
// treats any other failure as a broken git.
func TestPSRV0224_ReftableProbeNamesMissingSupport(t *testing.T) {
	t.Parallel()
	fake := func(t *testing.T, body string) string {
		t.Helper()
		path := filepath.Join(t.TempDir(), "git")
		if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		return path
	}
	old := fake(t, "echo \"error: unknown option \\`ref-format=reftable'\" >&2\necho 'usage: git init' >&2\nexit 129")
	if reason := reftableUnsupported(t, old); reason != "git lacks --ref-format=reftable (needs git 2.45 or later): error: unknown option `ref-format=reftable'" {
		t.Fatalf("old git reason %q", reason)
	}
	if reason := reftableUnsupported(t, fake(t, "exit 0")); reason != "" {
		t.Fatalf("supporting git reported %q", reason)
	}
	for _, body := range []string{"echo 'fatal: cannot mkdir' >&2\nexit 128", "echo 'usage: git init' >&2\nexit 129"} {
		if reason, err := probeReftable(t, fake(t, body)); err == nil {
			t.Fatalf("broken git %q skipped as %q", body, reason)
		}
	}
}

// PSR-V0-224: a HEAD-bound base is confirmed against the ref files under the
// lock, not against the journal. While the deriver is blocked the claim's
// checkout gains a commit and nothing writes the journal, so the audited
// snapshot is unchanged. The retried writer route must still refuse the base
// it observed before the commit, decline once more, and let the complete
// route bind the claim to the new commit.
func TestPSRV0224_ClaimRebindsMovedHeadWithoutJournalAdvance(t *testing.T) {
	t.Parallel()
	repo, root, ticketID := claimTargetStore(t)
	started, release := make(chan struct{}), make(chan struct{})
	var runs atomic.Int32
	derive := func(context.Context, string, string, string, string) ([]string, string, bool) {
		if runs.Add(1) == 1 {
			close(started)
			<-release
		}
		return []string{"src/"}, string(wire.Sum([]byte("src/"))), true
	}
	done := make(chan writerRun, 1)
	go func() {
		done <- stagedWrite(func(ctx context.Context) (*Report, error) {
			return Lease(ctx, repo, historyActor, LeaseChoice{QueueID: fixture.QueueID, RequestID: "moved-head", Root: root, Derive: derive,
				Lease: transaction.LeaseRequest{Verb: transaction.LeaseClaim, TicketID: ticketID, Holder: "agent-1", LeaseMinutes: "60"}}, WallClock())
		}, nil)
	}()
	released := false
	defer func() {
		if !released {
			close(release)
		}
	}()
	select {
	case <-started:
	case run := <-done:
		t.Fatalf("claim returned before the deriver ran: %+v %v %v", run.rep, run.err, run.stages)
	case <-time.After(2 * time.Minute):
		t.Fatal("deriver never started (hang guard)")
	}
	holdGit(t, root, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "--allow-empty", "-m", "integrated")
	moved := gitHead(t, root)
	close(release)
	released = true
	var run writerRun
	select {
	case run = <-done:
	case <-time.After(2 * time.Minute):
		t.Fatal("claim never returned after release (hang guard)")
	}
	if !run.completed() {
		t.Fatalf("claim not committed: %+v %v %v", run.rep, run.err, run.stages)
	}
	if n := claimDeclines(run); n != 2 || run.fast() {
		t.Fatalf("writer route deferred the claim observation %d times (fast %v), want 2 then the complete route: %v", n, run.fast(), run.stages)
	}
	proof, err := readLeaseProof(context.Background(), repo)
	if err != nil {
		t.Fatal(err)
	}
	a, ok := lockedAttempt(proof, run.rep.AttemptID)
	if !ok || a.BaseCommit != moved {
		t.Fatalf("claim base %q, want the commit made while the deriver ran %q", a.BaseCommit, moved)
	}
}

// PSR-V0-224: a HEAD resolution whose reads straddle a checkout is refused.
// HEAD names main at B; between the HEAD read and the main read, HEAD moves
// to other at C and main is reset to A. The chain read names A, a commit
// HEAD never named at one instant, so the re-read of HEAD must refuse it.
func TestPSRV0224_HeadRefusesChainMovedBetweenReads(t *testing.T) {
	t.Parallel()
	root := headRepo(t)
	a := gitHead(t, root)
	commit := func(msg string) {
		holdGit(t, root, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "--allow-empty", "-m", msg)
	}
	commit("b")
	holdGit(t, root, "checkout", "-q", "-b", "other")
	commit("c")
	c := gitHead(t, root)
	holdGit(t, root, "checkout", "-q", "main")
	l, ok := observeHeadLayout(root)
	if !ok {
		t.Fatal("layout not observed")
	}
	main := filepath.Join("refs", "heads", "main")
	fired := false
	l.read = func(path string) ([]byte, bool) {
		if !fired && strings.HasSuffix(path, main) {
			fired = true
			holdGit(t, root, "symbolic-ref", "HEAD", "refs/heads/other")
			holdGit(t, root, "update-ref", "refs/heads/main", a)
		}
		return readRefFile(path)
	}
	if oid, ok := l.head(); ok {
		t.Fatalf("chain read across a checkout served %q (main reset to %q, HEAD now at %q)", oid, a, c)
	}
	if !fired {
		t.Fatal("the main ref was never read")
	}
	if oid, ok := l.head(); !ok || oid != c || oid != gitHead(t, root) {
		t.Fatalf("settled HEAD: %q %v, want %q", oid, ok, c)
	}
}

// PSR-V0-224: a probe's cached layout serves only while the claim root is
// still routed to the Git directory it was observed with. The linked
// worktree observed at A moves away and a new worktree at B takes its root;
// the old admin directory still names A, and the locked observer must
// refuse it instead of confirming the stale base.
func TestPSRV0224_CachedLayoutRefusesReplacedWorktree(t *testing.T) {
	t.Parallel()
	root := headRepo(t)
	b := gitHead(t, root)
	linked := filepath.Join(filepath.Dir(root), "linked")
	holdGit(t, root, "worktree", "add", "-q", "-b", "side", linked)
	holdGit(t, linked, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "--allow-empty", "-m", "side")
	a := gitHead(t, linked)
	p := &claimProbe{layouts: map[string]*headLayout{}}
	if oid, ok := p.head(linked, false); !ok || oid != a {
		t.Fatalf("probe: %q %v, want %q", oid, ok, a)
	}
	holdGit(t, root, "worktree", "move", linked, filepath.Join(filepath.Dir(root), "moved"))
	holdGit(t, root, "worktree", "add", "-q", "-b", "other", linked, "main")
	if now := gitHead(t, linked); now != b {
		t.Fatalf("replacement worktree at %q, want %q", now, b)
	}
	if oid, ok := p.head(linked, true); ok {
		t.Fatalf("cached layout of the moved worktree served %q for a root now at %q", oid, b)
	}
	fresh := &claimProbe{layouts: map[string]*headLayout{}}
	if oid, ok := fresh.head(linked, false); !ok || oid != b {
		t.Fatalf("re-observed: %q %v, want %q", oid, ok, b)
	}
}
