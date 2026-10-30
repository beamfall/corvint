package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestCALV0240_GateRunReportsReusedFrom: through the CLI, two tickets'
// attempts at one candidate tree share one execution of a reusable gate.
// The executing `gate run` reports `reusedFrom: null`; the second reports
// the source attempt and a result digest, and the receipt audit passes.
func TestCALV0240_GateRunReportsReusedFrom(t *testing.T) {
	t.Setenv("CORVINT_TASKS_ACTOR", "tester")
	t.Setenv("ATM_ACTOR", "tester")
	root, tree := ergStore(t)
	// A gate argv entry is at most 128 bytes, so the run counter and the
	// clean worktree live under a short directory.
	dir, err := os.MkdirTemp("", "reuse")
	if err == nil {
		t.Cleanup(func() { os.RemoveAll(dir) })
		dir, err = filepath.EvalSymlinks(dir)
	}
	if err != nil {
		t.Fatal(err)
	}
	counter := filepath.Join(dir, "runs")
	gate := commandGate("verify", "/bin/sh", "-c", `printf x >> "$0"; printf ok`, counter)
	gate.Obj.Set("reusable", wire.Bool(true))
	ergPolicyUpdateRoles(t, root, "3", []string{"implement"}, []string{"review"}, []string{"OPERATOR", "OWNER"}, false, gate)
	runOK := func(args ...string) run {
		t.Helper()
		r := atm(t, root, nil, args...)
		if r.res.Outcome != wire.OutcomeOK {
			t.Fatalf("%v: %s", args, r.stdout)
		}
		return r
	}
	clean := filepath.Join(dir, "clean")
	git(t, root, "worktree", "add", "--detach", clean, "HEAD")
	type held struct{ attempt, gen string }
	attempts := []held{}
	for _, name := range []string{"src", "lib"} {
		id := planTicket(t, root, "reuse-"+name, "P1", `["`+name+`/"]`)
		c := runOK("claim", id, "--holder", "tester", "--stage", "implement", "--request-id", "claim-"+name)
		h := held{field(c.res.Items[0], "attemptId").Str, field(c.res.Items[0], "generation").Str}
		runOK("submit", "--attempt", h.attempt, "--generation", h.gen, "--tree", tree, "--request-id", "submit-"+name)
		attempts = append(attempts, h)
	}
	gateRun := func(h held, req string) wire.Value {
		t.Helper()
		return runOK("gate", "run", "--attempt", h.attempt, "--generation", h.gen, "--gate", "verify", "--request-id", req, "--worktree", clean).res.Items[0]
	}
	if r := field(gateRun(attempts[0], "gate-src"), "reusedFrom"); r.Kind != wire.KindNull {
		t.Fatalf("an executed gate run reports reusedFrom %s", wire.EncodeFile(r))
	}
	r := field(gateRun(attempts[1], "gate-lib"), "reusedFrom")
	if r.Kind != wire.KindObject || field(r, "attemptId").Str != attempts[0].attempt || len(field(r, "result").Str) != 64 {
		t.Fatalf("the reusing gate run reports reusedFrom %s", wire.EncodeFile(r))
	}
	if raw, err := os.ReadFile(counter); err != nil || string(raw) != "x" {
		t.Fatalf("the gate ran %q times: %v", raw, err)
	}
	runOK("receipt", "audit")
}
