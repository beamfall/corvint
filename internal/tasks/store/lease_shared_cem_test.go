package store_test

import (
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestNTPV0026_SharedCEMPathsAdmitIsolatedLanes reproduces the V1-0549
// serialization: with free capacity, a second isolated lane whose scope
// lists the mandatory dogfood CEM path `.corvint/change.cem.json` and the
// seal directory `.corvint/changes/` is refused RESOURCE_COLLISION. With
// those paths and the generated spec index declared in
// `reservations.sharedPaths` it is admitted, while a hand-authored path and
// the uncovered `.corvint/` directory key still collide.
func TestNTPV0026_SharedCEMPathsAdmitIsolatedLanes(t *testing.T) {
	t.Parallel()
	cem := []string{".corvint/change.cem.json", ".corvint/changes/"}
	shared := append(append([]string{}, cem...), generatedSpecIndex...)
	lane := func(own string) []string {
		return append(append([]string{}, cem...), "docs/specs/REQUIREMENTS.tsv", own)
	}

	t.Run("undeclared CEM paths serialize lanes", func(t *testing.T) {
		s := sharedLeaseStore(t, nil)
		first, second := s.ticket(t, "first"), s.ticket(t, "second")
		s.claim(t, "claim-first", first, 0, lane("src/a.go")...)
		refusedWith(t, s.lease(t, "claim-second", claimOf(second, lane("src/b.go")...), 1, nil), mutation.OutcomeBlocked, wire.CodeResourceCollision)
		auditOK(t, s.repo)
	})

	t.Run("declared CEM paths admit a second lane", func(t *testing.T) {
		s := sharedLeaseStore(t, shared)
		first, second := s.ticket(t, "first"), s.ticket(t, "second")
		s.claim(t, "claim-first", first, 0, lane("src/a.go")...)
		s.claim(t, "claim-second", second, 1, lane("src/b.go")...)
		auditOK(t, s.repo)
	})

	t.Run("uncovered keys keep exclusive semantics", func(t *testing.T) {
		s := sharedLeaseStore(t, shared)
		first, second, third := s.ticket(t, "first"), s.ticket(t, "second"), s.ticket(t, "third")
		s.claim(t, "claim-first", first, 0, lane("src/a.go")...)
		refusedWith(t, s.lease(t, "claim-second", claimOf(second, lane("src/a.go")...), 1, nil), mutation.OutcomeBlocked, wire.CodeResourceCollision)
		refusedWith(t, s.lease(t, "claim-third", claimOf(third, ".corvint/"), 2, nil), mutation.OutcomeBlocked, wire.CodeResourceCollision)
		auditOK(t, s.repo)
	})
}
