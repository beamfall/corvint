package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/obligation"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestTCDV0165_GateRunReportsFailingTail: through the CLI, a gate run that
// executes a failing gate reports its state, exit, a bounded secret-screened
// tail of its output and the failing line, and warns where the full output
// is retained; a passing gate reports a null tail (TCD-V0-165). The policy
// gate declares `evidence: []`, and the full output is still retained at the
// reported path (TCD-V0-166).
func TestTCDV0165_GateRunReportsFailingTail(t *testing.T) {
	t.Setenv("CORVINT_TASKS_ACTOR", "tester")
	t.Setenv("ATM_ACTOR", "tester")
	root, tree := ergStore(t)
	dir, err := os.MkdirTemp("", "tail")
	if err == nil {
		t.Cleanup(func() { os.RemoveAll(dir) })
		dir, err = filepath.EvalSymlinks(dir)
	}
	if err != nil {
		t.Fatal(err)
	}
	// The token is assembled at run time so no secret-shaped literal is
	// committed.
	token := "sk" + "-" + strings.Repeat("q", 24)
	fails := commandGate("docs", "/bin/sh", "-c", `printf '%s\n' building "token $0" 'line-citations: x.md:3 stale' 'make: *** [line-citations-check] Error 1'; exit 2`, token)
	passes := commandGate("unit", "/bin/sh", "-c", `printf 'ok\n'`)
	ergPolicyUpdateRoles(t, root, "3", []string{"implement"}, []string{"review"}, []string{"OPERATOR", "OWNER"}, false, fails, passes)
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
	id := planTicket(t, root, "tail", "P1", `["src/"]`)
	c := runOK("claim", id, "--holder", "tester", "--stage", "implement", "--request-id", "claim-tail")
	attempt, gen := field(c.res.Items[0], "attemptId").Str, field(c.res.Items[0], "generation").Str
	runOK("submit", "--attempt", attempt, "--generation", gen, "--tree", tree, "--request-id", "submit-tail")
	gateRun := func(gate string) run {
		t.Helper()
		return runOK("gate", "run", "--attempt", attempt, "--generation", gen, "--gate", gate, "--request-id", "gate-"+gate, "--worktree", clean)
	}

	r := gateRun("docs")
	g := field(r.res.Items[0], "gateResult")
	if g.Kind != wire.KindObject || field(g, "state").Str != "FAILED" || field(g, "exitCode").Str != "2" {
		t.Fatalf("the failing gate run reports gateResult %s", wire.EncodeFile(g))
	}
	failing := "make: *** [line-citations-check] Error 1"
	if field(g, "failingLine").Str != failing {
		t.Fatalf("failingLine = %s", wire.EncodeFile(field(g, "failingLine")))
	}
	var tail []string
	for _, v := range field(g, "tail").Arr {
		tail = append(tail, v.Str)
	}
	want := []string{"building", obligation.ExcerptWithheld, "line-citations: x.md:3 stale", failing}
	if strings.Join(tail, "\n") != strings.Join(want, "\n") {
		t.Fatalf("tail = %q, want %q", tail, want)
	}
	if strings.Contains(string(r.stdout), token) {
		t.Fatalf("the gate run result quotes the secret: %s", r.stdout)
	}
	out := field(g, "output")
	path := field(out, "path").Str
	if path != "evidence/"+field(out, "sha256").Str {
		t.Fatalf("output = %s", wire.EncodeFile(out))
	}
	warned := strings.Join(r.res.Warnings, "\n")
	if !strings.Contains(warned, "gate docs FAILED") || !strings.Contains(warned, failing) || !strings.Contains(warned, path) {
		t.Fatalf("warnings = %q", r.res.Warnings)
	}
	repo, err := intent.Resolve(root)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(repo.StateDir, filepath.FromSlash(path)))
	if err != nil || !strings.Contains(string(raw), "token "+token) || !strings.HasSuffix(string(raw), failing+"\n") {
		t.Fatalf("the retained output is %q: %v", raw, err)
	}

	r = gateRun("unit")
	g = field(r.res.Items[0], "gateResult")
	if field(g, "state").Str != "PASSED" || field(g, "tail").Kind != wire.KindNull || field(g, "failingLine").Kind != wire.KindNull {
		t.Fatalf("the passing gate run reports gateResult %s", wire.EncodeFile(g))
	}
	for _, w := range r.res.Warnings {
		if strings.Contains(w, "TCD-V0-165") {
			t.Fatalf("a passing gate run warns %q", w)
		}
	}
	// A replay of the same request executes nothing in this call.
	if g := field(gateRun("docs").res.Items[0], "gateResult"); g.Kind != wire.KindNull {
		t.Fatalf("a replayed gate run reports gateResult %s", wire.EncodeFile(g))
	}
	runOK("receipt", "audit")
}
