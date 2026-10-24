//go:build unix

package store

import (
	"context"
	"os"
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
		reftable := filepath.Join(t.TempDir(), "reftable")
		holdGit(t, reftable, "init", "-q", "--ref-format=reftable", "-b", "main")
		holdGit(t, reftable, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "--allow-empty", "-m", "base")
		if _, ok := observeHeadLayout(reftable); ok {
			t.Fatal("a reftable store was accepted for file resolution")
		}
	})
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
