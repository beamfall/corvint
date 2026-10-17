package cli_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestPSRV0080_OlderBinaryRefusesSlots is opt-in: it runs an older
// corvint-tasks binary named by CORVINT_TASKS_OLDER_BINARY (built from a
// release tag that predates slots) against temporary stores written by this
// build. A store whose policy declares slots, and one whose pools.json holds a
// slot entry, must both be refused MALFORMED by the older binary's read, while
// the same store without slots is read normally (the control), so the refusal
// is caused by the slots keys alone (PSR-V0-080).
func TestPSRV0080_OlderBinaryRefusesSlots(t *testing.T) {
	older := os.Getenv("CORVINT_TASKS_OLDER_BINARY")
	if older == "" {
		t.Skip("set CORVINT_TASKS_OLDER_BINARY to an older corvint-tasks binary")
	}
	runOlder := func(t *testing.T, root string, args ...string) *wire.Result {
		t.Helper()
		var out, errb bytes.Buffer
		cmd := exec.Command(older, args...)
		cmd.Dir = root
		cmd.Stdout, cmd.Stderr = &out, &errb
		_ = cmd.Run()
		res, err := wire.DecodeResult(out.Bytes())
		if err != nil {
			t.Fatalf("older %v: envelope does not decode: %v\nstdout %s\nstderr %s", args, err, out.Bytes(), errb.Bytes())
		}
		t.Logf("older %v: outcome %s codes %v exit %d: %.600s", args, res.Outcome, res.Codes, cmd.ProcessState.ExitCode(), out.Bytes())
		return res
	}
	t.Run("control-no-slots", func(t *testing.T) {
		r := exclusionCLIRepo(t)
		if x := atm(t, r.Root, nil, "init"); x.res.Outcome != wire.OutcomeOK {
			t.Fatal(x.res)
		}
		if res := runOlder(t, r.Root, "queue", "status"); res.Outcome != wire.OutcomeOK {
			t.Fatalf("control store refused by the older binary: %s %v", res.Outcome, res.Codes)
		}
	})
	t.Run("policy-declares-slots", func(t *testing.T) {
		r := slotsCLIRepo(t)
		before := fixture.TreeSnapshot(t, r.StateDir)
		res := runOlder(t, r.Root, "queue", "status")
		if res.Outcome == wire.OutcomeOK || !hasCode(res, "MALFORMED") {
			t.Fatalf("older binary accepted a policy with slots: %s %v", res.Outcome, res.Codes)
		}
		if !fixture.SameTree(before, fixture.TreeSnapshot(t, r.StateDir)) {
			t.Fatal("older binary wrote state while refusing")
		}
	})
	t.Run("pools-json-holds-slots", func(t *testing.T) {
		r := slotsCLIRepo(t)
		id := planTicket(t, r.Root, "one", "P2", `["one"]`)
		x := atm(t, r.Root, nil, "claim", id, "--holder", "builder", "--request-id", "claim-one", "--stage", "implement")
		if x.res.Outcome != wire.OutcomeOK {
			t.Fatalf("claim %s", x.stdout)
		}
		a, ag := field(x.res.Items[0], "attemptId").Str, field(x.res.Items[0], "generation").Str
		if x := atm(t, r.Root, nil, "slots", "acquire", "--attempt", a, "--generation", ag, "--slots", "gpu", "--request-id", "a-1"); x.res.Outcome != wire.OutcomeOK {
			t.Fatalf("acquire %s", x.stdout)
		}
		pools, err := os.ReadFile(filepath.Join(r.StateDir, "pools.json"))
		if err != nil || !bytes.Contains(pools, []byte(`"slots"`)) {
			t.Fatalf("pools.json holds no slots: %v %s", err, pools)
		}
		before := fixture.TreeSnapshot(t, r.StateDir)
		for _, args := range [][]string{{"queue", "status"}, {"attempt", "show", a}} {
			res := runOlder(t, r.Root, args...)
			if res.Outcome == wire.OutcomeOK || !hasCode(res, "MALFORMED") {
				t.Fatalf("older binary %v accepted a store with slots: %s %v", args, res.Outcome, res.Codes)
			}
		}
		if !fixture.SameTree(before, fixture.TreeSnapshot(t, r.StateDir)) {
			t.Fatal("older binary wrote state while refusing")
		}
		// A policy update that drops slots leaves the held entry in
		// pools.json until the next slot write, so the older binary now
		// meets slots only in pools.json and still refuses.
		v := fixture.PolicyValue()
		b, _ := v.Obj.Get("budgets")
		b.Obj.Set("requireEnforcedFields", wire.Array())
		v.Obj.Set("capacity", wire.ObjectValue(wire.NewObject().Set("maxActiveAttempts", wire.String("4")).Set("maxWorkersTotal", wire.String("4")).Set("classes", wire.Array())))
		v.Obj.Set("pools", wire.Array(wire.ObjectValue(wire.NewObject().Set("id", wire.String("db")).Set("members", wire.Strings([]string{"a", "b", "review"})).Set("reservedFor", wire.ObjectValue(wire.NewObject().Set("review", wire.String("review")))))))
		v.Obj.Set("policyVersion", wire.String("2"))
		dir, err := filepath.EvalSymlinks(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(dir, "policy-2.json")
		fixture.Write(t, path, wire.EncodeFile(v))
		if x := atm(t, r.Root, nil, "policy", "update", "--request-id", "drop-slots", "--expected-policy-version", "1", "--file", path); x.res.Outcome != wire.OutcomeOK {
			t.Fatalf("policy update %s", x.stdout)
		}
		if pools, err := os.ReadFile(filepath.Join(r.StateDir, "pools.json")); err != nil || !bytes.Contains(pools, []byte(`"slots"`)) {
			t.Fatalf("pools.json no longer holds slots: %v %s", err, pools)
		}
		before = fixture.TreeSnapshot(t, r.StateDir)
		res := runOlder(t, r.Root, "queue", "status")
		// The older decoder names the pools.json root ("/"), not policy.json.
		if w := strings.Join(res.Warnings, " "); res.Outcome == wire.OutcomeOK || !hasCode(res, "MALFORMED") || !strings.Contains(w, "unknown keys: slots") || strings.Contains(w, "policy.json") {
			t.Fatalf("older binary accepted pools.json with slots: %s %v %v", res.Outcome, res.Codes, res.Warnings)
		}
		if !fixture.SameTree(before, fixture.TreeSnapshot(t, r.StateDir)) {
			t.Fatal("older binary wrote state while refusing")
		}
	})
}
