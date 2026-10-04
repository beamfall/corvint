package cli_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// CAL-V0-044/046: the command boundary uses audited policy history for an
// evidence-only review return without rewriting the claim's policy hashes.
func TestCALV0044_CLIUnrelatedPoolPolicyReturn(t *testing.T) {
	root, _ := leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
	runOK := func(args ...string) run {
		t.Helper()
		r := handoffCLI(t, root, args...)
		if r.res.Outcome != wire.OutcomeOK {
			t.Fatalf("%v: %s", args, r.stdout)
		}
		return r
	}
	loaded, err := intent.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	v, err := wire.Parse(loaded.Policy.Raw)
	if err != nil {
		t.Fatal(err)
	}
	v.Obj.Set("policyVersion", wire.String("2"))
	v.Obj.Set("pools", wire.Array(wire.ObjectValue(wire.NewObject().Set("id", wire.String("lanes")).Set("members", wire.Strings([]string{"a", "b"})))))
	policyDir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	policyFile := filepath.Join(policyDir, "policy.json")
	if err := os.WriteFile(policyFile, wire.EncodeFile(v), 0600); err != nil {
		t.Fatal(err)
	}
	runOK("policy", "update", "--request-id", "pool-policy", "--expected-policy-version", "1", "--file", policyFile)
	id := planTicket(t, root, "policy-handoff", "P1", `["src/"]`)
	c := runOK("claim", id, "--holder", "reviewer", "--stage", "review", "--pool", "lanes", "--request-id", "claim")
	a, g := field(c.res.Items[0], "attemptId").Str, field(c.res.Items[0], "generation").Str
	before := runOK("attempt", "show", a)
	v.Obj.Set("policyVersion", wire.String("3"))
	pools, _ := v.Obj.Get("pools")
	pools.Arr[0].Obj.Set("reservedFor", wire.ObjectValue(wire.NewObject().Set("b", wire.String("implement"))))
	if err := os.WriteFile(policyFile, wire.EncodeFile(v), 0600); err != nil {
		t.Fatal(err)
	}
	runOK("policy", "update", "--request-id", "other-reservation", "--expected-policy-version", "2", "--file", policyFile)
	args := []string{"release", "--attempt", a, "--generation", g, "--request-id", "return", "--reason", wire.CodeReviewReturned, "--evidence", "external-review"}
	runOK(args...)
	after := runOK("attempt", "show", a)
	for _, key := range []string{"policySha256", "configSha256", "capabilityProfileSha256", "retryCount"} {
		if !bytes.Equal(wire.Encode(field(before.res.Items[0], key)), wire.Encode(field(after.res.Items[0], key))) {
			t.Fatalf("return rewrote %s", key)
		}
	}
	if field(after.res.Items[0], "phase").Str != "CANCELLED" || field(after.res.Items[0], "quiescence").Str != "FENCED" {
		t.Fatalf("return: %s", after.stdout)
	}
	runOK(args...)
	runOK("receipt", "audit")
}

// An optional compiled binary drives the same lifecycle as the in-process CLI.
// Qualification supplies the exact new binary and pinned old reader explicitly.
func handoffCLI(t *testing.T, root string, args ...string) run {
	t.Helper()
	return handoffCLIWithBinary(t, root, os.Getenv("CORVINT_HANDOFF_TEST_BINARY"), args...)
}

func handoffCLIWithBinary(t *testing.T, root, binary string, args ...string) run {
	t.Helper()
	if binary == "" {
		return atm(t, root, nil, args...)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.WaitDelay = time.Second
	cmd.Dir = root
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	err := cmd.Run()
	code := 0
	if err != nil {
		if e, ok := err.(*exec.ExitError); ok {
			code = e.ExitCode()
		} else {
			t.Fatal(err)
		}
	}
	res, e := wire.DecodeResult(out.Bytes())
	if e != nil {
		t.Fatalf("%v: %v %s %s", args, e, out.Bytes(), errb.Bytes())
	}
	return run{code: code, stdout: out.Bytes(), stderr: errb.Bytes(), res: res}
}

// CAL-V0-044: real CLI claims and pure preview agree throughout handoffs,
// preserve failures at count three, and do not make old readers silently accept
// new metadata. The disposable store is fixture-profile, not live migration.
func TestCALV0044_CLIHandoffAccounting(t *testing.T) {
	t.Run("CAL-V0-044 CLIHandoffAccounting", func(t *testing.T) {
		root, _ := leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
		id := planTicket(t, root, "handoff", "P1", `["src/"]`)
		repo, err := intent.Resolve(root)
		if err != nil {
			t.Fatal(err)
		}
		treeRaw, err := exec.Command("git", "-C", root, "rev-parse", "HEAD^{tree}").Output()
		if err != nil {
			t.Fatal(err)
		}
		tree := strings.TrimSpace(string(treeRaw))
		runOK := func(args ...string) run {
			t.Helper()
			r := handoffCLI(t, root, args...)
			if r.res.Outcome != wire.OutcomeOK {
				t.Fatalf("%v: %s", args, r.stdout)
			}
			return r
		}
		preview := func(want string) {
			t.Helper()
			before := fixture.TreeSnapshot(t, repo.StateDir)
			r := runOK("plan", "preview")
			entries := field(r.res.Items[0], "entries").Arr
			if len(entries) != 1 || field(entries[0], "state").Str != want {
				t.Fatalf("plan: %s", r.stdout)
			}
			if !fixture.SameTree(before, fixture.TreeSnapshot(t, repo.StateDir)) {
				t.Fatal("preview mutated store")
			}
		}
		var attempt, generation string
		for i := 0; i < 12; i++ {
			preview("SELECTED")
			stage, reason := "implement", wire.CodeHandoff
			if i%2 == 1 {
				stage, reason = "review", wire.CodeReviewReturned
			}
			r := runOK("claim", id, "--holder", fmt.Sprintf("worker-%d", i), "--stage", stage, "--request-id", fmt.Sprintf("claim-%d", i))
			attempt, generation = field(r.res.Items[0], "attemptId").Str, field(r.res.Items[0], "generation").Str
			show := runOK("attempt", "show", attempt)
			expected := 0
			if i >= 6 {
				expected = i - 6
				if expected > 3 {
					expected = 3
				}
			}
			if field(show.res.Items[0], "retryCount").Str != fmt.Sprint(expected) {
				t.Fatalf("retry debt: %s", show.stdout)
			}
			if i == 0 {
				if old := os.Getenv("CORVINT_HANDOFF_TEST_OLD_BINARY"); old != "" {
					before := fixture.TreeSnapshot(t, repo.StateDir)
					cmd := exec.Command(old, "plan", "preview")
					cmd.Dir = root
					raw, err := cmd.Output()
					if err == nil {
						t.Fatalf("old planner accepted first accounting claim: %s", raw)
					}
					if !fixture.SameTree(before, fixture.TreeSnapshot(t, repo.StateDir)) {
						t.Fatal("old reader mutated store")
					}
				}
			}
			runOK("submit", "--attempt", attempt, "--generation", generation, "--tree", tree, "--request-id", fmt.Sprintf("submit-%d", i))
			if (i >= 6 && i < 9) || i == 11 {
				reason = wire.CodeContaminated
			}
			runOK("release", "--attempt", attempt, "--generation", generation, "--reason", reason, "--request-id", fmt.Sprintf("release-%d", i))
		}
		preview("BLOCKED")
		r := handoffCLI(t, root, "claim", id, "--holder", "last", "--stage", "review", "--request-id", "exhausted")
		if !hasCode(r.res, wire.CodeRetryExhausted) {
			t.Fatalf("expected exhaustion: %s", r.stdout)
		}
		runOK("receipt", "audit")
	})
}
