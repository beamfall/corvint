package mutation_test

import (
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
	"strings"
	"testing"
)

func TestIssue503_ReopenReasonDoesNotAuthorize(t *testing.T) {
	t.Run("CAL-V0-043 reason retains refusal", func(t *testing.T) {
		for _, mismatched := range []bool{false, true} {
			rec := fixture.Ticket("AT-01")
			ctx := newCtx(t, owner, nil, rec)
			ctx.RetryRecovery = &mutation.RetryRecovery{TicketID: rec.TicketID.Raw, AcceptanceRevision: rec.AcceptanceRevision, State: ticket.Unsatisfied, Reason: "RETRY_BUDGET_NOT_EXHAUSTED"}
			expected := "RETRY_BUDGET_NOT_EXHAUSTED"
			if mismatched {
				ctx.RetryRecovery.TicketID = fixture.TicketID("other")
				expected = "RECOVERY_OBSERVATION_MISSING_OR_MISMATCHED"
			}
			p := apply(t, ctx, envelope("reason", owner, "AT-01", "1", mutation.OpReopen, obj("reason", str("readmission"))))
			want(t, p, mutation.OutcomeBlocked, wire.CodeTicketState)
			if !strings.Contains(p.Detail, expected) {
				t.Fatalf("missing reason: %+v", p.Outcome)
			}
		}
	})
}
