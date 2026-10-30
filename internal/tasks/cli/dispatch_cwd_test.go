package cli_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestATRV0341_ObservationLooksUpTheTicketWorktree: the native dispatcher
// observation offers a ticket-worktree lookup only when the policy declares
// worktrees; it finds nothing before `worktree ensure` and the ensured
// worktree after it.
func TestATRV0341_ObservationLooksUpTheTicketWorktree(t *testing.T) {
	plain := policyCLIRepo(t, func(wire.Value) {})
	obs, err := cli.DispatchQueueForTest(plain.Root).Observe(context.Background())
	if err != nil || obs.TicketWorktree != nil {
		t.Fatalf("undeclared worktrees: lookup set %v, err %v", obs != nil && obs.TicketWorktree != nil, err)
	}
	r := policyCLIRepo(t, func(p wire.Value) {
		p.Obj.Set("worktrees", objectOf(t, `{"repository":".","base":"main","branchPattern":"ticket/{ticket}"}`))
	})
	id := planTicket(t, r.Root, "wt", "P2", `["src/a.go"]`)
	local := id[strings.LastIndex(id, ":")+1:]
	obs, err = cli.DispatchQueueForTest(r.Root).Observe(context.Background())
	if err != nil || obs.TicketWorktree == nil {
		t.Fatalf("declared worktrees: no lookup, err %v", err)
	}
	if got := obs.TicketWorktree(local); got != "" {
		t.Fatalf("before ensure: %q", got)
	}
	x := atm(t, r.Root, nil, "worktree", "ensure", id)
	if x.res.Outcome != wire.OutcomeOK || len(x.res.Items) != 1 {
		t.Fatalf("ensure: %s", x.stdout)
	}
	want, err := filepath.EvalSymlinks(field(x.res.Items[0], "path").Str)
	if err != nil {
		t.Fatal(err)
	}
	if got := obs.TicketWorktree(local); got != want {
		t.Fatalf("after ensure: %q, want %q", got, want)
	}
}
