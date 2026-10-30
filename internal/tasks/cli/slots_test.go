package cli_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// slotsCLIRepo is an initialized repository whose policy declares slots gpu
// of capacity 2 with one slot reserved for class ci.
func slotsCLIRepo(t *testing.T) *fixture.Repo {
	t.Helper()
	r := exclusionCLIRepo(t)
	v := fixture.PolicyValue()
	b, _ := v.Obj.Get("budgets")
	b.Obj.Set("requireEnforcedFields", wire.Array())
	v.Obj.Set("capacity", wire.ObjectValue(wire.NewObject().Set("maxActiveAttempts", wire.String("4")).Set("maxWorkersTotal", wire.String("4")).Set("classes", wire.Array())))
	v.Obj.Set("slots", wire.Array(wire.ObjectValue(wire.NewObject().Set("id", wire.String("gpu")).Set("capacity", wire.String("2")).Set("reserved", wire.ObjectValue(wire.NewObject().Set("ci", wire.String("1")))))))
	fixture.Write(t, filepath.Join(r.IntentDir, "policy.json"), wire.EncodeFile(v))
	if x := atm(t, r.Root, nil, "init"); x.res.Outcome != wire.OutcomeOK {
		t.Fatal(x.res)
	}
	return r
}

// TestPSRV0038_CLISlotResultsAndQueueStatus: slots acquire and release
// report the attempt's `slot`, a replay reports the receipt as replayed with
// `slot` null (PSR-V0-043), and
// queue status shows each resource's occupancy, holders and waiters in wait
// order as a pure read; release accepts --lock-wait; help names both verbs'
// flags.
func TestPSRV0038_CLISlotResultsAndQueueStatus(t *testing.T) {
	r := slotsCLIRepo(t)
	claim := func(title, priority, path string) (string, string) {
		id := planTicket(t, r.Root, title, priority, `["`+path+`"]`)
		x := atm(t, r.Root, nil, "claim", id, "--holder", "builder", "--request-id", "claim-"+title, "--stage", "implement")
		if x.res.Outcome != wire.OutcomeOK {
			t.Fatalf("claim %s", x.stdout)
		}
		return field(x.res.Items[0], "attemptId").Str, field(x.res.Items[0], "generation").Str
	}
	a, ag := claim("one", "P2", "one")
	b, bg := claim("two", "P1", "two")
	acquire := []string{"slots", "acquire", "--attempt", a, "--generation", ag, "--slots", "gpu", "--request-id", "a-1"}
	x := atm(t, r.Root, nil, acquire...)
	slot := field(x.res.Items[0], "slot")
	if x.res.Outcome != wire.OutcomeOK || field(slot, "state").Str != "HELD" || field(slot, "count").Str != "1" || field(slot, "position").Kind != wire.KindNull || field(slot, "class").Kind != wire.KindNull {
		t.Fatalf("acquire %s", x.stdout)
	}
	if again := atm(t, r.Root, nil, acquire...); !field(again.res.Items[0], "replayed").Bool || field(again.res.Items[0], "slot").Kind != wire.KindNull {
		t.Fatalf("replay %s", again.stdout)
	}
	// One slot is reserved for ci, so b's unreserved request waits.
	x = atm(t, r.Root, nil, "slots", "acquire", "--attempt", b, "--generation", bg, "--slots", "gpu", "--count", "1", "--request-id", "b-1")
	if slot := field(x.res.Items[0], "slot"); x.res.Outcome != wire.OutcomeOK || field(slot, "state").Str != "WAITING" || field(slot, "position").Str != "1" {
		t.Fatalf("waiting %s", x.stdout)
	}
	before := fixture.TreeSnapshot(t, r.StateDir)
	status := atm(t, r.Root, nil, "queue", "status")
	if !fixture.SameTree(before, fixture.TreeSnapshot(t, r.StateDir)) {
		t.Fatal("queue status wrote state")
	}
	slots := field(status.res.Items[0], "slots")
	if status.res.Outcome != wire.OutcomeOK || slots.Kind != wire.KindArray || len(slots.Arr) != 1 {
		t.Fatalf("queue status %s", status.stdout)
	}
	gpu := slots.Arr[0]
	holders, waiters := field(gpu, "holders").Arr, field(gpu, "waiters").Arr
	if field(gpu, "slotsId").Str != "gpu" || field(gpu, "capacity").Str != "2" || field(gpu, "held").Str != "1" || field(gpu, "free").Str != "1" || field(field(gpu, "reserved"), "ci").Str != "1" ||
		len(holders) != 1 || field(holders[0], "attemptId").Str != a || field(holders[0], "holder").Str != "builder" || field(holders[0], "leaseExpired").Bool ||
		len(waiters) != 1 || field(waiters[0], "attemptId").Str != b || field(waiters[0], "priority").Str != "P1" || field(waiters[0], "position").Str != "1" {
		t.Fatalf("slots occupancy %s", wire.Encode(gpu))
	}
	x = atm(t, r.Root, nil, "slots", "release", "--attempt", a, "--generation", ag, "--slots", "gpu", "--request-id", "a-2", "--lock-wait", "5")
	if x.res.Outcome != wire.OutcomeOK || field(field(x.res.Items[0], "slot"), "state").Str != "NONE" {
		t.Fatalf("release %s", x.stdout)
	}
	if gpu := field(atm(t, r.Root, nil, "queue", "status").res.Items[0], "slots").Arr[0]; len(field(gpu, "waiters").Arr) != 0 || field(field(gpu, "holders").Arr[0], "attemptId").Str != b {
		t.Fatalf("after release %s", wire.Encode(gpu))
	}
	for name, want := range map[string]string{"acquire": "--slots ID --request-id ID [--count N | --exclusive] [--class LABEL]", "release": "--slots ID --request-id ID"} {
		help := atm(t, r.Root, nil, "slots", name, "--help")
		if help.res.Outcome != wire.OutcomeOK || !strings.Contains(field(help.res.Items[0], "usage").Str, want) {
			t.Fatalf("%s help %s", name, help.stdout)
		}
	}
	for _, bad := range [][]string{
		{"slots", "acquire", "--attempt", b, "--generation", bg, "--request-id", "no-slots"},
		{"slots", "acquire", "--attempt", b, "--generation", bg, "--slots", "gpu", "--exclusive", "--count", "1", "--request-id", "both"},
		{"slots", "acquire", "--attempt", b, "--generation", bg, "--slots", "gpu", "--count", "65", "--request-id", "huge"},
		{"slots", "release", "--attempt", b, "--generation", bg, "--slots", "gpu", "--exclusive", "--request-id", "release-exclusive"},
		{"claim", "--next", "--holder", "h", "--exclusive", "--request-id", "claim-exclusive"},
		{"slots", "status"},
	} {
		if x := atm(t, r.Root, nil, bad...); x.res.Outcome == wire.OutcomeOK {
			t.Fatalf("malformed %v accepted: %s", bad, x.stdout)
		}
	}
}
