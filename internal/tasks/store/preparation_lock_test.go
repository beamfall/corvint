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
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func preparationReap(id string) transaction.Request {
	return transaction.Request{Operation: transaction.Lease, QueueID: fixture.QueueID, RequestID: id, Actor: mutation.Binding{ID: "tester", Role: "OWNER"}, Lease: &transaction.LeaseRequest{Verb: transaction.LeaseReap}}
}

// CAL-V0-026, CAL-V0-263: the outer gate serializes cooperating lease
// writers' bounded commits without replacing the existing writer lock,
// snapshot retry, or cancellation boundary. Preparation, including the
// complete audit and the model, runs before the gate.
func TestCALV0026_PreparationGateStore(t *testing.T) {
	const now = wire.Timestamp("2026-09-27T00:00:00Z")
	t.Run("held-gate-waits-only-at-commit", func(t *testing.T) {
		repo, choice := inventoryClaimRepo(t)
		holder, err := authority.AcquirePreparation(context.Background(), repo, authority.LockOptions{})
		if err != nil {
			t.Fatal(err)
		}
		defer holder.Close()
		before := fixture.TreeSnapshot(t, repo.StateDir)
		// The deadline bounds a gate wrapped around preparation again.
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		audits, swept, acquires := 0, false, 0
		hooks := hooksForInventory(context.Background())
		audit := hooks.audit
		hooks.audit = func(r journal.Reader) (*journal.Result, journal.PhysicalObservation, error) {
			audits++
			return audit(r)
		}
		hooks.commitStage = func(stage string) {
			if stage == "sweep" {
				// Prepared while another writer holds the gate; the
				// planned commit now waits for it and is cancelled there.
				swept = true
				cancel()
			}
		}
		ctx = context.WithValue(ctx, inventoryHooksKey{}, hooks)
		ctx = authority.WithLockObserver(ctx, func(time.Duration) { acquires++ })
		report, err := Lease(ctx, repo, mutation.Binding{ID: "tester", Role: "OWNER"}, choice, fixture.Timestamp)
		if !errors.Is(err, context.Canceled) || audits == 0 || !swept || acquires != 0 || report.Receipt != "" {
			t.Fatalf("waiting writer %v audits=%d swept=%t acquires=%d", err, audits, swept, acquires)
		}
		if !reflect.DeepEqual(before, fixture.TreeSnapshot(t, repo.StateDir)) {
			t.Fatal("waiting cancellation changed journal/request/attempt state")
		}
	})
	t.Run("cooperating-preparations-overlap", func(t *testing.T) {
		repo := cachedLeaseRepo(t)
		// A gate around preparation deadlocks here until the deadline: the
		// first preparation, inside its facts, waits for the second to
		// reach its own.
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		firstEntered := make(chan struct{})
		secondEntered := make(chan struct{})
		results := make(chan error, 2)
		var workers sync.WaitGroup
		defer func() { cancel(); workers.Wait() }()
		workers.Add(1)
		go func() {
			defer workers.Done()
			var once sync.Once
			_, _, err := leaseWrite(ctx, repo, preparationReap("first"), now, nil, func(*journal.Result, *transaction.Input) (transaction.LeaseFacts, error) {
				once.Do(func() { close(firstEntered) })
				select {
				case <-secondEntered:
				case <-ctx.Done():
					return transaction.LeaseFacts{}, ctx.Err()
				}
				return transaction.LeaseFacts{}, nil
			})
			results <- err
		}()
		select {
		case <-firstEntered:
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
		workers.Add(1)
		go func() {
			defer workers.Done()
			var once sync.Once
			_, _, err := leaseWrite(ctx, repo, preparationReap("second"), now, nil, func(*journal.Result, *transaction.Input) (transaction.LeaseFacts, error) {
				once.Do(func() { close(secondEntered) })
				return transaction.LeaseFacts{}, nil
			})
			results <- err
		}()
		for i := 0; i < 2; i++ {
			if err := <-results; err != nil {
				t.Fatal(err)
			}
		}
	})
	t.Run("cancel-after-preparation-before-commit", func(t *testing.T) {
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

// GH494, CAL-V0-263: a refused preparation never enters the gate; a planned
// commit's admission outlives monitor teardown, and cleanup failures remain
// terminal without retaining ownership.
func TestGH494PreparationAdmissionStore(t *testing.T) {
	canonical := func(t *testing.T, repo *intent.Repository) {
		t.Helper()
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
	gateFree := func(t *testing.T, repo *intent.Repository) {
		t.Helper()
		next, e := authority.AcquirePreparation(context.Background(), repo, authority.LockOptions{Wait: time.Second})
		if e != nil {
			t.Fatal("retained preparation", e)
		}
		if e = next.Close(); e != nil {
			t.Fatal(e)
		}
	}
	for _, kind := range []string{"refused", "guard-error", "preparation-error"} {
		t.Run(kind, func(t *testing.T) {
			var repo *intent.Repository
			var choice LeaseChoice
			if kind == "preparation-error" {
				repo, choice = inventoryClaimRepo(t)
			} else {
				repo = cachedLeaseRepo(t)
			}
			canonical(t, repo)
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
			if kind == "preparation-error" {
				// The planned claim commits under the gate; the gate's close
				// failure is still reported and ownership is not retained.
				report, e := Lease(ctx, repo, mutation.Binding{ID: "tester", Role: "OWNER"}, choice, fixture.Timestamp)
				if !guardClosed || !prepClosed || !observed || !errors.Is(e, cleanup) || report.Receipt == "" {
					t.Fatalf("lifecycle flags %t %t %t receipt %q error %v", guardClosed, prepClosed, observed, report.Receipt, e)
				}
				gateFree(t, repo)
				canonical(t, repo)
				return
			}
			report, _, e := leaseWrite(ctx, repo, preparationReap("fair-"+kind), wire.Timestamp("2026-09-27T00:00:00Z"), nil, func(*journal.Result, *transaction.Input) (transaction.LeaseFacts, error) {
				factsReached = true
				return transaction.LeaseFacts{}, failure
			})
			// The refusal is decided while preparing and reported without
			// entering the gate (CAL-V0-263).
			if !factsReached || !guardClosed || prepClosed || observed || !errors.Is(e, failure) {
				t.Fatalf("reached/lifecycle flags %t %t %t %t error%v", factsReached, guardClosed, prepClosed, observed, e)
			}
			if kind == "guard-error" && !errors.Is(e, cleanup) {
				t.Fatal("cleanup failure lost", e)
			}
			if report.Receipt != "" || !reflect.DeepEqual(before, fixture.TreeSnapshot(t, repo.StateDir)) {
				t.Fatal("refusal/cleanup changed journal")
			}
			gateFree(t, repo)
			canonical(t, repo)
		})
	}
}
