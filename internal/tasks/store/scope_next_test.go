package store_test

import (
	"context"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/mutation"
)

func TestCALV0022_NextDerivesOnlySelectedTicket(t *testing.T) {
	t.Parallel()
	t.Run("CAL-V0-022 selected ticket derived scope", func(t *testing.T) {
		s := newLeaseStore(t)
		first := s.ticket(t, "first")
		s.ticket(t, "second")
		calls := 0
		derive := func(_ context.Context, _, _, title, _ string) ([]string, string, bool) {
			calls++
			if title != "first" {
				t.Fatalf("derived %q", title)
			}
			return []string{"selected/"}, strings.Repeat("a", 64), true
		}
		report := s.lease(t, "next-derived", claimNext, 0, derive)
		if report.Outcome.Outcome != mutation.OutcomeCompleted || report.Ticket != first || calls != 1 {
			t.Fatalf("claim: %+v calls=%d", report, calls)
		}
		attempt := s.attempt(t, report.AttemptID)
		if attempt.Scope.Source != "DERIVED" || attempt.Scope.Resources[0].Key != "selected/" {
			t.Fatalf("scope: %+v", attempt.Scope)
		}
	})
}
