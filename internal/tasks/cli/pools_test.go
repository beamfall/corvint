package cli_test

import (
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/wire"
	"path/filepath"
	"reflect"
	"testing"
)

// CAL-V0-034.
func TestPoolPreviewConsumesMembersWithoutProbes(t *testing.T) {
	r := fixture.TempRepo(t)
	fixture.Write(t, filepath.Join(r.IntentDir, "queue.json"), fixture.QueueBytes())
	v := fixture.PolicyValue()
	v.Obj.Set("capacity", wire.ObjectValue(wire.NewObject().Set("maxActiveAttempts", wire.String("4")).Set("maxWorkersTotal", wire.String("4")).Set("classes", wire.Array())))
	b, _ := v.Obj.Get("budgets")
	b.Obj.Set("requireEnforcedFields", wire.Array())
	command := wire.ObjectValue(wire.NewObject().Set("argv", wire.Strings([]string{"/does-not-exist"})).Set("cwd", wire.String("REPOSITORY")).Set("env", wire.Array()).Set("timeoutSeconds", wire.String("1")))
	v.Obj.Set("pools", wire.Array(wire.ObjectValue(wire.NewObject().Set("id", wire.String("db")).Set("members", wire.Strings([]string{"a", "review"})).Set("reservedFor", wire.ObjectValue(wire.NewObject().Set("review", wire.String("review")))).Set("memberConfig", wire.ObjectValue(wire.NewObject().Set("a", wire.ObjectValue(wire.NewObject().Set("health", command))))))))
	fixture.Write(t, filepath.Join(r.IntentDir, "policy.json"), wire.EncodeFile(v))
	if x := atm(t, r.Root, nil, "init"); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("init %+v", x.res)
	}
	for _, name := range []string{"one", "two", "three"} {
		planTicket(t, r.Root, name, "P2", `["`+name+`"]`)
	}
	before := fixture.TreeSnapshot(t, r.StateDir)
	intents := fixture.TreeSnapshot(t, r.IntentDir)

	for _, args := range [][]string{
		{"--stage", "unknown"}, {"--stage", ""}, {"--pool", "bad\npool"},
		{"--pool", "db", "--pool", "db"}, {"--stage", "implement", "--stage", "review"},
	} {
		bad := atm(t, r.Root, nil, append([]string{"plan", "preview"}, args...)...)
		if bad.res.Outcome == wire.OutcomeOK {
			t.Fatalf("accepted malformed preview %v: %+v", args, bad.res)
		}
	}
	x := atm(t, r.Root, nil, "plan", "preview", "--pool", "db", "--stage", "implement")
	if x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("preview %+v", x.res)
	}
	entries := field(x.res.Items[0], "entries")
	selected := 0
	for _, en := range entries.Arr {
		if field(en, "state").Str == "SELECTED" {
			selected++
		}
	}
	if selected != 1 {
		t.Fatalf("selected %d: %+v", selected, x.res)
	}
	if !reflect.DeepEqual(before, fixture.TreeSnapshot(t, r.StateDir)) || !reflect.DeepEqual(intents, fixture.TreeSnapshot(t, r.IntentDir)) {
		t.Fatal("preview mutated queue")
	}
}
