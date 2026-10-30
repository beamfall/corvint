package cli_test

import (
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestATRV0214_OutOfScopeClaimNamesDeclaredPathsAndRetry (V1-2709, GitHub
// #1818): an OUT_OF_SCOPE claim names the declared and rejected paths and
// the retry (ATR-V0-214), a requested subset of the declared scope is
// admitted (ATR-V0-215), and ticket show exposes the declared scope
// (ATR-V0-216).
func TestATRV0214_OutOfScopeClaimNamesDeclaredPathsAndRetry(t *testing.T) {
	root, _ := leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
	id := planTicket(t, root, "scoped", "P1", `["src/scoped/"]`)
	open := planTicket(t, root, "open", "P2", `[]`)
	if d := field(handoffCLI(t, root, "ticket", "show", id).res.Items[0], "declaredScope"); d.Kind != wire.KindArray || len(d.Arr) != 1 || d.Arr[0].Str != "src/scoped/" {
		t.Fatalf("show declaredScope %s", wire.Encode(d))
	}
	if d, ok := handoffCLI(t, root, "ticket", "show", open).res.Items[0].Obj.Get("declaredScope"); !ok || d.Kind != wire.KindNull {
		t.Fatalf("undeclared ticket declaredScope %s", wire.Encode(d))
	}
	if d, ok := handoffCLI(t, root, "ticket", "blockers", id).res.Items[0].Obj.Get("declaredScope"); ok {
		t.Fatalf("ticket blockers carries declaredScope %s", wire.Encode(d))
	}
	r := handoffCLI(t, root, "claim", id, "--holder", "builder", "--request-id", "refused", "--scope", "src/scoped/a.go", "cmd/x/golden.json", "src/scoped")
	if !hasCode(r.res, wire.CodeOutOfScope) || len(r.res.Items) != 1 {
		t.Fatalf("want OUT_OF_SCOPE: %s", r.stdout)
	}
	sr := field(r.res.Items[0], "scopeRefusal")
	declared, rejected := field(sr, "declared"), field(sr, "rejected")
	if len(declared.Arr) != 1 || declared.Arr[0].Str != "src/scoped/" || len(rejected.Arr) != 2 || rejected.Arr[0].Str != "cmd/x/golden.json" || rejected.Arr[1].Str != "src/scoped" || field(sr, "nextAction").Str != transaction.ScopeRetry {
		t.Fatalf("scopeRefusal %s", wire.Encode(sr))
	}
	for _, want := range []string{"declared paths: src/scoped/", "src/scoped -> src/scoped/", "retry the claim without --scope"} {
		if !strings.Contains(string(r.stdout), want) {
			t.Fatalf("refusal detail lacks %q: %s", want, r.stdout)
		}
	}
	c := handoffCLI(t, root, "claim", id, "--holder", "builder", "--request-id", "subset", "--scope", "src/scoped/", "src/scoped/a.go", "src/scoped/b/")
	if c.res.Outcome != wire.OutcomeOK {
		t.Fatalf("subset claim refused: %s", c.stdout)
	}
	if _, ok := c.res.Items[0].Obj.Get("scopeRefusal"); ok || field(field(c.res.Items[0], "scope"), "source").Str != "DECLARED" {
		t.Fatalf("subset claim result %s", c.stdout)
	}
}
