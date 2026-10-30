package cli_test

import (
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// CAL-V0-101: with pools[].priorityAdmission, a lower-priority explicit
// pooled claim yields the last free member to a waiting higher-priority
// ticket; plan preview (default and --pool) defers it naming that ticket and
// stays a pure read; claim-next --pool admits the waiting ticket.
func TestCALV0101_PriorityYieldThroughTheCLI(t *testing.T) {
	r := fixture.TempRepo(t)
	v := fixture.PolicyValue()
	b, _ := v.Obj.Get("budgets")
	b.Obj.Set("requireEnforcedFields", wire.Array())
	v.Obj.Set("capacity", wire.ObjectValue(wire.NewObject().Set("maxActiveAttempts", wire.String("4")).Set("maxWorkersTotal", wire.String("4")).Set("classes", wire.Array())))
	v.Obj.Set("pools", wire.Array(wire.ObjectValue(wire.NewObject().Set("id", wire.String("lanes")).Set("members", wire.Strings([]string{"a", "b"})).Set("priorityAdmission", wire.Bool(true)))))
	fixture.Write(t, filepath.Join(r.IntentDir, "queue.json"), fixture.QueueBytes())
	fixture.Write(t, filepath.Join(r.IntentDir, "policy.json"), wire.EncodeFile(v))
	git(t, r.Root, "init", "-b", "main")
	git(t, r.Root, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "--allow-empty", "-m", "base")
	if x := atm(t, r.Root, nil, "init"); x.res.Outcome != wire.OutcomeOK {
		t.Fatal(x.res)
	}
	pooled := func(title, priority string) string {
		payload := strings.NewReplacer(`"priority":"P2"`, `"priority":"`+priority+`"`, `"touchPaths":[]`, `"touchPaths":["`+title+`"]`, `"title":"Console ticket"`, `"title":"`+title+`"`, `"requirementRefs":[],`, `"requirementRefs":[],"requiresPool":"lanes",`).Replace(createPayloadJSON)
		x := atm(t, r.Root, nil, "ticket", "create", "--request-id", "create-"+title, "--issued-at", "2026-09-27T12:00:00Z", "--payload", payload)
		if x.res.Outcome != wire.OutcomeOK {
			t.Fatalf("create %s: %+v", title, x.res)
		}
		return field(x.res.Items[0], "ticketId").Str
	}
	lead, hi, lo := pooled("lead", "P0"), pooled("hi", "P0"), pooled("lo", "P2")
	if x := atm(t, r.Root, nil, "claim", lead, "--holder", "lead", "--request-id", "lead", "--pool", "lanes"); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("lead claim %+v", x.res)
	}

	state, intents := fixture.TreeSnapshot(t, r.StateDir), fixture.TreeSnapshot(t, r.IntentDir)
	entries := func(args ...string) map[string]string {
		x := atm(t, r.Root, nil, append([]string{"plan", "preview"}, args...)...)
		if x.res.Outcome != wire.OutcomeOK {
			t.Fatal(x.res)
		}
		out := map[string]string{}
		for _, en := range field(x.res.Items[0], "entries").Arr {
			out[field(en, "ticketId").Str] = field(en, "state").Str + " " + field(en, "reason").Str + " " + string(wire.Encode(field(en, "blockers")))
		}
		return out
	}
	want := fmt.Sprintf(`DEFERRED RESOURCE_COLLISION [%q]`, hi)
	for _, args := range [][]string{nil, {"--pool", "lanes"}} {
		got := entries(args...)
		if got[hi] != "SELECTED DEVELOPMENT_MODE []" || got[lo] != want {
			t.Fatalf("plan preview %v: %v", args, got)
		}
	}
	if !reflect.DeepEqual(state, fixture.TreeSnapshot(t, r.StateDir)) || !reflect.DeepEqual(intents, fixture.TreeSnapshot(t, r.IntentDir)) {
		t.Fatal("preview changed queue")
	}

	refused := atm(t, r.Root, nil, "claim", lo, "--holder", "lo", "--request-id", "lo", "--pool", "lanes")
	if refused.res.Outcome == wire.OutcomeOK || !hasCode(refused.res, wire.CodeResourceCollision) || !strings.Contains(string(refused.stdout), "yields to "+hi) {
		t.Fatalf("lo claim %+v\n%s", refused.res, refused.stdout)
	}
	next := atm(t, r.Root, nil, "claim", "--next", "--holder", "builder", "--request-id", "next", "--pool", "lanes")
	if next.res.Outcome != wire.OutcomeOK || field(next.res.Items[0], "ticketId").Str != hi {
		t.Fatalf("claim-next %+v", next.res)
	}
}
