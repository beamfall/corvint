package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/obligation"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestCALV0322_ShowReportsLastFailedGateTail: after a GATE_FAILED run,
// `attempt show` and `ticket show` both re-show the failed gate with the
// TCD-V0-165 bounded, secret-screened tail of its retained output, and a
// later passing gate does not hide it. Before a gate fails both report null
// (V1-2359).
func TestCALV0322_ShowReportsLastFailedGateTail(t *testing.T) {
	t.Setenv("CORVINT_TASKS_ACTOR", "tester")
	t.Setenv("ATM_ACTOR", "tester")
	root, tree := ergStore(t)
	dir, err := os.MkdirTemp("", "failedgate")
	if err == nil {
		t.Cleanup(func() { os.RemoveAll(dir) })
		dir, err = filepath.EvalSymlinks(dir)
	}
	if err != nil {
		t.Fatal(err)
	}
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
	id := planTicket(t, root, "failedgate", "P1", `["src/"]`)
	c := runOK("claim", id, "--holder", "tester", "--stage", "implement", "--request-id", "claim-failedgate")
	attempt, gen := field(c.res.Items[0], "attemptId").Str, field(c.res.Items[0], "generation").Str
	runOK("submit", "--attempt", attempt, "--generation", gen, "--tree", tree, "--request-id", "submit-failedgate")
	shown := func() (wire.Value, wire.Value, string) {
		t.Helper()
		a := runOK("attempt", "show", attempt)
		tk := runOK("ticket", "show", id)
		return field(a.res.Items[0], "lastFailedGate"), field(tk.res.Items[0], "lastFailedGate"), string(a.stdout) + string(tk.stdout)
	}
	if a, tk, _ := shown(); a.Kind != wire.KindNull || tk.Kind != wire.KindNull {
		t.Fatalf("before any gate failed: attempt %s ticket %s", wire.EncodeFile(a), wire.EncodeFile(tk))
	}
	gateRun := func(gate string) {
		t.Helper()
		runOK("gate", "run", "--attempt", attempt, "--generation", gen, "--gate", gate, "--request-id", "gate-"+gate, "--worktree", clean)
	}
	gateRun("docs")
	gateRun("unit")

	failing := "make: *** [line-citations-check] Error 1"
	want := []string{"building", obligation.ExcerptWithheld, "line-citations: x.md:3 stale", failing}
	a, tk, stdout := shown()
	for name, g := range map[string]wire.Value{"attempt show": a, "ticket show": tk} {
		if g.Kind != wire.KindObject || field(g, "gateId").Str != "docs" || field(g, "state").Str != "FAILED" || field(g, "exitCode").Str != "2" || field(g, "attemptId").Str != attempt {
			t.Fatalf("%s lastFailedGate %s", name, wire.EncodeFile(g))
		}
		if field(g, "failingLine").Str != failing || field(field(g, "output"), "observed").Str != "OBSERVED" {
			t.Fatalf("%s lastFailedGate %s", name, wire.EncodeFile(g))
		}
		var tail []string
		for _, v := range field(g, "tail").Arr {
			tail = append(tail, v.Str)
		}
		if strings.Join(tail, "\n") != strings.Join(want, "\n") {
			t.Fatalf("%s tail = %q, want %q", name, tail, want)
		}
	}
	if strings.Contains(stdout, token) {
		t.Fatalf("a show result quotes the secret: %s", stdout)
	}

}
