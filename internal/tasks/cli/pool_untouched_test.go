package cli_test

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// Terminal qualification: a compiled CLI and fixture:false native queue.
// The explicitly synthetic cutover input exercises parser/admission wiring;
// it is not evidence that CAL019 ran or that a deployment is qualified.
func TestPoolLaneUntouched_NativeFixture(t *testing.T) {
	t.Run("CAL-V0-067", func(t *testing.T) {
		binary := filepath.Join(t.TempDir(), "corvint-tasks")
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		build := exec.CommandContext(ctx, "go", "build", "-o", binary, "../../../cmd/corvint-tasks")
		build.WaitDelay = time.Second
		if out, err := build.CombinedOutput(); err != nil {
			t.Fatalf("build %v %s", err, out)
		}
		r := exclusionCLIRepo(t)
		q := fixture.QueueValue()
		q.Obj.Set("fixture", wire.Bool(false))
		q.Obj.Set("canonicalWriter", wire.String("NATIVE"))
		fixture.Write(t, filepath.Join(r.IntentDir, "queue.json"), wire.EncodeFile(q))
		marker := filepath.Join(t.TempDir(), "cleanup")
		raw, err := os.ReadFile(filepath.Join(r.IntentDir, "policy.json"))
		if err != nil {
			t.Fatal(err)
		}
		policy, err := wire.Parse(raw)
		if err != nil {
			t.Fatal(err)
		}
		pools, _ := policy.Obj.Get("pools")
		command := wire.ObjectValue(wire.NewObject().Set("argv", wire.Strings([]string{"/usr/bin/touch", marker})).Set("cwd", wire.String("REPOSITORY")).Set("env", wire.Array()).Set("timeoutSeconds", wire.String("3")))
		pools.Arr[0].Obj.Set("memberConfig", wire.ObjectValue(wire.NewObject().Set("review", wire.ObjectValue(wire.NewObject().Set("cleanup", command)))))
		fixture.Write(t, filepath.Join(r.IntentDir, "policy.json"), wire.EncodeFile(policy))
		call := func(args ...string) run { t.Helper(); return handoffCLIWithBinary(t, r.Root, binary, args...) }
		ok := func(args ...string) run {
			t.Helper()
			x := call(args...)
			if x.res.Outcome != wire.OutcomeOK {
				t.Fatalf("%v %s", args, x.stdout)
			}
			return x
		}
		ok("init")
		var qualification strings.Builder
		qualification.WriteString(`{"Action":"start","Package":"` + transaction.QualificationPackage + `"}` + "\n")
		for _, name := range transaction.QualificationSuite {
			for _, action := range []string{"run", "pass"} {
				qualification.WriteString(`{"Action":"` + action + `","Package":"` + transaction.QualificationPackage + `","Test":"` + name + `"}` + "\n")
			}
		}
		qualification.WriteString(`{"Action":"pass","Package":"` + transaction.QualificationPackage + `"}` + "\n")
		qp := filepath.Join(t.TempDir(), "synthetic-cutover.jsonl")
		fixture.Write(t, qp, []byte(qualification.String()))
		qp, err = filepath.EvalSymlinks(qp)
		if err != nil {
			t.Fatal(err)
		}
		ok("cutover", "--execution", "--decision", "test-only-lane-untouched", "--qualification", qp)
		id := field(ok("ticket", "create", "--request-id", "ticket", "--payload", createPayloadJSON).res.Items[0], "ticketId").Str
		claim := ok("claim", id, "--holder", "builder", "--request-id", "claim", "--scope", "src", "--pool", "db", "--stage", "review")
		item := claim.res.Items[0]
		a, g := field(item, "attemptId").Str, field(item, "generation").Str
		allocation := field(item, "poolAllocation")
		if field(allocation, "memberId").Str != "review" {
			t.Fatal("wrong member")
		}
		show := ok("attempt", "show", a)
		if field(field(show.res.Items[0], "directPoolAdmission"), "profile").Str != snapshot.ProfileDirectPoolAdmission {
			t.Fatal("origin absent")
		}
		base := []string{"release", "--attempt", a, "--generation", g, "--request-id", "untouched"}
		before := fixture.TreeSnapshot(t, r.StateDir)
		for _, extra := range [][]string{{"--lane-untouched"}, {"--lane-untouched", "--evidence", ""}, {"--lane-untouched", "--lane-untouched", "--evidence", "local:unused"}, {"--lane-untouched", "--evidence", "local:unused", "--role", "WORKER"}} {
			if x := call(append(append([]string{}, base...), extra...)...); x.res.Outcome == wire.OutcomeOK {
				t.Fatal("malformed accepted")
			}
			if !fixture.SameTree(before, fixture.TreeSnapshot(t, r.StateDir)) {
				t.Fatal("malformed request wrote")
			}
		}
		args := append(base, "--lane-untouched", "--evidence", "local:unused")
		released := ok(args...)
		att := field(released.res.Items[0], "laneUntouchedAttestation")
		if field(att, "profile").Str != snapshot.ProfileLaneUntouched || field(att, "physicalFacts").Str != "NOT_OBSERVED" || !bytes.Equal(wire.Encode(allocation), wire.Encode(field(released.res.Items[0], "poolAllocation"))) {
			t.Fatal("missing release payload")
		}
		if _, err := os.Stat(marker); !os.IsNotExist(err) {
			t.Fatal("untouched ran cleanup", err)
		}
		successor := ok("claim", id, "--holder", "builder", "--request-id", "successor", "--scope", "src", "--pool", "db", "--stage", "review")
		next := successor.res.Items[0]
		if field(field(next, "poolAllocation"), "allocationId").Str == field(allocation, "allocationId").Str {
			t.Fatal("allocation reused")
		}
		replay := ok(args...)
		if !field(replay.res.Items[0], "replayed").Bool || !bytes.Equal(wire.Encode(att), wire.Encode(field(replay.res.Items[0], "laneUntouchedAttestation"))) {
			t.Fatal("replay lost original payload")
		}
		// Ordinary terminal release still quarantines, and configured cleanup is mandatory.
		ok("release", "--attempt", field(next, "attemptId").Str, "--generation", field(next, "generation").Str, "--request-id", "ordinary")
		raw, err = os.ReadFile(filepath.Join(r.StateDir, "pools.json"))
		if err != nil {
			t.Fatal(err)
		}
		state, err := snapshot.DecodePools(raw)
		if err != nil || len(state.Entries) != 1 || state.Entries[0].State != "QUARANTINED" {
			t.Fatal("default quarantine changed", err)
		}
		safe := call("pool", "confirm-safe", "--member", "review", "--allocation", string(state.Entries[0].AllocationID), "--request-id", "unsafe-confirm", "--reason", "no cleanup", "--evidence", "local:ref")
		if safe.res.Outcome == wire.OutcomeOK {
			t.Fatal("configured cleanup bypass")
		}
		audit := ok("receipt", "audit")
		if field(audit.res.Items[0], "structuralConsistency").Str != "CONSISTENT" {
			t.Fatal("audit")
		}
	})
}
