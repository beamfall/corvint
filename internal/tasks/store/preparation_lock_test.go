//go:build darwin || linux

package store

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/authority"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func preparationReap(id string) transaction.Request {
	return transaction.Request{Operation: transaction.Lease, QueueID: fixture.QueueID, RequestID: id, Actor: mutation.Binding{ID: "tester", Role: "OWNER"}, Lease: &transaction.LeaseRequest{Verb: transaction.LeaseReap}}
}

// CAL-V0-026: the outer gate suppresses overlapping lease preparation without
// replacing the existing writer lock, snapshot retry, or cancellation boundary.
func TestCALV0026_PreparationGateStore(t *testing.T) {
	const now = wire.Timestamp("2026-09-27T00:00:00Z")
	t.Run("wait-cancellation-before-preparation", func(t *testing.T) {
		repo := cachedLeaseRepo(t)
		holder, err := authority.AcquirePreparation(context.Background(), repo, authority.LockOptions{})
		if err != nil {
			t.Fatal(err)
		}
		defer holder.Close()
		before := fixture.TreeSnapshot(t, repo.StateDir)
		called := false
		ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
		defer cancel()
		facts := func(*journal.Result, *transaction.Input) (transaction.LeaseFacts, error) {
			called = true
			return transaction.LeaseFacts{}, nil
		}
		_, _, err = leaseWrite(ctx, repo, preparationReap("waiting"), now, nil, facts)
		if !errors.Is(err, context.DeadlineExceeded) || called {
			t.Fatalf("waiting writer %v facts=%t", err, called)
		}
		if !reflect.DeepEqual(before, fixture.TreeSnapshot(t, repo.StateDir)) {
			t.Fatal("waiting cancellation changed journal/request/attempt state")
		}
	})
	t.Run("cooperating-preparations-do-not-overlap", func(t *testing.T) {
		repo := cachedLeaseRepo(t)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		entered := make(chan struct{})
		release := make(chan struct{})
		secondEntered := make(chan struct{})
		results := make(chan error, 2)
		var once sync.Once
		var workers sync.WaitGroup
		defer func() { cancel(); once.Do(func() { close(release) }); workers.Wait() }()
		workers.Add(1)
		go func() {
			defer workers.Done()
			_, _, err := leaseWrite(ctx, repo, preparationReap("first"), now, nil, func(*journal.Result, *transaction.Input) (transaction.LeaseFacts, error) {
				close(entered)
				select {
				case <-release:
				case <-ctx.Done():
					return transaction.LeaseFacts{}, ctx.Err()
				}
				return transaction.LeaseFacts{}, nil
			})
			results <- err
		}()
		select {
		case <-entered:
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
		workers.Add(1)
		go func() {
			defer workers.Done()
			_, _, err := leaseWrite(ctx, repo, preparationReap("second"), now, nil, func(*journal.Result, *transaction.Input) (transaction.LeaseFacts, error) {
				close(secondEntered)
				return transaction.LeaseFacts{}, nil
			})
			results <- err
		}()
		select {
		case <-secondEntered:
			t.Fatal("second prepared while first held admission")
		case <-time.After(40 * time.Millisecond):
		}
		once.Do(func() { close(release) })
		for i := 0; i < 2; i++ {
			select {
			case err := <-results:
				if err != nil {
					t.Fatal(err)
				}
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
		}
		select {
		case <-secondEntered:
		default:
			t.Fatal("second never prepared")
		}
	})
	t.Run("cancel-after-admission-before-commit", func(t *testing.T) {
		repo := cachedLeaseRepo(t)
		before := fixture.TreeSnapshot(t, repo.StateDir)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		called := false
		_, _, err := leaseWrite(ctx, repo, preparationReap("precommit-cancel"), now, nil, func(*journal.Result, *transaction.Input) (transaction.LeaseFacts, error) {
			called = true
			cancel()
			return transaction.LeaseFacts{}, nil
		})
		if !called || !errors.Is(err, context.Canceled) {
			t.Fatalf("cancel %v called=%t", err, called)
		}
		if !reflect.DeepEqual(before, fixture.TreeSnapshot(t, repo.StateDir)) {
			t.Fatal("precommit cancellation changed journal/request/attempt state")
		}
		next, err := authority.AcquirePreparation(context.Background(), repo, authority.LockOptions{Wait: time.Second})
		if err != nil {
			t.Fatal(err)
		}
		if err := next.Close(); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("ordinary-writer-still-invalidates-preparation", func(t *testing.T) {
		repo := cachedLeaseRepo(t)
		var calls atomic.Int32
		facts := func(*journal.Result, *transaction.Input) (transaction.LeaseFacts, error) {
			if calls.Add(1) == 1 {
				policy := fixture.PolicyValue()
				policy.Obj.Set("policyVersion", wire.String("2"))
				report, err := PolicyUpdate(context.Background(), repo, mutation.Binding{ID: "tester", Role: "OWNER"}, PolicyRequest{QueueID: fixture.QueueID, RequestID: "mixed-policy", ExpectedPolicyVersion: "1", Policy: wire.EncodeFile(policy)}, now)
				if err != nil {
					return transaction.LeaseFacts{}, err
				}
				if report.Outcome.Outcome != mutation.OutcomeCompleted {
					return transaction.LeaseFacts{}, errors.New("policy update did not commit")
				}
			}
			return transaction.LeaseFacts{}, nil
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, _, err := leaseWrite(ctx, repo, preparationReap("mixed"), now, nil, facts); err != nil {
			t.Fatal(err)
		}
		if calls.Load() != 2 {
			t.Fatalf("expected stale observation retry, facts=%d", calls.Load())
		}
		raw, _, _, err := journalBytes(repo)
		if err != nil {
			t.Fatal(err)
		}
		head, err := snapshot.DecodeHead(raw)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := journalReader(repo, head).Audit(); err != nil {
			t.Fatal(err)
		}
	})
}

// GH494: preparation admission outlives monitor teardown even on a refused
// preparation, and cleanup failures remain terminal without retaining ownership.
func TestGH494PreparationAdmissionStore(t *testing.T) {
	for _, kind := range []string{"refused", "guard-error", "preparation-error"} {
		t.Run(kind, func(t *testing.T) {
			repo := cachedLeaseRepo(t)
			audit := func() {
				raw, _, _, e := journalBytes(repo)
				if e != nil {
					t.Fatal(e)
				}
				head, e := snapshot.DecodeHead(raw)
				if e != nil {
					t.Fatal(e)
				}
				a, e := journalReader(repo, head).Audit()
				if e != nil || a == nil || a.StructuralConsistency != "CONSISTENT" || a.ProjectionAgreement != "AGREES" {
					t.Fatalf("canonical audit %+v %v", a, e)
				}
			}
			audit()
			before := fixture.TreeSnapshot(t, repo.StateDir)
			failure := errors.New("reached fact refusal")
			cleanup := errors.New("reached teardown failure")
			guardClosed, prepClosed, observed, factsReached := false, false, false, false
			hooks := hooksForInventory(context.Background())
			hooks.closeGuard = func(g *authority.ChangeGuard) error {
				e := g.Close()
				guardClosed = true
				if kind == "guard-error" {
					e = errors.Join(e, cleanup)
				}
				return e
			}
			hooks.closePreparation = func(p *authority.PreparationLock) error {
				if !guardClosed {
					t.Error("preparation retired before monitor")
				}
				prepClosed = true
				e := p.Close()
				if kind == "preparation-error" {
					e = errors.Join(e, cleanup)
				}
				return e
			}
			ctx := context.WithValue(context.Background(), inventoryHooksKey{}, hooks)
			ctx = authority.WithPreparationObserver(ctx, func(o authority.PreparationObservation) {
				observed = true
				if !guardClosed || !o.Acquired || !o.Released || o.Err != nil {
					t.Errorf("observer before retirement or inaccurate state: %+v", o)
				}
			})
			report, _, e := leaseWrite(ctx, repo, preparationReap("fair-"+kind), wire.Timestamp("2026-09-27T00:00:00Z"), nil, func(*journal.Result, *transaction.Input) (transaction.LeaseFacts, error) {
				factsReached = true
				return transaction.LeaseFacts{}, failure
			})
			if !factsReached || !guardClosed || !prepClosed || !observed || !errors.Is(e, failure) {
				t.Fatalf("reached/lifecycle flags %t %t %t %t error%v", factsReached, guardClosed, prepClosed, observed, e)
			}
			if kind != "refused" && !errors.Is(e, cleanup) {
				t.Fatal("cleanup failure lost", e)
			}
			if report.Receipt != "" || !reflect.DeepEqual(before, fixture.TreeSnapshot(t, repo.StateDir)) {
				t.Fatal("refusal/cleanup changed journal")
			}
			next, e := authority.AcquirePreparation(context.Background(), repo, authority.LockOptions{Wait: time.Second})
			if e != nil {
				t.Fatal("retained preparation", e)
			}
			if e = next.Close(); e != nil {
				t.Fatal(e)
			}
			audit()
		})
	}
}
