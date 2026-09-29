package cli_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// planTicket creates a ticket at the given priority declaring the given
// touch paths.
func planTicket(t *testing.T, root, title, priority, paths string) string {
	t.Helper()
	payload := strings.NewReplacer(`"priority":"P2"`, `"priority":"`+priority+`"`, `"touchPaths":[]`, `"touchPaths":`+paths, `"title":"Console ticket"`, `"title":"`+title+`"`).Replace(createPayloadJSON)
	x := atm(t, root, nil, "ticket", "create", "--request-id", "create-"+title, "--issued-at", "2026-09-27T12:00:00Z", "--payload", payload)
	if x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("create %s: %+v", title, x.res)
	}
	return field(x.res.Items[0], "ticketId").Str
}

// TestCALV0014_PlanPreviewIsAPurePriorityFirstPlan: plan preview renders a
// taskman-plan/0 object in (priority, order, ticketId) order, selects the
// first eligible ticket, defers the ones whose resources collide with it,
// blocks everything under an admission barrier, and writes nothing.
func TestCALV0014_PlanPreviewIsAPurePriorityFirstPlan(t *testing.T) {
	r := fixture.TempRepo(t)
	fixture.Write(t, filepath.Join(r.IntentDir, "queue.json"), fixture.QueueBytes())
	fixture.Write(t, filepath.Join(r.IntentDir, "policy.json"), fixture.PolicyBytes())
	if x := atm(t, r.Root, nil, "init"); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("init: %+v", x.res)
	}
	v := fixture.PolicyValue()
	v.Obj.Set("policyVersion", wire.String("2"))
	budgets, _ := v.Obj.Get("budgets")
	budgets.Obj.Set("requireEnforcedFields", wire.Strings(nil))
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "policy-2.json")
	fixture.Write(t, path, wire.EncodeFile(v))
	if x := atm(t, r.Root, nil, "policy", "update", "--request-id", "policy-2", "--expected-policy-version", "1", "--file", path); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("policy update: %+v", x.res)
	}
	whole := planTicket(t, r.Root, "whole", "P3", `[]`)
	narrow := planTicket(t, r.Root, "narrow", "P2", `["src/b.go"]`)
	top := planTicket(t, r.Root, "top", "P1", `["src/"]`)

	state, intents := fixture.TreeSnapshot(t, r.StateDir), fixture.TreeSnapshot(t, r.IntentDir)
	x := atm(t, r.Root, nil, "plan", "preview")
	if x.res.Outcome != wire.OutcomeOK || len(x.res.Items) != 1 {
		t.Fatalf("plan preview: %+v", x.res)
	}
	plan := x.res.Items[0]
	reservations, err := os.ReadFile(filepath.Join(r.StateDir, "reservations.json"))
	if err != nil {
		t.Fatal(err)
	}
	if field(plan, "profile").Str != "taskman-plan/0" || field(plan, "planningProfile").Str != "taskman-priority-first/0" || field(plan, "mutationAuthority").Bool || field(plan, "reservationSetSha256").Str != string(wire.Sum(reservations)) || field(field(plan, "capacity"), "maxActiveAttempts").Str != "1" {
		t.Fatalf("plan header: %+v", plan)
	}
	want := []struct{ id, state, reason, blocker string }{
		{top, "SELECTED", "DEVELOPMENT_MODE", ""},
		{narrow, "DEFERRED", "RESOURCE_COLLISION", top},
		{whole, "DEFERRED", "RESOURCE_COLLISION", top},
	}
	entries := field(plan, "entries").Arr
	if len(entries) != len(want) {
		t.Fatalf("entries: %+v", entries)
	}
	for i, w := range want {
		e := entries[i]
		blockers := field(e, "blockers").Arr
		if field(e, "ticketId").Str != w.id || field(e, "state").Str != w.state || field(e, "reason").Str != w.reason || field(e, "closureComplete").Bool != (w.id != whole) {
			t.Errorf("entry %d: %+v", i, e)
		}
		if (w.blocker == "") != (len(blockers) == 0) || (w.blocker != "" && blockers[0].Str != w.blocker) {
			t.Errorf("entry %d blockers: %+v", i, blockers)
		}
	}
	sameStore(t, r, state, intents, "plan preview")

	if p := atm(t, r.Root, nil, "pause", "--request-id", "pause"); p.res.Outcome != wire.OutcomeOK {
		t.Fatalf("pause: %+v", p.res)
	}
	paused := atm(t, r.Root, nil, "plan", "preview")
	for _, e := range field(paused.res.Items[0], "entries").Arr {
		if field(e, "state").Str != "BLOCKED" || field(e, "reason").Str != wire.CodePaused {
			t.Errorf("paused entry: %+v", e)
		}
	}
	if u := atm(t, r.Root, nil, "plan", "preview", "extra"); u.res.Outcome == wire.OutcomeOK {
		t.Fatalf("plan preview took an argument: %+v", u.res)
	}
}

// TestCALV0014_SelectedOnlyPlanPreviewIsComplete: the compact projection
// returns every selected ID and an explicit complete total even when the
// underlying plan contains hundreds of detailed entries.
func TestCALV0014_SelectedOnlyPlanPreviewIsComplete(t *testing.T) {
	r := fixture.TempRepo(t)
	fixture.WriteState(t, r)
	queue := fixture.QueueValue()
	policy := fixture.PolicyValue()
	capacity, _ := policy.Obj.Get("capacity")
	capacity.Obj.Set("maxActiveAttempts", wire.String("5"))
	budgets, _ := policy.Obj.Get("budgets")
	budgets.Obj.Set("requireEnforcedFields", wire.Strings(nil))
	fixture.Write(t, filepath.Join(r.IntentDir, "queue.json"), wire.EncodeFile(queue))
	fixture.Write(t, filepath.Join(r.IntentDir, "policy.json"), wire.EncodeFile(policy))
	for i := 0; i < 358; i++ {
		local := fmt.Sprintf("L%03d", i)
		rec := fixture.Ticket(local)
		rec.Priority = "P1"
		rec.Order = wire.CountOf(int64(i))
		rec.Effects.TouchPaths = []string{fmt.Sprintf("src/%03d.go", i)}
		fixture.Write(t, filepath.Join(r.IntentDir, "tickets", local+".json"), rec.Encode())
	}
	state, intents := fixture.TreeSnapshot(t, r.StateDir), fixture.TreeSnapshot(t, r.IntentDir)

	x := atm(t, r.Root, nil, "plan", "preview", "--selected-only")
	if x.res.Outcome != wire.OutcomeOK || len(x.res.Items) != 1 {
		t.Fatalf("selected-only preview: %+v", x.res)
	}
	summary := x.res.Items[0]
	wantIDs := []string{
		fixture.TicketID("L000"), fixture.TicketID("L001"), fixture.TicketID("L002"),
		fixture.TicketID("L003"), fixture.TicketID("L004"),
	}
	gotIDs := field(summary, "selectedTicketIds").Arr
	if field(summary, "profile").Str != "taskman-plan-selected/0" ||
		field(summary, "planningProfile").Str != "taskman-priority-first/0" ||
		field(summary, "queueId").Str != fixture.QueueID ||
		field(summary, "selectedTotal").Str != "5" ||
		!field(summary, "complete").Bool || field(summary, "mutationAuthority").Bool ||
		len(summary.Obj.Keys) != 7 || len(gotIDs) != len(wantIDs) {
		t.Fatalf("summary: %+v", summary)
	}
	for i, want := range wantIDs {
		if gotIDs[i].Str != want {
			t.Errorf("selected ID %d = %q, want %q", i, gotIDs[i].Str, want)
		}
	}
	sameStore(t, r, state, intents, "selected-only plan preview")
}

// TestCALV0002_PlanPreviewBlocksANonFixtureQueue: before its execution
// cutover a non-fixture queue plans every ticket BLOCKED CUTOVER_MISSING.
func TestCALV0002_PlanPreviewBlocksANonFixtureQueue(t *testing.T) {
	r := fixture.TempRepo(t)
	q := fixture.QueueValue()
	q.Obj.Set("fixture", wire.Bool(false))
	fixture.Write(t, filepath.Join(r.IntentDir, "queue.json"), wire.EncodeFile(q))
	fixture.Write(t, filepath.Join(r.IntentDir, "policy.json"), fixture.PolicyBytes())
	if x := atm(t, r.Root, nil, "init"); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("init: %+v", x.res)
	}
	id := planTicket(t, r.Root, "only", "P1", `["src/"]`)
	x := atm(t, r.Root, nil, "plan", "preview")
	if x.res.Outcome != wire.OutcomeOK || len(x.res.Items) != 1 {
		t.Fatalf("plan preview: %+v", x.res)
	}
	entries := field(x.res.Items[0], "entries").Arr
	if len(entries) != 1 || field(entries[0], "ticketId").Str != id || field(entries[0], "state").Str != "BLOCKED" || field(entries[0], "reason").Str != wire.CodeCutoverMissing {
		t.Fatalf("entries: %+v", entries)
	}
}
