package cli

import (
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// CAL-V0-141: a full plan of wire.MaxTicketsPerQueue typical entries encodes
// under the per-entry node bound that refused it under the flat envelope
// bound, and the bound still refuses entries far over their budget.
func TestCALV0141_PlanNodeBoundScalesPerEntry(t *testing.T) {
	retries, err := wire.Parse([]byte(`{"byReason":{"EXPIRED":"0","FAILED":"0","RELEASED":"0","UNKNOWN":"0"},"charged":"0","exhausted":false,"limit":"3","reasonHistory":"COMPLETE","remaining":"3","remainingMeaning":"RETRY_CAPACITY","retryAdmissionReason":"RETRY_AVAILABLE"}` + "\n"))
	if err != nil {
		t.Fatal(err)
	}
	rec := fixture.Ticket("A")
	plan := func(blockers int) *wire.Result {
		e := transaction.PlanEntry{Retries: retries, NextStage: wire.Null(), Ticket: rec, Resources: []ticket.Resource{{Class: "WHOLE_REPOSITORY", Key: "repo"}}, State: "DEFERRED", Reason: "RESOURCE_COLLISION"}
		for i := 0; i < blockers; i++ {
			e.Blockers = append(e.Blockers, rec.TicketID.Raw)
		}
		v, err := planEntryValue(e, false, nil)
		if err != nil {
			t.Fatal(err)
		}
		entries := make([]wire.Value, wire.MaxTicketsPerQueue)
		for i := range entries {
			entries[i] = v
		}
		o := wire.NewObject()
		o.Set("entries", wire.Array(entries...))
		return &wire.Result{Command: []string{"plan", "preview"}, Outcome: wire.OutcomeOK, Codes: []string{}, Items: []wire.Value{wire.ObjectValue(o)}, Warnings: []string{}}
	}
	flat := plan(1)
	if _, err := flat.Encode(); wire.CodeOf(err) != wire.CodeLimitExceeded {
		t.Fatalf("flat bound: %v, want LIMIT_EXCEEDED", err)
	}
	flat.MaxNodes = planNodeBound(wire.MaxTicketsPerQueue)
	if _, err := flat.Encode(); err != nil {
		t.Fatalf("per-entry bound: %v", err)
	}
	if got := planNodeBound(wire.MaxTicketsPerQueue); got != 890000 {
		t.Fatalf("bound %d", got)
	}
	heavy := plan(planEntryNodes)
	heavy.MaxNodes = planNodeBound(wire.MaxTicketsPerQueue)
	if _, err := heavy.Encode(); wire.CodeOf(err) != wire.CodeLimitExceeded || !strings.Contains(err.Error(), "decoded nodes") {
		t.Fatalf("over budget: %v, want LIMIT_EXCEEDED", err)
	}
}
