package cli_test

import (
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestATRV0211_ClaimResultListsEffectiveScope (V1-1321): a claim result
// names the effective reserved scope, a requested path outside the ticket's
// declared scope refuses OUT_OF_SCOPE naming it (ATR-V0-210) for both
// --scope argv forms, and a refusal carries scope null.
func TestATRV0211_ClaimResultListsEffectiveScope(t *testing.T) {
	root, _ := leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
	id := planTicket(t, root, "scoped", "P1", `["src/scoped/"]`)
	for i, args := range [][]string{
		{"--scope", "src/scoped/a.go", "cmd/x/golden.json"},
		{"--scope", "src/scoped/a.go", "--scope", "cmd/x/golden.json"},
	} {
		r := handoffCLI(t, root, append([]string{"claim", id, "--holder", "builder", "--request-id", "refused-" + string(rune('a'+i))}, args...)...)
		if r.res.Outcome == wire.OutcomeOK || len(r.res.Items) != 1 || !strings.Contains(string(r.stdout), "OUT_OF_SCOPE") || !strings.Contains(string(r.stdout), "cmd/x/golden.json") {
			t.Fatalf("%v: want OUT_OF_SCOPE naming the dropped path: %s", args, r.stdout)
		}
		if sc, ok := r.res.Items[0].Obj.Get("scope"); !ok || sc.Kind != wire.KindNull {
			t.Fatalf("%v: refused claim lacks scope null: %s", args, r.stdout)
		}
	}
	c := handoffCLI(t, root, "claim", id, "--holder", "builder", "--request-id", "ok", "--scope", "src/scoped/a.go")
	if c.res.Outcome != wire.OutcomeOK {
		t.Fatalf("covered claim: %s", c.stdout)
	}
	sc := field(c.res.Items[0], "scope")
	res := field(sc, "resources")
	if field(sc, "source").Str != "DECLARED" || len(res.Arr) != 1 || field(res.Arr[0], "key").Str != "src/scoped/" || field(res.Arr[0], "class").Str != "PATH" {
		t.Fatalf("claim result scope: %s", c.stdout)
	}
	if h := handoffCLI(t, root, "claim", "--help", "--verbose"); !strings.Contains(string(h.stdout), "ATR-V0-210") || !strings.Contains(string(h.stdout), "ATR-V0-211") {
		t.Fatalf("claim verbose help omits the scope rule: %s", h.stdout)
	}
}
