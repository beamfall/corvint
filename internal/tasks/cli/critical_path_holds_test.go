package cli_test

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// cpPureFirstBlocker runs critical-path ROOT over records written by
// cpWrite, checks the read wrote nothing, and returns the first blocker code
// of node local.
func cpPureFirstBlocker(t *testing.T, local string, recs ...*ticket.Record) string {
	t.Helper()
	r := fixture.TempRepo(t)
	cpWrite(t, r, true, recs...)
	state, intents := fixture.TreeSnapshot(t, r.StateDir), fixture.TreeSnapshot(t, r.IntentDir)
	x := atm(t, r.Root, nil, "critical-path", "ROOT")
	if x.res.Outcome != wire.OutcomeOK || x.res.Mutation != nil {
		t.Fatalf("critical-path: %s", x.stdout)
	}
	if !fixture.SameTree(state, fixture.TreeSnapshot(t, r.StateDir)) || !fixture.SameTree(intents, fixture.TreeSnapshot(t, r.IntentDir)) {
		t.Fatal("critical-path wrote")
	}
	cpPlanParity(t, r, x.res.Items[0])
	return cpFirstCode(cpNode(t, x.res.Items[0], local))
}

// cpFirstCode is a node's first blocker code, or "" when it has none.
func cpFirstCode(n wire.Value) string {
	if fb := field(n, "firstBlocker"); fb.Kind == wire.KindObject {
		return field(fb, "code").Str
	}
	return ""
}

// CAL-V0-081 (V1-0802): a stage-scoped execution prerequisite is a node
// blocker. critical-path is stageless, so an unsatisfied prerequisite fails
// closed as PREREQUISITE_UNSATISFIED; without the prerequisite the same node
// has no such blocker.
func TestCALV0081_CriticalPathPrerequisiteNodeBlocker(t *testing.T) {
	build := func(withPrereq bool) []*ticket.Record {
		root, dep, pre := fixture.Ticket("ROOT"), fixture.Ticket("DEP"), fixture.Ticket("PRE")
		root.Dependencies = []ticket.Dependency{fixture.Dep("DEP")}
		pre.Status = ticket.StatusHeld
		pre.Holds = []ticket.Hold{{HoldID: "hold", Actor: "owner", Reason: "hold", PlacedAt: fixture.Timestamp}}
		if withPrereq {
			dep.ExecutionPrerequisites = []ticket.Prerequisite{{TicketID: pre.TicketID, Obligation: "COMPLETED", Stages: []string{"integrate"}}}
		}
		return []*ticket.Record{root, dep, pre}
	}
	if got := cpPureFirstBlocker(t, "DEP", build(true)...); got != wire.CodePrerequisiteUnsatisfied {
		t.Fatalf("first blocker %q; want %s", got, wire.CodePrerequisiteUnsatisfied)
	}
	if got := cpPureFirstBlocker(t, "DEP", build(false)...); got == wire.CodePrerequisiteUnsatisfied {
		t.Fatal("control without a prerequisite still reports PREREQUISITE_UNSATISFIED")
	}
}

// CAL-V0-081 (V1-0802): a current OPEN blocking escalation question holds
// the node as ESCALATION_PENDING (ESC-V0-006); an answered one does not.
func TestCALV0081_CriticalPathEscalationNodeBlocker(t *testing.T) {
	build := func(state string) []*ticket.Record {
		root, dep := fixture.Ticket("ROOT"), fixture.Ticket("DEP")
		root.Dependencies = []ticket.Dependency{fixture.Dep("DEP")}
		q := ticket.EscalationRef{RequestID: "q-a", OriginSha256: wire.Sum([]byte("q-a-origin")), HeadSha256: wire.Sum([]byte("q-a-" + state)), Revision: "1", AcceptanceRevision: "2", Kind: "decision", State: state}
		if state != "OPEN" {
			q.Revision = "2"
		}
		dep.AcceptanceRevision = "2"
		prev := wire.Sum([]byte("revision 1"))
		dep.PreviousRecordSha256 = &prev
		dep.Revision = wire.CountOf(2 + q.Revision.Int())
		dep.Escalations = &ticket.EscalationRefs{Revision: q.Revision, LastControlTicketRevision: dep.Revision, WorkRevision: "2", Entries: []ticket.EscalationRef{q}}
		return []*ticket.Record{root, dep}
	}
	if got := cpPureFirstBlocker(t, "DEP", build("OPEN")...); got != wire.CodeEscalationPending {
		t.Fatalf("first blocker %q; want %s", got, wire.CodeEscalationPending)
	}
	if got := cpPureFirstBlocker(t, "DEP", build("ANSWERED")...); got == wire.CodeEscalationPending {
		t.Fatal("control with an answered question still reports ESCALATION_PENDING")
	}
}

// CAL-V0-081 and CAL-V0-102 (V1-0802): a derived no-progress loop hold is a
// node blocker. Under an opted-in policy three clean no-tree hand-offs hold
// the dependency as LOOP_DETECTED; without the policy the same history does
// not.
func TestCALV0081_CriticalPathLoopNodeBlocker(t *testing.T) {
	for _, opted := range []bool{true, false} {
		root, _ := leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
		if opted {
			loopCLIPolicy(t, root)
		}
		dep := planTicket(t, root, "looping", "P1", `["src/"]`)
		loopCLIHandoffs(t, root, dep, 3)
		payload := strings.NewReplacer(`"dependencies":[]`, `"dependencies":[{"gateId":null,"obligation":"COMPLETED","ticketId":"`+dep+`"}]`, `"title":"Console ticket"`, `"title":"root"`).Replace(createPayloadJSON)
		created := handoffCLI(t, root, "ticket", "create", "--request-id", "create-root", "--issued-at", "2026-09-27T12:00:00Z", "--payload", payload)
		if created.res.Outcome != wire.OutcomeOK {
			t.Fatalf("create root: %s", created.stdout)
		}
		stateDir, intentDir := filepath.Join(root, ".git", "taskman"), filepath.Join(root, intent.Dir)
		state, intents := fixture.TreeSnapshot(t, stateDir), fixture.TreeSnapshot(t, intentDir)
		x := handoffCLI(t, root, "critical-path", field(created.res.Items[0], "ticketId").Str)
		if x.res.Outcome != wire.OutcomeOK || x.res.Mutation != nil {
			t.Fatalf("critical-path: %s", x.stdout)
		}
		if !fixture.SameTree(state, fixture.TreeSnapshot(t, stateDir)) || !fixture.SameTree(intents, fixture.TreeSnapshot(t, intentDir)) {
			t.Fatal("critical-path wrote")
		}
		var got string
		for _, n := range field(x.res.Items[0], "nodes").Arr {
			if field(n, "ticketId").Str == dep {
				got = cpFirstCode(n)
			}
		}
		if opted && got != wire.CodeLoopDetected {
			t.Fatalf("opted-in first blocker %q; want %s: %s", got, wire.CodeLoopDetected, x.stdout)
		}
		if !opted && got == wire.CodeLoopDetected {
			t.Fatalf("policy-absent history reports LOOP_DETECTED: %s", x.stdout)
		}
	}
}
