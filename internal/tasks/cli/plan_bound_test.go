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
	raw, err := flat.Encode()
	if err != nil {
		t.Fatalf("per-entry bound: %v", err)
	}
	if got := planNodeBound(wire.MaxTicketsPerQueue); got != wire.MaxResultNodes {
		t.Fatalf("bound %d, wire cap %d", got, wire.MaxResultNodes)
	}
	// The generic decoder keeps its bound; a plan consumer decodes the full
	// plan through the bounded route, which refuses a bound over the cap.
	if _, err := wire.DecodeResult(raw); wire.CodeOf(err) != wire.CodeLimitExceeded {
		t.Fatalf("generic decode of a full plan: %v, want LIMIT_EXCEEDED", err)
	}
	back, err := wire.DecodeResultLimit(raw, planNodeBound(wire.MaxTicketsPerQueue))
	if err != nil || len(back.Items) != 1 {
		t.Fatalf("bounded decode of a full plan: %v", err)
	}
	if got, _ := back.Items[0].Obj.Get("entries"); len(got.Arr) != wire.MaxTicketsPerQueue {
		t.Fatalf("bounded decode kept %d entries", len(got.Arr))
	}
	if _, err := wire.DecodeResultLimit(raw, wire.MaxResultNodes+1); wire.CodeOf(err) != wire.CodeLimitExceeded {
		t.Fatalf("decode bound over the cap: %v, want LIMIT_EXCEEDED", err)
	}
	flat.MaxNodes = wire.MaxResultNodes + 1
	if _, err := flat.Encode(); wire.CodeOf(err) != wire.CodeLimitExceeded {
		t.Fatalf("encode bound over the cap: %v, want LIMIT_EXCEEDED", err)
	}
	heavy := plan(planEntryNodes)
	heavy.MaxNodes = planNodeBound(wire.MaxTicketsPerQueue)
	if _, err := heavy.Encode(); wire.CodeOf(err) != wire.CodeLimitExceeded || !strings.Contains(err.Error(), "decoded nodes") {
		t.Fatalf("over budget: %v, want LIMIT_EXCEEDED", err)
	}
}
