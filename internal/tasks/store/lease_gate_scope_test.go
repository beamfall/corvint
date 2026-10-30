//go:build darwin || linux

package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/authority"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// gateProbe returns ctx with an audit hook that, while each complete audit
// of a lease preparation runs, tries the preparation gate. A writer that
// holds the gate across the audit leaves the probe waiting to LOCK_TIMEOUT,
// which the returned counters record as held.
func gateProbe(t *testing.T, repo *intent.Repository) (ctx context.Context, audits, held *int) {
	t.Helper()
	audits, held = new(int), new(int)
	hooks := hooksForInventory(context.Background())
	audit := hooks.audit
	hooks.audit = func(r journal.Reader) (*journal.Result, journal.PhysicalObservation, error) {
		*audits++
		probe, err := authority.AcquirePreparation(context.Background(), repo, authority.LockOptions{Wait: 2 * time.Second})
		if err != nil {
			if wire.CodeOf(err) != wire.CodeLockTimeout {
				t.Errorf("gate probe: %v", err)
			}
			*held++
			t.Logf("held: %v", err)
		} else if err := probe.Close(); err != nil {
			t.Errorf("gate probe close: %v", err)
		}
		return audit(r)
	}
	return context.WithValue(context.Background(), inventoryHooksKey{}, hooks), audits, held
}

// CAL-V0-263 (proposed): no lease write runs a complete-history audit while
// it holds the preparation gate, whether the audit leads to a committed
// write or to a refusal; the refusal is decided before the gate. Moving the
// preparation back inside the gate fails every case here.
func TestCALV0263_NoCompleteAuditUnderGate(t *testing.T) {
	t.Run("planned-claim", func(t *testing.T) {
		repo, choice := inventoryClaimRepo(t)
		ctx, audits, held := gateProbe(t, repo)
		entered := 0
		ctx = authority.WithPreparationObserver(ctx, func(authority.PreparationObservation) { entered++ })
		report, err := Lease(ctx, repo, mutation.Binding{ID: "tester", Role: "OWNER"}, choice, fixture.Timestamp)
		if err != nil || report.Receipt == "" {
			t.Fatalf("claim: %+v %v", report, err)
		}
		if *audits == 0 || *held != 0 || entered != 1 {
			t.Fatalf("audits %d, %d under the gate, gate entered %d times", *audits, *held, entered)
		}
	})
	t.Run("refusal", func(t *testing.T) {
		repo := cachedLeaseRepo(t)
		ctx, audits, held := gateProbe(t, repo)
		entered := 0
		ctx = authority.WithPreparationObserver(ctx, func(authority.PreparationObservation) { entered++ })
		refusal := errors.New("no eligible member")
		_, _, err := leaseWrite(ctx, repo, preparationReap("gate-scope-refusal"), fixture.Timestamp, nil, func(*journal.Result, *transaction.Input) (transaction.LeaseFacts, error) {
			return transaction.LeaseFacts{}, refusal
		})
		if !errors.Is(err, refusal) {
			t.Fatalf("refusal: %v", err)
		}
		if *audits == 0 || *held != 0 || entered != 0 {
			t.Fatalf("audits %d, %d under the gate, gate entered %d times", *audits, *held, entered)
		}
	})
	t.Run("no-op-reap", func(t *testing.T) {
		repo := cachedLeaseRepo(t)
		ctx, audits, held := gateProbe(t, repo)
		if _, _, err := leaseWrite(ctx, repo, preparationReap("gate-scope-reap"), fixture.Timestamp, nil, nil); err != nil {
			t.Fatal(err)
		}
		if *audits == 0 || *held != 0 {
			t.Fatalf("audits %d, %d under the gate", *audits, *held)
		}
	})
}
