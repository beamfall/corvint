package cli

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// CAL-V0-215: the snapshot boundary fails before the acquire writer; the
// fault is per invocation so no global OS limit or shared hook is changed.
func TestCALV0215_PoolAcquireResourceExhausted(t *testing.T) {
	t.Setenv("CORVINT_TASKS_ACTOR", fixture.Actor)
	t.Setenv("ATM_ACTOR", "")
	r := fixture.TempRepo(t)
	fixture.WriteIntent(t, r)
	policy := fixture.PolicyValue()
	policy.Obj.Vals["budgets"].Obj.Set("requireEnforcedFields", wire.Array())
	policy.Obj.Set("pools", wire.Array(wire.ObjectValue(wire.NewObject().Set("id", wire.String("db")).Set("members", wire.Strings([]string{"a"})))))
	fixture.Write(t, filepath.Join(r.IntentDir, "policy.json"), wire.EncodeFile(policy))
	for _, args := range [][]string{{"init", "-b", "main"}, {"-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "--allow-empty", "-m", "base"}} {
		c := exec.Command("git", args...)
		c.Dir = r.Root
		if b, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git fixture: %s %v", b, err)
		}
	}
	run := func(env Env, args ...string) (*wire.Result, string) {
		t.Helper()
		var out bytes.Buffer
		env.Cwd, env.Args, env.Stdin, env.Stdout, env.Stderr = r.Root, args, strings.NewReader(""), &out, &out
		Run(env)
		res, err := wire.DecodeResult(out.Bytes())
		if err != nil {
			t.Fatalf("decode: %s %v", out.Bytes(), err)
		}
		return res, out.String()
	}
	for _, args := range [][]string{{"init"}, {"receipt", "audit"}} {
		if res, raw := run(Env{}, args...); res.Outcome != wire.OutcomeOK {
			t.Fatalf("fixture: %s", raw)
		}
	}
	payload, err := os.ReadFile("testdata/external-agents/ticket-create.json")
	if err != nil {
		t.Fatal(err)
	}
	payload = []byte(strings.NewReplacer("queue:example:main", fixture.QueueID, `"coverage":"INCOMPLETE"`, `"coverage":"QUALIFIED"`, `"requiredGates":["verify"]`, `"requiredGates":[]`).Replace(string(payload)))
	created, raw := run(Env{}, "ticket", "create", "--request-id", "create", "--payload", string(payload))
	if created.Outcome != wire.OutcomeOK {
		t.Fatalf("create: %s", raw)
	}
	ticketID := created.Items[0].Obj.Vals["ticketId"].Str
	claim, raw := run(Env{}, "claim", ticketID, "--holder", "builder", "--request-id", "claim", "--stage", "implement")
	if claim.Outcome != wire.OutcomeOK {
		t.Fatalf("claim: %s", raw)
	}
	item := claim.Items[0].Obj
	args := []string{"pool", "acquire", "--attempt", item.Vals["attemptId"].Str, "--generation", item.Vals["generation"].Str, "--pool", "db", "--request-id", "acquire"}
	capture := func() [][]fixture.Entry {
		t.Helper()
		return [][]fixture.Entry{fixture.TreeSnapshot(t, r.StateDir), fixture.TreeSnapshot(t, r.IntentDir)}
	}
	// Prove the real probe and canonical envelope before fault injection.
	if _, err := snapshot.Probe(r.StateDir); err != nil {
		t.Fatal(err)
	}
	before := capture()
	for _, errno := range []error{syscall.EMFILE, syscall.ENFILE} {
		t.Run(errno.Error(), func(t *testing.T) {
			reached := 0
			env := Env{leaseProbe: func(path string) (*snapshot.Snapshot, error) {
				reached++
				if path != r.StateDir {
					t.Fatalf("probe path: %s", path)
				}
				return nil, fmt.Errorf("probe: %w", &os.PathError{Op: "open", Path: filepath.Join(path, "head.json"), Err: errno})
			}}
			res, raw := run(env, args...)
			if reached != 1 {
				t.Fatalf("fault reached %d times", reached)
			}
			if res.Outcome != wire.OutcomeError || !reflect.DeepEqual(res.Codes, []string{wire.CodeResourceExhausted}) || !strings.Contains(raw, `"retryable":true`) || !strings.Contains(raw, `"mutation":null`) {
				t.Fatalf("result: %s", raw)
			}
			if !reflect.DeepEqual(before, capture()) {
				t.Fatal("failed acquire changed store or journal")
			}
		})
	}
	// The identical well-formed request succeeds once the injected pressure clears.
	if res, raw := run(Env{}, args...); res.Outcome != wire.OutcomeOK {
		t.Fatalf("retry: %s", raw)
	}
}

// CAL-V0-215: execution, committed-step and propagated error marks still win.
func TestCALV0215_RetryOverrides(t *testing.T) {
	for _, errno := range []error{syscall.EMFILE, syscall.ENFILE} {
		marked := wire.WithoutRetry(errno)
		for name, res := range map[string]*wire.Result{
			"program": errorResult([]string{"run"}, marked),
			"read":    failure([]string{"queue", "status"}, nil, marked),
			"lease":   unretryableResult([]string{"pool", "acquire"}, &store.Report{Unretryable: true}, errno),
			"sweep":   sweepErrorResult([]string{"pool", "sweep"}, &store.PoolSweepReport{Unretryable: true}, errno),
			"batch":   laneFailureResult([]string{"run"}, errno, []error{errno, marked}),
		} {
			raw, err := res.Encode()
			if err != nil || !strings.Contains(string(raw), `"codes":["RESOURCE_EXHAUSTED"]`) || !strings.Contains(string(raw), `"retryable":false`) {
				t.Fatalf("%s: %s %v", name, raw, err)
			}
		}
	}
}
