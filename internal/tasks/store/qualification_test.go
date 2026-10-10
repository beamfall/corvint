package store_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// The CAL-V0-019 suite. Its test names are the run that a QUALIFICATION
// receipt names (CAL-V0-020), so renaming one is a contract change.

// try runs one lease verb without failing the test, for racing goroutines
// and injected faults.
func (s *leaseStore) try(requestID string, l transaction.LeaseRequest, at wire.Timestamp) (*store.Report, error) {
	choice := store.LeaseChoice{QueueID: fixture.QueueID, RequestID: requestID, Root: s.root, Lease: l}
	return store.Lease(context.Background(), s.repo, operator(), choice, at)
}

// tryGate runs one gate with the clock fixed at at.
func (s *leaseStore) tryGate(requestID string, l transaction.LeaseRequest, at wire.Timestamp) (*store.Report, error) {
	clock, err := time.Parse("2006-01-02T15:04:05Z", string(at))
	if err != nil {
		return nil, err
	}
	choice := store.LeaseChoice{QueueID: fixture.QueueID, RequestID: requestID, Root: s.root, Lease: l}
	return store.GateRun(context.Background(), s.repo, operator(), choice, s.root, func() time.Time { return clock })
}

// race runs every call at once and returns their reports in order; a call
// that errors fails the test.
func race(t *testing.T, calls ...func() (*store.Report, error)) []*store.Report {
	t.Helper()
	reports := make([]*store.Report, len(calls))
	errs := make([]error, len(calls))
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i, call := range calls {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			reports[i], errs[i] = call()
		}()
	}
	close(start)
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("racer %d: %v", i, err)
		}
	}
	return reports
}

// consistent audits the store and checks that every reservation entry
// belongs to an attempt that is still unfenced.
func (s *leaseStore) consistent(t *testing.T) {
	t.Helper()
	auditOK(t, s.repo)
	for _, e := range s.entries(t) {
		if a := s.attempt(t, e.AttemptID); a.Quiescence == "FENCED" {
			t.Fatalf("entry %+v holds for fenced attempt %+v", e, a)
		}
	}
}

// TestCALV0019_ConcurrentCollidingClaimsAdmitOne: eight claims of eight
// tickets over one path, sent at once, admit exactly one; the rest refuse
// RESOURCE_COLLISION. Two claims of one ticket admit one too.
func TestCALV0019_ConcurrentCollidingClaimsAdmitOne(t *testing.T) {
	t.Parallel()
	t.Run("CAL-V0-019 ConcurrentCollidingClaimsAdmitOne", func(t *testing.T) {
		s := newLeaseStore(t)
		ids := []string{}
		for i := range 8 {
			ids = append(ids, s.ticket(t, "t"+strconv.Itoa(i)))
		}
		calls := []func() (*store.Report, error){}
		for i, id := range ids {
			calls = append(calls, func() (*store.Report, error) { return s.try("claim-"+strconv.Itoa(i), claimOf(id, "src/"), s.at(t, 0)) })
		}
		admitted := 0
		for _, r := range race(t, calls...) {
			switch {
			case r.Outcome.Outcome == mutation.OutcomeCompleted:
				admitted++
			case !has(r.Outcome.Codes, wire.CodeResourceCollision):
				t.Fatalf("unexpected refusal: %+v", r)
			}
		}
		if admitted != 1 || len(s.entries(t)) != 1 {
			t.Fatalf("admitted %d, entries %d", admitted, len(s.entries(t)))
		}
		one := s.ticket(t, "same")
		same := func(req string) func() (*store.Report, error) {
			return func() (*store.Report, error) { return s.try(req, claimOf(one, "docs/"), s.at(t, 0)) }
		}
		admitted = 0
		for _, r := range race(t, same("same-1"), same("same-2")) {
			if r.Outcome.Outcome == mutation.OutcomeCompleted {
				admitted++
			}
		}
		if admitted != 1 {
			t.Fatalf("one ticket admitted %d claims", admitted)
		}
		s.consistent(t)
	})
}

// TestCALV0019_RacingLeaseVerbsLeaveOneConsistentHead: claim, renew, reap,
// widen, release, submit, gate run and complete over four attempts, sent at
// once, each commit or refuse, and the store audits CONSISTENT with no
// reservation left to a fenced attempt.
func TestCALV0019_RacingLeaseVerbsLeaveOneConsistentHead(t *testing.T) {
	t.Parallel()
	t.Run("CAL-V0-019 RacingLeaseVerbsLeaveOneConsistentHead", func(t *testing.T) {
		s := newGateStore(t)
		a, b, c, d := s.ticket(t, "a"), s.ticket(t, "b"), s.ticket(t, "c"), s.ticket(t, "d")
		built, commit := s.submitted(t, a, "src", 0)
		tree := gitOut(t, s.root, "rev-parse", "HEAD^{tree}")
		short := claimOf(b, "docs/")
		short.LeaseMinutes = "5"
		s.lease(t, "claim-b", short, 1, nil)
		live := s.claim(t, "claim-d", d, 1, "web/")
		at := s.at(t, 10)
		widen := transaction.LeaseRequest{Verb: transaction.LeaseWiden, AttemptID: live.AttemptID, Generation: live.Generation, Scope: []string{"web2/"}}
		race(t,
			func() (*store.Report, error) { return s.try("renew-a", renewOf(built), at) },
			func() (*store.Report, error) { return s.try("submit-a", submitOf(built, tree), at) },
			func() (*store.Report, error) { return s.tryGate("gate-a", gateOf(built, "verify"), at) },
			func() (*store.Report, error) { return s.try("complete-a", completeOf(built, commit), at) },
			func() (*store.Report, error) {
				return s.try("reap-all", transaction.LeaseRequest{Verb: transaction.LeaseReap}, at)
			},
			func() (*store.Report, error) { return s.try("claim-c", claimOf(c, "docs/"), at) },
			func() (*store.Report, error) { return s.try("renew-d", renewOf(live), at) },
			func() (*store.Report, error) { return s.try("widen-d", widen, at) },
			func() (*store.Report, error) { return s.try("release-d", releaseOf(live), at) },
		)
		s.consistent(t)
	})
}

// TestCALV0019_FencedGenerationCannotMoveOrComplete: once a generation is
// released or reaped, and on a live attempt named by a wrong generation,
// every verb refuses FENCED and the attempt file keeps its bytes.
func TestCALV0019_FencedGenerationCannotMoveOrComplete(t *testing.T) {
	t.Parallel()
	t.Run("CAL-V0-019 FencedGenerationCannotMoveOrComplete", func(t *testing.T) {
		s := newGateStore(t)
		one, two, three := s.ticket(t, "one"), s.ticket(t, "two"), s.ticket(t, "three")
		released, commit := s.submitted(t, one, "src", 0)
		tree := gitOut(t, s.root, "rev-parse", "HEAD^{tree}")
		s.passes(t, "gate-1", gateOf(released, "verify"), 2)
		if r := s.lease(t, "release-1", releaseOf(released), 3, nil); r.Outcome.Outcome != mutation.OutcomeCompleted {
			t.Fatalf("release: %+v", r)
		}
		short := claimOf(two, "docs/")
		short.LeaseMinutes = "5"
		reaped := s.lease(t, "claim-2", short, 3, nil)
		wrong := s.claim(t, "claim-3", three, 4, "lib/")
		if r := s.lease(t, "reap-2", transaction.LeaseRequest{Verb: transaction.LeaseReap, AttemptID: reaped.AttemptID, Generation: reaped.Generation}, 10, nil); r.Kind != "Transaction" {
			t.Fatalf("reap: %+v", r)
		}
		wrong.Generation = wire.SizeOf(wrong.Generation.Uint64() + 1)
		for _, fenced := range []*store.Report{released, reaped, wrong} {
			file := filepath.Join(s.repo.StateDir, "attempts", fenced.AttemptID+".json")
			before, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			widen := transaction.LeaseRequest{Verb: transaction.LeaseWiden, AttemptID: fenced.AttemptID, Generation: fenced.Generation, Scope: []string{"other/"}}
			verbs := map[string]transaction.LeaseRequest{
				"renew": renewOf(fenced), "widen": widen, "submit": submitOf(fenced, tree),
				"complete": completeOf(fenced, commit), "release": releaseOf(fenced),
			}
			for name, l := range verbs {
				r, err := s.try(name+"-"+fenced.AttemptID, l, s.at(t, 20))
				if err != nil {
					t.Fatalf("%s: %v", name, err)
				}
				refusedWith(t, r, mutation.OutcomeRevisionConflict, wire.CodeFenced)
			}
			r, err := s.tryGate("gate-"+fenced.AttemptID, gateOf(fenced, "verify"), s.at(t, 20))
			if err != nil {
				t.Fatalf("gate: %v", err)
			}
			refusedWith(t, r, mutation.OutcomeRevisionConflict, wire.CodeFenced)
			if after, _ := os.ReadFile(file); !bytes.Equal(before, after) {
				t.Fatalf("fenced verbs changed %s", fenced.AttemptID)
			}
		}
		s.consistent(t)
	})
}

// verbCase builds a fresh store up to one lease verb and returns the call
// that runs it, so a fault can be injected before each of its artifacts.
type verbCase struct {
	name  string
	setup func(t *testing.T, s *leaseStore) func(requestID string) (*store.Report, error)
}

var leaseVerbs = []verbCase{
	{"claim", func(t *testing.T, s *leaseStore) func(string) (*store.Report, error) {
		id := s.ticket(t, "one")
		return func(req string) (*store.Report, error) { return s.try(req, claimOf(id, "src/"), s.at(t, 0)) }
	}},
	{"claim-next", func(t *testing.T, s *leaseStore) func(string) (*store.Report, error) {
		s.planned(t, "one", "P1", "src/")
		return func(req string) (*store.Report, error) { return s.try(req, claimNext, s.at(t, 0)) }
	}},
	{"renew", func(t *testing.T, s *leaseStore) func(string) (*store.Report, error) {
		c := s.claim(t, "claim-1", s.ticket(t, "one"), 0, "src/")
		return func(req string) (*store.Report, error) { return s.try(req, renewOf(c), s.at(t, 1)) }
	}},
	{"widen", func(t *testing.T, s *leaseStore) func(string) (*store.Report, error) {
		c := s.claim(t, "claim-1", s.ticket(t, "one"), 0, "src/")
		l := transaction.LeaseRequest{Verb: transaction.LeaseWiden, AttemptID: c.AttemptID, Generation: c.Generation, Scope: []string{"docs/"}}
		return func(req string) (*store.Report, error) { return s.try(req, l, s.at(t, 1)) }
	}},
	{"release", func(t *testing.T, s *leaseStore) func(string) (*store.Report, error) {
		c := s.claim(t, "claim-1", s.ticket(t, "one"), 0, "src/")
		return func(req string) (*store.Report, error) { return s.try(req, releaseOf(c), s.at(t, 1)) }
	}},
	{"reap", func(t *testing.T, s *leaseStore) func(string) (*store.Report, error) {
		l := claimOf(s.ticket(t, "one"), "src/")
		l.LeaseMinutes = "5"
		c := s.lease(t, "claim-1", l, 0, nil)
		reap := transaction.LeaseRequest{Verb: transaction.LeaseReap, AttemptID: c.AttemptID, Generation: c.Generation}
		return func(req string) (*store.Report, error) { return s.try(req, reap, s.at(t, 10)) }
	}},
	{"submit", func(t *testing.T, s *leaseStore) func(string) (*store.Report, error) {
		c := s.claim(t, "claim-1", s.ticket(t, "one"), 0, "src/")
		_, tree := s.commit(t, "src/a.go")
		return func(req string) (*store.Report, error) { return s.try(req, submitOf(c, tree), s.at(t, 1)) }
	}},
	{"gate-run", func(t *testing.T, s *leaseStore) func(string) (*store.Report, error) {
		c, _ := s.submitted(t, s.ticket(t, "one"), "src", 0)
		return func(req string) (*store.Report, error) { return s.tryGate(req, gateOf(c, "verify"), s.at(t, 2)) }
	}},
	{"complete", func(t *testing.T, s *leaseStore) func(string) (*store.Report, error) {
		c, commit := s.submitted(t, s.ticket(t, "one"), "src", 0)
		s.passes(t, "gate-1", gateOf(c, "verify"), 2)
		return func(req string) (*store.Report, error) { return s.try(req, completeOf(c, commit), s.at(t, 3)) }
	}},
}

var errInjected = errors.New("injected publish fault")

// journalState is the head bytes and the receipt count.
func journalState(t *testing.T, repo *intent.Repository) (string, int) {
	t.Helper()
	head, err := os.ReadFile(filepath.Join(repo.StateDir, "head.json"))
	if err != nil {
		t.Fatal(err)
	}
	receipts, err := os.ReadDir(filepath.Join(repo.StateDir, "receipts"))
	if err != nil {
		t.Fatal(err)
	}
	return string(head), len(receipts)
}

// roles runs one verb unfaulted and returns the roles of its artifacts in
// publication order.
func roles(t *testing.T, v verbCase) []string {
	t.Helper()
	s := newGateStore(t)
	run := v.setup(t, s)
	out := []string{}
	defer store.SetPublishFaultForTest(func(a transaction.Artifact) error {
		out = append(out, a.Role)
		return nil
	})()
	if r, err := run("verb"); err != nil || r.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("%s: %+v %v", v.name, r, err)
	}
	return out
}

// TestCALV0019_FaultAtEveryArtifactIsAllOrNothing: for every lease verb and
// every artifact it publishes, a failure just before that artifact leaves
// the head where it was; before the receipt no receipt exists and the retry
// commits afresh, and from the receipt on the next writer redoes it and the
// retry replays it (the §5.3 crash matrix, lease rows).
func TestCALV0019_FaultAtEveryArtifactIsAllOrNothing(t *testing.T) {
	t.Run("CAL-V0-019 FaultAtEveryArtifactIsAllOrNothing", func(t *testing.T) {
		for _, v := range leaseVerbs {
			t.Run(v.name, func(t *testing.T) {
				order := roles(t, v)
				receiptAt := -1
				for i, role := range order {
					if role == "RECEIPT" {
						receiptAt = i
					}
				}
				for k := range order {
					s := newGateStore(t)
					run := v.setup(t, s)
					head, count := journalState(t, s.repo)
					n := 0
					restore := store.SetPublishFaultForTest(func(transaction.Artifact) error {
						if n++; n > k {
							return errInjected
						}
						return nil
					})
					_, err := run("verb")
					restore()
					if !errors.Is(err, errInjected) {
						t.Fatalf("fault %d (%s): %v", k, order[k], err)
					}
					gotHead, gotCount := journalState(t, s.repo)
					committed := k > receiptAt
					if gotHead != head || gotCount != count+btoi(committed) {
						t.Fatalf("fault %d (%s): head moved or receipts %d -> %d", k, order[k], count, gotCount)
					}
					r, err := run("verb")
					if err != nil || r.Outcome.Outcome != mutation.OutcomeCompleted || r.Redone != committed || replayed(r.Kind) != committed {
						t.Fatalf("fault %d (%s) retry: %+v %v", k, order[k], r, err)
					}
					s.consistent(t)
				}
			})
		}
	})
}

func btoi(b bool) int {
	if b {
		return 1
	}
	return 0
}

// killCall is the verb the kill test's child runs: a claim, whose receipt is
// its first artifact, or a gate run, which publishes evidence before its
// receipt.
func killCall(s *leaseStore, verb, target, generation string, at wire.Timestamp) (*store.Report, error) {
	if verb == "gate-run" {
		return s.tryGate("verb", transaction.LeaseRequest{Verb: transaction.LeaseGateRun, AttemptID: target, Generation: wire.Size(generation), Gate: "verify"}, at)
	}
	return s.try("verb", claimOf(target, "src/"), at)
}

// TestCALV0019KillChild is the process the kill test re-executes: it runs one
// verb against the store the parent built and exits 7 just before artifact
// KILL_AT, as a crash there would. It is skipped when run directly.
func TestCALV0019KillChild(t *testing.T) {
	root := os.Getenv("KILL_REPO")
	if root == "" {
		t.Skip("run only by TestCALV0019_KilledWriterRecovers")
	}
	k, _ := strconv.Atoi(os.Getenv("KILL_AT"))
	repo, err := intent.Resolve(root)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	store.SetPublishFaultForTest(func(transaction.Artifact) error {
		if n++; n > k {
			os.Exit(7)
		}
		return nil
	})
	s := &leaseStore{repo: repo, root: os.Getenv("KILL_GIT")}
	if _, err := killCall(s, os.Getenv("KILL_VERB"), os.Getenv("KILL_TARGET"), os.Getenv("KILL_GEN"), wire.Timestamp(os.Getenv("KILL_T"))); err != nil {
		t.Fatal(err)
	}
}

// TestCALV0019_KilledWriterRecovers: a writer process killed just before
// each artifact of a claim and of a gate run leaves staging slots behind;
// the retry, in a new process, clears them, completes the transaction and
// audits CONSISTENT, and a later write commits.
func TestCALV0019_KilledWriterRecovers(t *testing.T) {
	t.Parallel()
	t.Run("CAL-V0-019 KilledWriterRecovers", func(t *testing.T) {
		for _, verb := range []string{"claim", "gate-run"} {
			t.Run(verb, func(t *testing.T) {
				for k := 0; ; k++ {
					s := newGateStore(t)
					target, generation, at := s.ticket(t, "one"), "", s.at(t, 0)
					if verb == "gate-run" {
						c, _ := s.submitted(t, target, "src", 0)
						target, generation, at = c.AttemptID, string(c.Generation), s.at(t, 2)
					}
					child := exec.Command(os.Args[0], "-test.run=^TestCALV0019KillChild$", "-test.count=1")
					child.Env = append(os.Environ(), "KILL_REPO="+s.repo.PrimaryWorktree, "KILL_GIT="+s.root, "KILL_AT="+strconv.Itoa(k),
						"KILL_VERB="+verb, "KILL_TARGET="+target, "KILL_GEN="+generation, "KILL_T="+string(at))
					out, err := child.CombinedOutput()
					var exit *exec.ExitError
					if err == nil {
						if k == 0 {
							t.Fatal("the child published nothing")
						}
						return
					}
					if !errors.As(err, &exit) || exit.ExitCode() != 7 {
						t.Fatalf("child at %d: %v\n%s", k, err, out)
					}
					r, err := killCall(s, verb, target, generation, at)
					if err != nil || r.Outcome.Outcome != mutation.OutcomeCompleted {
						t.Fatalf("retry after kill at %d: %+v %v", k, r, err)
					}
					s.consistent(t)
					later, err := store.Mutate(context.Background(), s.repo, operator(), envelope("after", "CREATE", "", "", createPayload("after")), s.at(t, 60))
					if err != nil || later.Outcome.Outcome != mutation.OutcomeCompleted {
						t.Fatalf("write after kill at %d: %+v %v", k, later, err)
					}
				}
			})
		}
	})
}

// replayed reports a retry answered from the committed receipt. A reap
// finds the attempt already fenced once its receipt is redone, so it answers
// NoChange rather than Replay.
func replayed(kind string) bool {
	return kind == "Replay" || kind == "NoChange"
}
