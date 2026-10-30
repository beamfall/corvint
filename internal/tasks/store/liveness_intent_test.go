//go:build darwin || linux

package store

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// livenessFixture is a writer-route store with tickets tickets, a held lease
// on the first and the worktree the lease verbs run in.
type livenessFixture struct {
	repo        *intent.Repository
	root        string
	tickets     []string
	ticketFiles []string
	attempt     string
	generation  wire.Size
	requestSeq  int
	ticketsDir  string
	releasesDir string
}

func newLivenessFixture(t *testing.T, tickets int) *livenessFixture {
	t.Helper()
	base, err := os.MkdirTemp("", "atm-liveness-")
	if err != nil {
		t.Fatal(err)
	}
	real, err := filepath.EvalSymlinks(base)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(real) })
	repo := historyStoreAt(t, filepath.Join(real, "repo"), tickets, tickets+20)
	lowerWriterCheckpoint(t, repo)
	if run := writerMutate(t, repo, "writer-seed", nil); !run.completed() || run.fast() {
		t.Fatalf("seed: %+v %v %v", run.rep, run.err, run.stages)
	}
	holdLeasePolicy(t, repo)
	f := &livenessFixture{repo: repo, root: filepath.Join(real, "worktree")}
	holdGit(t, f.root, "init", "-q", "-b", "main")
	holdGit(t, f.root, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "--allow-empty", "-m", "base")
	f.ticketsDir = filepath.Join(repo.IntentRoot(), intent.Dir, "tickets")
	f.releasesDir = filepath.Join(repo.IntentRoot(), intent.Dir, "releases")
	entries, err := os.ReadDir(f.ticketsDir)
	if err != nil || len(entries) != tickets+1 {
		t.Fatalf("tickets %d %v", len(entries), err)
	}
	for _, e := range entries[:2] {
		f.ticketFiles = append(f.ticketFiles, filepath.Join(f.ticketsDir, e.Name()))
		f.tickets = append(f.tickets, fixture.TicketID(strings.TrimSuffix(e.Name(), ".json")))
	}
	claim := f.lease(nil, transaction.LeaseRequest{Verb: transaction.LeaseClaim, TicketID: f.tickets[0], Holder: "agent-1", LeaseMinutes: "60"})
	if !claim.completed() {
		t.Fatalf("claim: %+v %v %v", claim.rep, claim.err, claim.stages)
	}
	f.attempt, f.generation = claim.rep.AttemptID, claim.rep.Generation
	return f
}

func (f *livenessFixture) lease(hook func(string), l transaction.LeaseRequest) writerRun {
	f.requestSeq++
	id := fmt.Sprintf("liveness-%s-%d", strings.ToLower(l.Verb), f.requestSeq)
	return stagedWrite(func(ctx context.Context) (*Report, error) {
		return Lease(ctx, f.repo, historyActor, LeaseChoice{QueueID: fixture.QueueID, RequestID: id, Root: f.root, Lease: l}, WallClock())
	}, hook)
}

func (f *livenessFixture) heartbeat(hook func(string)) writerRun {
	return f.lease(hook, transaction.LeaseRequest{Verb: transaction.LeaseHeartbeat, AttemptID: f.attempt, Generation: f.generation})
}

func (f *livenessFixture) renew(hook func(string)) writerRun {
	return f.lease(hook, transaction.LeaseRequest{Verb: transaction.LeaseRenew, AttemptID: f.attempt, Generation: f.generation, LeaseMinutes: "60"})
}

// sealIntentRecords clears the mode of every intent ticket and release
// file until t ends, so any read of one fails, and plants a symlink in the
// tickets directory, which any listing scan of the intent tree refuses. The
// directories themselves stay openable: the authority session pins each
// directory it may publish into, an O(1) open that lists nothing and reads
// no ticket (decision 0549 S1).
func (f *livenessFixture) sealIntentRecords(t *testing.T) {
	t.Helper()
	for _, dir := range []string{f.ticketsDir, f.releasesDir} {
		entries, err := os.ReadDir(dir)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range entries {
			p := filepath.Join(dir, e.Name())
			info, err := os.Stat(p)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(p, 0); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { os.Chmod(p, info.Mode().Perm()) })
		}
	}
	stray := filepath.Join(f.ticketsDir, "zz-stray.json")
	if err := os.Symlink(os.DevNull, stray); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Remove(stray) })
	if _, err := os.ReadFile(f.ticketFiles[0]); err == nil {
		t.Skip("a cleared mode still reads in this process")
	}
}

// TestJSGV0001_LivenessReadsNoIntentRecords counts intent record reads on
// the liveness route by making every one fail: with every ticket and
// release unreadable and a listing trap planted, at two ticket counts and two history
// lengths, heartbeat and renew still commit on the writer route with the
// attempt and generation unchanged, and the writer checkpoint is not
// rewritten below the advance tail. A claim, which reads tickets, does not
// commit on that route (the control that the seal bites).
func TestJSGV0001_LivenessReadsNoIntentRecords(t *testing.T) {
	t.Parallel()
	for _, tickets := range []int{70, 210} {
		for _, extra := range []int{0, 30} {
			t.Run(fmt.Sprintf("tickets=%d/heartbeats=%d", tickets, extra), func(t *testing.T) {
				t.Parallel()
				f := newLivenessFixture(t, tickets)
				for i := 0; i < extra; i++ {
					if run := f.heartbeat(nil); !run.completed() || !run.fast() {
						t.Fatalf("heartbeat %d: %+v %v %v", i, run.rep, run.err, run.stages)
					}
				}
				f.sealIntentRecords(t)
				before := writerCheckpointFiles(t, f.repo)
				for _, run := range []writerRun{f.heartbeat(nil), f.renew(nil)} {
					if !run.completed() || !run.fast() || run.rep.AttemptID != f.attempt || run.rep.Generation != f.generation {
						t.Fatalf("liveness with unreadable intent records: %v %v, want a fast commit", run.err, run.stages)
					}
				}
				if after := writerCheckpointFiles(t, f.repo); after != before {
					t.Fatal("a liveness write below the advance tail rewrote the writer checkpoint")
				}
				control := f.lease(nil, transaction.LeaseRequest{Verb: transaction.LeaseClaim, TicketID: f.tickets[1], Holder: "agent-2", LeaseMinutes: "60"})
				if control.completed() && control.fast() {
					t.Fatalf("claim with unreadable tickets committed on the writer route: %v", control.stages)
				}
			})
		}
	}
}

// TestJSGV0001_LivenessBindDeclinesMovedInputs changes, between the model and
// the commit, each input the liveness model read: the attempt it rewrites,
// reservations.json, pools.json, programs.json, queue.json, policy.json,
// head.json and the primary branch. Each declines the writer route at its
// bind or branch recheck, so no fast commit is made from the moved input.
// An intent ticket edit, which the liveness model does not read, does not.
func TestJSGV0001_LivenessBindDeclinesMovedInputs(t *testing.T) {
	t.Parallel()
	state := func(name string) func(*livenessFixture) string {
		return func(f *livenessFixture) string { return filepath.Join(f.repo.StateDir, filepath.FromSlash(name)) }
	}
	intentFile := func(name string) func(*livenessFixture) string {
		return func(f *livenessFixture) string { return filepath.Join(f.repo.IntentRoot(), intent.Dir, name) }
	}
	cases := []struct {
		name string
		path func(*livenessFixture) string
	}{
		{"attempt", func(f *livenessFixture) string { return state("attempts/" + f.attempt + ".json")(f) }},
		{"reservations", state("reservations.json")},
		{"pools", state("pools.json")},
		{"programs", state("programs.json")},
		{"queue", intentFile(intent.QueueFile)},
		{"policy", intentFile(intent.PolicyFile)},
		{"head", state("head.json")},
		{"branch", func(f *livenessFixture) string { return filepath.Join(f.repo.PrimaryWorktree, ".git", "HEAD") }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			f := newLivenessFixture(t, 70)
			p := c.path(f)
			change := func() {
				raw, err := os.ReadFile(p)
				if err != nil && !os.IsNotExist(err) {
					t.Fatal(err)
				}
				if c.name == "branch" {
					raw = []byte("ref: refs/heads/moved")
				}
				fixture.Write(t, p, append(raw, '\n'))
			}
			run := f.heartbeat(at("fast.lease.model", change))
			why := run.declined()
			if run.fast() || !(strings.HasSuffix(why, ": bind") || strings.HasSuffix(why, ": branch")) {
				t.Fatalf("%s moved after the model: declined %q, stages %v, want a bind or branch decline", c.name, why, run.stages)
			}
		})
	}
	t.Run("ticket", func(t *testing.T) {
		t.Parallel()
		g := newLivenessFixture(t, 70)
		ticket := g.ticketFiles[1]
		edit := func() {
			raw, err := os.ReadFile(ticket)
			if err != nil {
				t.Fatal(err)
			}
			fixture.Write(t, ticket, append(raw, '\n'))
		}
		edit()
		if run := g.heartbeat(nil); !run.completed() || !run.fast() {
			t.Fatalf("heartbeat after an intent ticket edit: %+v %v %v, want a fast commit", run.rep, run.err, run.stages)
		}
		if run := g.renew(at("fast.lease.model", edit)); !run.completed() || !run.fast() {
			t.Fatalf("renew across an intent ticket edit: %+v %v %v, want a fast commit", run.rep, run.err, run.stages)
		}
		// A verb that reads the ticket still observes the edit.
		claim := g.lease(nil, transaction.LeaseRequest{Verb: transaction.LeaseClaim, TicketID: g.tickets[1], Holder: "agent-2", LeaseMinutes: "60"})
		if claim.completed() {
			t.Fatalf("claim of an edited ticket completed: %+v %v", claim.rep, claim.stages)
		}
	})
}
