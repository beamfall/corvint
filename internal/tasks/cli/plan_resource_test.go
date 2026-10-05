package cli_test

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// CAL-V0-078: the default preview defers pool-needing tickets beyond the
// pool's free eligible members without consuming the window, reports the
// per-pool summary, and stays a pure read; claim-next without --pool claims
// the first lane-free SELECTED ticket and an explicit --pool claim admits a
// SELECTED pool ticket.
func TestCALV0078_DefaultPreviewIsResourceAware(t *testing.T) {
	r := fixture.TempRepo(t)
	v := fixture.PolicyValue()
	b, _ := v.Obj.Get("budgets")
	b.Obj.Set("requireEnforcedFields", wire.Array())
	v.Obj.Set("capacity", wire.ObjectValue(wire.NewObject().Set("maxActiveAttempts", wire.String("4")).Set("maxWorkersTotal", wire.String("4")).Set("classes", wire.Array())))
	v.Obj.Set("pools", wire.Array(wire.ObjectValue(wire.NewObject().Set("id", wire.String("lanes")).Set("members", wire.Strings([]string{"a", "b"})))))
	fixture.Write(t, filepath.Join(r.IntentDir, "queue.json"), fixture.QueueBytes())
	fixture.Write(t, filepath.Join(r.IntentDir, "policy.json"), wire.EncodeFile(v))
	git(t, r.Root, "init", "-b", "main")
	git(t, r.Root, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "--allow-empty", "-m", "base")
	if x := atm(t, r.Root, nil, "init"); x.res.Outcome != wire.OutcomeOK {
		t.Fatal(x.res)
	}
	pooled := func(title string) string {
		payload := strings.NewReplacer(`"priority":"P2"`, `"priority":"P0"`, `"touchPaths":[]`, `"touchPaths":["`+title+`"]`, `"title":"Console ticket"`, `"title":"`+title+`"`, `"requirementRefs":[],`, `"requirementRefs":[],"requiresPool":"lanes",`).Replace(createPayloadJSON)
		x := atm(t, r.Root, nil, "ticket", "create", "--request-id", "create-"+title, "--issued-at", "2026-09-27T12:00:00Z", "--payload", payload)
		if x.res.Outcome != wire.OutcomeOK {
			t.Fatalf("create %s: %+v", title, x.res)
		}
		return field(x.res.Items[0], "ticketId").Str
	}
	p1, _, _ := pooled("p1"), pooled("p2"), pooled("p3")
	f1 := planTicket(t, r.Root, "f1", "P2", `["f1"]`)
	planTicket(t, r.Root, "f2", "P2", `["f2"]`)
	planTicket(t, r.Root, "f3", "P2", `["f3"]`)

	state, intents := fixture.TreeSnapshot(t, r.StateDir), fixture.TreeSnapshot(t, r.IntentDir)
	x := atm(t, r.Root, nil, "plan", "preview")
	if x.res.Outcome != wire.OutcomeOK {
		t.Fatal(x.res)
	}
	plan := x.res.Items[0]
	got := map[string]int{}
	for _, en := range field(plan, "entries").Arr {
		key := field(en, "state").Str + " " + field(en, "reason").Str + " " + string(wire.Encode(field(en, "blockers")))
		got[key]++
	}
	want := map[string]int{"SELECTED DEVELOPMENT_MODE []": 4, `DEFERRED RESOURCE_COLLISION ["lanes"]`: 1, `DEFERRED LIMIT_EXCEEDED ["LIMIT_EXCEEDED"]`: 1}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("entries %v", got)
	}
	rows := field(plan, "resourceDeferred").Arr
	if len(rows) != 1 || string(wire.Encode(rows[0])) != `{"availability":"OBSERVED","deferred":"1","freeEligibleMembers":"2","poolId":"lanes","selected":"2"}` {
		t.Fatalf("resourceDeferred %s", wire.Encode(field(plan, "resourceDeferred")))
	}
	if !reflect.DeepEqual(state, fixture.TreeSnapshot(t, r.StateDir)) || !reflect.DeepEqual(intents, fixture.TreeSnapshot(t, r.IntentDir)) {
		t.Fatal("preview changed queue")
	}
	if pp := atm(t, r.Root, nil, "plan", "preview", "--pool", "lanes"); pp.res.Outcome != wire.OutcomeOK || hasKey(pp.res.Items[0], "resourceDeferred") {
		t.Fatalf("--pool preview %+v", pp.res)
	}

	next := atm(t, r.Root, nil, "claim", "--next", "--holder", "builder", "--request-id", "next")
	if next.res.Outcome != wire.OutcomeOK || field(next.res.Items[0], "ticketId").Str != f1 {
		t.Fatalf("claim-next %+v", next.res)
	}
	pool := atm(t, r.Root, nil, "claim", p1, "--holder", "lane", "--request-id", "pool", "--pool", "lanes")
	if pool.res.Outcome != wire.OutcomeOK || field(field(pool.res.Items[0], "poolAllocation"), "memberId").Str != "a" {
		t.Fatalf("explicit pool claim %+v", pool.res)
	}
}

func hasKey(v wire.Value, key string) bool {
	_, ok := v.Obj.Get(key)
	return ok
}
