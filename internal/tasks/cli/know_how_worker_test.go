package cli_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestKHNV0009_WorkerKnowHowThroughTheCLI: with policy knowHow.workerAdd the
// claim holder, acting as WORKER, adds a note on its claimed ticket naming the
// attempt and generation the claim returned, and the audited attempt record
// refuses a stale generation, an anchor outside touchPaths, another ticket and
// another actor, each with its stable detail prefix.
func TestKHNV0009_WorkerKnowHowThroughTheCLI(t *testing.T) {
	r := exclusionCLIRepo(t)
	policyPath := filepath.Join(r.IntentDir, "policy.json")
	raw, err := os.ReadFile(policyPath)
	if err != nil {
		t.Fatal(err)
	}
	pv, err := wire.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	pv.Obj.Set("knowHow", wire.ObjectValue(wire.NewObject().Set("workerAdd", wire.Bool(true))))
	fixture.Write(t, policyPath, wire.EncodeFile(pv))
	for p, body := range map[string]string{"src/a.go": "package a\n", "docs/x.md": "# x\n"} {
		fixture.Write(t, filepath.Join(r.Root, p), []byte(body))
	}
	git(t, r.Root, "add", "src", "docs")
	git(t, r.Root, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "-m", "files")
	if x := atm(t, r.Root, nil, "init"); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("init: %+v", x.res)
	}
	t.Setenv("CORVINT_TASKS_ACTOR", "agent")
	home := planTicket(t, r.Root, "home", "P1", `["src/"]`)
	other := planTicket(t, r.Root, "other", "P2", `["docs/x.md"]`)
	c := atm(t, r.Root, nil, "claim", home, "--holder", "agent", "--request-id", "claim-home")
	if c.res.Outcome != wire.OutcomeOK {
		t.Fatalf("claim: %s", c.stdout)
	}
	attempt, gen := field(c.res.Items[0], "attemptId").Str, field(c.res.Items[0], "generation").Str
	n, _ := strconv.Atoi(gen)
	add := func(req, target, generation, anchor string) []string {
		return []string{"ticket", "know-how", "add", target, "--request-id", req, "--expected-revision", "1",
			"--issued-at", "2026-10-07T12:00:00Z", "--text", "rebuild the parser before the focused tests",
			"--anchor", anchor, "--attempt", attempt, "--generation", generation, "--role", "WORKER"}
	}
	refusals := []struct {
		name, actor, prefix string
		args                []string
	}{
		{"stale generation", "agent", mutation.KnowHowWorkerAttemptStale, add("w-stale", home, strconv.Itoa(n+1), "src/a.go")},
		{"anchor out of scope", "agent", mutation.KnowHowWorkerAnchorScope, add("w-scope", home, gen, "docs/x.md")},
		{"other ticket", "agent", mutation.KnowHowWorkerOtherTicket, add("w-other", other, gen, "docs/x.md")},
		{"foreign actor", "intruder", mutation.KnowHowWorkerAttemptForeign, add("w-foreign", home, gen, "src/a.go")},
	}
	for _, rc := range refusals {
		t.Setenv("CORVINT_TASKS_ACTOR", rc.actor)
		if x := atm(t, r.Root, nil, rc.args...); x.res.Outcome == wire.OutcomeOK || !strings.Contains(string(x.stdout), rc.prefix) {
			t.Fatalf("%s: %s", rc.name, x.stdout)
		}
	}
	t.Setenv("CORVINT_TASKS_ACTOR", "agent")
	x := atm(t, r.Root, nil, add("w-ok", home, gen, "src/a.go")...)
	if x.res.Outcome != wire.OutcomeOK || field(x.res.Items[0], "outcome").Str != mutation.OutcomeCompleted {
		t.Fatalf("worker add: %s", x.stdout)
	}
	show := atm(t, r.Root, nil, "ticket", "show", home)
	if !strings.Contains(string(show.stdout), `"role":"WORKER"`) || !strings.Contains(string(show.stdout), `"attempt":"`+attempt+`"`) {
		t.Fatalf("ticket show lacks the WORKER entry: %s", show.stdout)
	}
	if x := atm(t, r.Root, nil, "ticket", "know-how", "retract", home, "--request-id", "w-retract", "--expected-revision", "2",
		"--note", "1", "--reason", "wrong", "--role", "WORKER"); x.res.Outcome == wire.OutcomeOK {
		t.Fatalf("WORKER retract accepted: %s", x.stdout)
	}
}

// TestKHNV0008_WorkerKnowHowRefusedWithoutPolicy: without knowHow.workerAdd
// the claim holder's WORKER add is refused UNAUTHORIZED with the same detail
// an unadmitted role always received, and nothing is written.
func TestKHNV0008_WorkerKnowHowRefusedWithoutPolicy(t *testing.T) {
	r := knowHowCLIRepo(t)
	t.Setenv("CORVINT_TASKS_ACTOR", "agent")
	home := planTicket(t, r.Root, "home", "P1", `["src/"]`)
	c := atm(t, r.Root, nil, "claim", home, "--holder", "agent", "--request-id", "claim-home")
	if c.res.Outcome != wire.OutcomeOK {
		t.Fatalf("claim: %s", c.stdout)
	}
	state := fixture.TreeSnapshot(t, r.StateDir)
	x := atm(t, r.Root, nil, "ticket", "know-how", "add", home, "--request-id", "w-1", "--expected-revision", "1",
		"--issued-at", "2026-10-07T12:00:00Z", "--text", "t", "--anchor", "src/a.go",
		"--attempt", field(c.res.Items[0], "attemptId").Str, "--generation", field(c.res.Items[0], "generation").Str, "--role", "WORKER")
	if x.res.Outcome == wire.OutcomeOK || field(x.res.Items[0], "outcome").Str != mutation.OutcomeUnauthorized || !strings.Contains(string(x.stdout), "outside hypothetical role subset") {
		t.Fatalf("WORKER add without the policy key: %s", x.stdout)
	}
	if after := fixture.TreeSnapshot(t, r.StateDir); !reflect.DeepEqual(after, state) {
		t.Fatal("a refused WORKER add changed the state directory")
	}
}
