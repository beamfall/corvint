//go:build darwin || linux

package store_test

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/authority"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
)

// syncEvent is one step of a lease write as LGL-V0-016's tests observe it:
// an artifact about to be published, or a directory sync starting, ending or
// failing by injection.
type syncEvent struct {
	kind string // publish, start, end, fail
	role string // publish only
	dir  string // the directory, relative to the state dir
	info os.FileInfo
}

// syncRecorder records one lease write's publications and directory syncs.
// failAt, when not negative, fails that sync (counting from zero) without
// calling it.
type syncRecorder struct {
	t        *testing.T
	stateDir string
	failAt   int
	inject   error

	mu       sync.Mutex
	events   []syncEvent
	calls    int
	inFlight int
	maxIn    int
}

func newSyncRecorder(t *testing.T, stateDir string, failAt int) *syncRecorder {
	t.Helper()
	r := &syncRecorder{t: t, stateDir: stateDir, failAt: failAt, inject: errors.New("injected directory sync failure")}
	t.Cleanup(authority.SetDirectorySyncTestHook(r.sync))
	t.Cleanup(store.SetPublishFaultForTest(func(a transaction.Artifact) error {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.events = append(r.events, syncEvent{kind: "publish", role: a.Role, dir: filepath.Dir(filepath.FromSlash(a.Target))})
		return nil
	}))
	return r
}

func (r *syncRecorder) reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events, r.calls, r.inFlight, r.maxIn = nil, 0, 0, 0
}

func (r *syncRecorder) sync(info os.FileInfo, next func() error) error {
	r.mu.Lock()
	k := r.calls
	r.calls++
	if k == r.failAt {
		r.events = append(r.events, syncEvent{kind: "fail", info: info})
		r.mu.Unlock()
		return r.inject
	}
	r.events = append(r.events, syncEvent{kind: "start", info: info})
	r.inFlight++
	r.maxIn = max(r.maxIn, r.inFlight)
	r.mu.Unlock()
	// A short wait makes syncs issued together observably overlap.
	time.Sleep(2 * time.Millisecond)
	err := next()
	r.mu.Lock()
	r.inFlight--
	r.events = append(r.events, syncEvent{kind: "end", info: info})
	r.mu.Unlock()
	return err
}

// resolved names each sync event's directory relative to the state dir.
func (r *syncRecorder) resolved() []syncEvent {
	r.t.Helper()
	dirs := map[string]os.FileInfo{}
	err := filepath.WalkDir(r.stateDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return err
		}
		info, err := os.Stat(p)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(r.stateDir, p)
		dirs[rel] = info
		return err
	})
	if err != nil {
		r.t.Fatal(err)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	out := append([]syncEvent(nil), r.events...)
	for i := range out {
		if out[i].info == nil {
			continue
		}
		out[i].dir = "?"
		for rel, info := range dirs {
			if os.SameFile(info, out[i].info) {
				out[i].dir = rel
			}
		}
	}
	return out
}

// endedBetween reports whether a sync of dir ended strictly between the
// events at from and to.
func endedBetween(events []syncEvent, dir string, from, to int) bool {
	for i := from + 1; i < to && i < len(events); i++ {
		if events[i].kind == "end" && events[i].dir == dir {
			return true
		}
	}
	return false
}

// checkOrderingEdges asserts the edges crash consistency relies on for one
// lease write: no sync is in flight when any artifact is published; the
// receipts directory sync returns before the first post is published; every
// post's directory sync returns after the last post is published (overlap
// only) and before the head is; the head's directory sync returns after it.
func checkOrderingEdges(t *testing.T, label string, events []syncEvent, overlap bool) {
	t.Helper()
	receipt, head, lastPost := -1, -1, -1
	var posts []int
	open := 0
	for i, e := range events {
		switch e.kind {
		case "start":
			open++
		case "end":
			open--
		case "publish":
			if open != 0 {
				t.Errorf("%s: %d directory syncs in flight when %s %s was published", label, open, e.role, e.dir)
			}
			switch e.role {
			case "RECEIPT":
				receipt = i
			case "POST":
				posts = append(posts, i)
				lastPost = i
			case "HEAD":
				head = i
			}
		}
	}
	if receipt < 0 || head < 0 || len(posts) == 0 {
		t.Fatalf("%s: receipt %d, posts %v, head %d published", label, receipt, posts, head)
	}
	if !endedBetween(events, "receipts", receipt, posts[0]) {
		t.Errorf("%s: the receipts directory sync did not return before the first post was published", label)
	}
	for _, p := range posts {
		from := p
		if overlap {
			from = lastPost
		}
		if !endedBetween(events, events[p].dir, from, head) {
			t.Errorf("%s: no sync of post directory %q returned between its publication and the head's (overlap %v)", label, events[p].dir, overlap)
		}
	}
	if !endedBetween(events, ".", head, len(events)) {
		t.Errorf("%s: the head directory sync did not return after the head was published", label)
	}
}

// TestLGLV0016_OverlappedPublishSyncsKeepEveryOrderingEdge: on both lease
// routes a claim, heartbeat, renew and release with overlapped publish syncs
// keep every ordering edge, overlap the claim's post syncs, and make exactly
// the sync calls the one-at-a-time writes make; the audit passes.
func TestLGLV0016_OverlappedPublishSyncsKeepEveryOrderingEdge(t *testing.T) {
	for _, route := range leaseRoutes {
		t.Run(route.name, func(t *testing.T) {
			run := func(overlap bool) []int {
				defer store.SetLeasePublishSyncsOverlapForTest(overlap)()
				s := newLeaseStore(t)
				if route.lower {
					store.LowerWriterCheckpointForTest(t, s.repo)
				}
				id := s.ticket(t, "one")
				rec := newSyncRecorder(t, s.repo.StateDir, -1)
				claim, n := s.timedLease(t, "claim-1", claimOf(id, "src/"), 0)
				label := fmt.Sprintf("overlap=%v claim", overlap)
				checkOrderingEdges(t, label, rec.resolved(), overlap)
				if overlap && rec.maxIn < 2 {
					t.Errorf("%s: at most %d directory syncs ran at once, want the post syncs together", label, rec.maxIn)
				}
				if !overlap && rec.maxIn != 1 {
					t.Errorf("%s: %d directory syncs ran at once without overlap", label, rec.maxIn)
				}
				syncs := []int{n}
				for i, l := range []transaction.LeaseRequest{heartbeatOf(claim), renewOf(claim), releaseOf(claim)} {
					rec.reset()
					_, n := s.timedLease(t, fmt.Sprintf("live-%d", i), l, i+1)
					checkOrderingEdges(t, fmt.Sprintf("overlap=%v step %d", overlap, i), rec.resolved(), overlap)
					syncs = append(syncs, n)
				}
				auditOK(t, s.repo)
				return syncs
			}
			on, off := run(true), run(false)
			if fmt.Sprint(on) != fmt.Sprint(off) {
				t.Fatalf("sync calls with overlap %v, without %v", on, off)
			}
		})
	}
}

// TestLGLV0016_FaultAtEachPublishSyncStopsTheNextStep: failing each
// directory sync of a claim and of a heartbeat in turn, with overlapped
// publish syncs, fails the write, publishes nothing after the failed sync,
// leaves staging empty, and the retried write then completes with a clean
// audit.
func TestLGLV0016_FaultAtEachPublishSyncStopsTheNextStep(t *testing.T) {
	defer store.SetLeasePublishSyncsOverlapForTest(true)()
	for _, verb := range []string{"claim", "heartbeat"} {
		// setup returns the lease request under test on a fresh store.
		setup := func(t *testing.T) (*leaseStore, transaction.LeaseRequest) {
			s := newLeaseStore(t)
			id := s.ticket(t, "one")
			if verb == "claim" {
				return s, claimOf(id, "src/")
			}
			return s, heartbeatOf(s.claim(t, "claim-1", id, 0, "src/"))
		}
		s, l := setup(t)
		count := newSyncRecorder(t, s.repo.StateDir, -1)
		s.lease(t, verb+"-1", l, 1, nil)
		total := count.calls
		if total < 4 {
			t.Fatalf("%s made %d directory syncs", verb, total)
		}
		for k := 0; k < total; k++ {
			t.Run(fmt.Sprintf("%s-sync-%d", verb, k), func(t *testing.T) {
				s, l := setup(t)
				rec := newSyncRecorder(t, s.repo.StateDir, k)
				choice := store.LeaseChoice{QueueID: fixture.QueueID, RequestID: verb + "-1", Root: s.root, Lease: l}
				r, err := store.Lease(context.Background(), s.repo, operator(), choice, s.at(t, 1))
				events := rec.resolved()
				// Every sync failure, including sync 0 that the session's
				// open makes and reports as UNSUPPORTED_FILESYSTEM, wraps
				// the injected cause (V1-2878).
				if err == nil || !errors.Is(err, rec.inject) {
					t.Fatalf("failing sync %d did not fail the %s: %+v %v", k, verb, r, err)
				}
				failed := false
				for _, e := range events {
					if e.kind == "fail" {
						failed = true
					} else if failed && e.kind == "publish" {
						t.Fatalf("%s %s was published after sync %d failed", e.role, e.dir, k)
					}
				}
				if names := dirNames(t, filepath.Join(s.repo.StateDir, "staging")); len(names) != 0 {
					t.Fatalf("staging entries after the failed sync: %v", names)
				}
				rec.failAt = -1
				if r := s.lease(t, verb+"-1", l, 1, nil); r.Outcome.Outcome != mutation.OutcomeCompleted {
					t.Fatalf("the retried %s: %+v", verb, r)
				}
				auditOK(t, s.repo)
			})
		}
	}
}
