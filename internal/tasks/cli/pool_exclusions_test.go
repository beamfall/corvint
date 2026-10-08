package cli_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func exclusionCLIRepo(t *testing.T) *fixture.Repo {
	t.Helper()
	r := fixture.TempRepo(t)
	v := fixture.PolicyValue()
	b, _ := v.Obj.Get("budgets")
	b.Obj.Set("requireEnforcedFields", wire.Array())
	v.Obj.Set("capacity", wire.ObjectValue(wire.NewObject().Set("maxActiveAttempts", wire.String("4")).Set("maxWorkersTotal", wire.String("4")).Set("classes", wire.Array())))
	v.Obj.Set("pools", wire.Array(wire.ObjectValue(wire.NewObject().Set("id", wire.String("db")).Set("members", wire.Strings([]string{"a", "b", "review"})).Set("reservedFor", wire.ObjectValue(wire.NewObject().Set("review", wire.String("review")))))))
	fixture.Write(t, filepath.Join(r.IntentDir, "queue.json"), fixture.QueueBytes())
	fixture.Write(t, filepath.Join(r.IntentDir, "policy.json"), wire.EncodeFile(v))
	git(t, r.Root, "init", "-b", "main")
	git(t, r.Root, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "--allow-empty", "-m", "base")
	return r
}

// CAL-V0-065: both parsers normalize repeated single values, retain other
// duplicate refusals, and preview writes no journal, intent or probe state.
func TestCALV0065_CLIExclusionsAndPreviewPurity(t *testing.T) {
	r := exclusionCLIRepo(t)
	if x := atm(t, r.Root, nil, "init"); x.res.Outcome != wire.OutcomeOK {
		t.Fatal(x.res)
	}
	id := planTicket(t, r.Root, "one", "P2", `["one"]`)
	planTicket(t, r.Root, "two", "P2", `["two"]`)
	state, intents := fixture.TreeSnapshot(t, r.StateDir), fixture.TreeSnapshot(t, r.IntentDir)
	for _, args := range [][]string{
		{"--exclude-member"}, {"--exclude-member", ""}, {"--exclude-member", "--pool", "db"}, {"--exclude-member", "a"},
		{"--pool", "db", "--exclude-member", "foreign"}, {"--pool", "db", "--pool", "db"}, {"--stage", "review", "--stage", "review"},
	} {
		if x := atm(t, r.Root, nil, append([]string{"plan", "preview"}, args...)...); x.res.Outcome == wire.OutcomeOK {
			t.Fatalf("malformed preview accepted: %v", args)
		}
	}
	a := atm(t, r.Root, nil, "plan", "preview", "--pool", "db", "--stage", "review", "--exclude-member", "review", "--exclude-member", "a", "--exclude-member", "review")
	b := atm(t, r.Root, nil, "plan", "preview", "--pool", "db", "--stage", "review", "--exclude-member", "a", "--exclude-member", "review")
	if a.res.Outcome != wire.OutcomeOK || !reflect.DeepEqual(a.res.Items, b.res.Items) {
		t.Fatalf("preview normalization: %+v %+v", a.res, b.res)
	}
	selected := 0
	for _, en := range field(a.res.Items[0], "entries").Arr {
		if field(en, "state").Str == "SELECTED" {
			selected++
		}
	}
	if selected != 1 {
		t.Fatalf("preview selected %d", selected)
	}
	all := atm(t, r.Root, nil, "plan", "preview", "--pool", "db", "--exclude-member", "a", "--exclude-member", "b", "--exclude-member", "review")
	for _, en := range field(all.res.Items[0], "entries").Arr {
		if field(en, "state").Str == "SELECTED" || field(en, "reason").Str != wire.CodeResourceCollision {
			t.Fatalf("all excluded preview %s", wire.Encode(en))
		}
	}
	if !reflect.DeepEqual(state, fixture.TreeSnapshot(t, r.StateDir)) || !reflect.DeepEqual(intents, fixture.TreeSnapshot(t, r.IntentDir)) {
		t.Fatal("preview changed queue")
	}
	for i, args := range [][]string{{"--exclude-member"}, {"--exclude-member", ""}, {"--exclude-member", "--pool", "db"}, {"--exclude-member", "a"}, {"--pool", "db", "--holder", "again"}} {
		prefix := []string{"claim", id, "--holder", "builder", "--request-id", string(rune('a' + i))}
		if x := atm(t, r.Root, nil, append(prefix, args...)...); x.res.Outcome == wire.OutcomeOK {
			t.Fatalf("malformed claim accepted %v", args)
		}
	}
	claim := atm(t, r.Root, nil, "claim", id, "--holder", "builder", "--request-id", "normalized", "--pool", "db", "--stage", "review", "--exclude-member", "review", "--exclude-member", "a", "--exclude-member", "review")
	replay := atm(t, r.Root, nil, "claim", id, "--holder", "builder", "--request-id", "normalized", "--pool", "db", "--stage", "review", "--exclude-member", "a", "--exclude-member", "review")
	if claim.res.Outcome != wire.OutcomeOK || replay.res.Outcome != wire.OutcomeOK || !field(replay.res.Items[0], "replayed").Bool || field(field(claim.res.Items[0], "poolAllocation"), "memberId").Str != "b" {
		t.Fatalf("claim normalization %+v %+v", claim.res, replay.res)
	}
}

// CAL-V0-065: the freshly built native executable drives a real disposable
// Git/store lifecycle. Excluded reserved health never runs; preview is pure;
// ordinary release still quarantines, and request replay returns its old tuple.
func TestCALV0065_NativeFixture(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "corvint-tasks")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	build := exec.CommandContext(ctx, "go", "build", "-o", binary, "../../../cmd/corvint-tasks")
	build.WaitDelay = time.Second
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v %s", err, out)
	}
	r := exclusionCLIRepo(t)
	markers := t.TempDir()
	excluded := filepath.Join(markers, "excluded")
	allowed := filepath.Join(markers, "allowed")
	raw, err := os.ReadFile(filepath.Join(r.IntentDir, "policy.json"))
	if err != nil {
		t.Fatal(err)
	}
	v, err := wire.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	command := func(marker string) wire.Value {
		return wire.ObjectValue(wire.NewObject().Set("argv", wire.Strings([]string{"/usr/bin/touch", marker})).Set("cwd", wire.String("REPOSITORY")).Set("env", wire.Array()).Set("timeoutSeconds", wire.String("3")))
	}
	pools, _ := v.Obj.Get("pools")
	pools.Arr[0].Obj.Set("memberConfig", wire.ObjectValue(wire.NewObject().Set("review", wire.ObjectValue(wire.NewObject().Set("health", command(excluded)))).Set("b", wire.ObjectValue(wire.NewObject().Set("health", command(allowed))))))
	fixture.Write(t, filepath.Join(r.IntentDir, "policy.json"), wire.EncodeFile(v))
	call := func(args ...string) run { t.Helper(); return handoffCLIWithBinary(t, r.Root, binary, args...) }
	if x := call("init"); x.res.Outcome != wire.OutcomeOK {
		t.Fatal(x.res)
	}
	created := call("ticket", "create", "--request-id", "native-one", "--payload", createPayloadJSON)
	if created.res.Outcome != wire.OutcomeOK {
		t.Fatal(created.res)
	}
	id := field(created.res.Items[0], "ticketId").Str
	state, intents := fixture.TreeSnapshot(t, r.StateDir), fixture.TreeSnapshot(t, r.IntentDir)
	preview := call("plan", "preview", "--pool", "db", "--stage", "review", "--exclude-member", "review", "--exclude-member", "a")
	if preview.res.Outcome != wire.OutcomeOK || !reflect.DeepEqual(state, fixture.TreeSnapshot(t, r.StateDir)) || !reflect.DeepEqual(intents, fixture.TreeSnapshot(t, r.IntentDir)) {
		t.Fatal("native preview changed queue")
	}
	if _, err := os.Stat(allowed); !os.IsNotExist(err) {
		t.Fatalf("preview probed: %v", err)
	}
	args := []string{"claim", id, "--holder", "builder", "--request-id", "native-claim", "--scope", "native", "--pool", "db", "--stage", "review", "--exclude-member", "review", "--exclude-member", "a", "--exclude-member", "review"}
	claim := call(args...)
	if claim.res.Outcome != wire.OutcomeOK {
		pools, _ := os.ReadFile(filepath.Join(r.StateDir, "pools.json"))
		t.Fatalf("native claim %+v pools %s", claim.res, pools)
	}
	item := claim.res.Items[0]
	allocation := field(item, "poolAllocation")
	if field(allocation, "memberId").Str != "b" {
		t.Fatalf("wrong native member %s", wire.Encode(allocation))
	}
	if _, err := os.Stat(excluded); !os.IsNotExist(err) {
		t.Fatalf("excluded native probe ran %v", err)
	}
	if _, err := os.Stat(allowed); err != nil {
		t.Fatal("eligible native health missing", err)
	}
	release := call("release", "--attempt", field(item, "attemptId").Str, "--generation", field(item, "generation").Str, "--request-id", "native-release")
	if release.res.Outcome != wire.OutcomeOK {
		t.Fatal(release.res)
	}
	poolRaw, err := os.ReadFile(filepath.Join(r.StateDir, "pools.json"))
	if err != nil {
		t.Fatal(err)
	}
	poolState, err := snapshot.DecodePools(poolRaw)
	if err != nil {
		t.Fatal(err)
	}
	if len(poolState.Entries) != 1 || poolState.Entries[0].State != "QUARANTINED" {
		t.Fatalf("release weakened quarantine %+v", poolState)
	}
	args = args[:len(args)-2]
	args[len(args)-3], args[len(args)-1] = "a", "review"
	replay := call(args...)
	if replay.res.Outcome != wire.OutcomeOK || !field(replay.res.Items[0], "replayed").Bool || !reflect.DeepEqual(field(replay.res.Items[0], "poolAllocation"), allocation) {
		t.Fatalf("native replay %+v", replay.res)
	}
	blocked := call("claim", "--next", "--holder", "builder", "--request-id", "native-all-excluded", "--pool", "db", "--exclude-member", "a", "--exclude-member", "b", "--exclude-member", "review")
	if blocked.res.Outcome == wire.OutcomeOK {
		t.Fatalf("native all excluded claim admitted %+v", blocked.res)
	}
	if audit := call("receipt", "audit"); audit.res.Outcome != wire.OutcomeOK || field(audit.res.Items[0], "structuralConsistency").Str != "CONSISTENT" {
		t.Fatalf("native audit %+v", audit.res)
	}
}
