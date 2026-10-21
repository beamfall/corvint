package cli_test

import (
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestATRV0212_ReadOnlySessionRefusesMutations (V1-1237, V1-1441): with
// CORVINT_TASKS_READ_ONLY set, the mutations a read-only review ran (note
// set as OWNER, attach-evidence, ticket create) and any lease command are
// refused UNSUPPORTED before any store access and write nothing, while pure
// reads still answer. Unset or "0" leaves mutations working.
func TestATRV0212_ReadOnlySessionRefusesMutations(t *testing.T) {
	root, _ := leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
	id := planTicket(t, root, "reviewed", "P1", `["src/reviewed/"]`)
	resolved, err := intent.Resolve(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv(cli.ReadOnlyEnv, "1")
	before := fixture.TreeSnapshot(t, resolved.StateDir)
	for _, args := range [][]string{
		{"ticket", "note", "set", id, "--role", "OWNER", "--request-id", "ro-note", "--text", "x"},
		{"ticket", "attach-evidence", id, "--request-id", "ro-evidence"},
		{"ticket", "create", "--request-id", "ro-create"},
		{"claim", id, "--holder", "reviewer", "--request-id", "ro-claim"},
		{"doctor", "--refresh"},
		{"pause", "--request-id", "ro-pause"},
	} {
		r := handoffCLI(t, root, args...)
		if r.res.Outcome != wire.OutcomeError || !strings.Contains(string(r.stdout), wire.CodeUnsupported) || !strings.Contains(string(r.stdout), cli.ReadOnlyEnv) {
			t.Fatalf("%v: want a read-only refusal: %s", args, r.stdout)
		}
	}
	for _, args := range [][]string{{"ticket", "show", id}, {"queue", "status"}, {"receipt", "audit"}, {"version"}} {
		if r := handoffCLI(t, root, args...); r.res.Outcome != wire.OutcomeOK {
			t.Fatalf("%v: pure read refused under read-only: %s", args, r.stdout)
		}
	}
	if !fixture.SameTree(before, fixture.TreeSnapshot(t, resolved.StateDir)) {
		t.Fatal("a read-only session wrote the store")
	}
	t.Setenv(cli.ReadOnlyEnv, "0")
	if r := handoffCLI(t, root, "claim", id, "--holder", "builder", "--request-id", "rw-claim"); r.res.Outcome != wire.OutcomeOK {
		t.Fatalf("claim with %s=0: %s", cli.ReadOnlyEnv, r.stdout)
	}
}
