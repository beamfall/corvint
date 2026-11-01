package cli_test

import (
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestTCDV0966_SearchAllListsEveryTicket covers V1-3645: --all lists every
// ticket in pages of --limit, refuses a filter or a repeat, and a search with
// no filter names --all in its refusal (TCD-V0-966).
func TestTCDV0966_SearchAllListsEveryTicket(t *testing.T) {
	t.Setenv("CORVINT_TASKS_ACTOR", "tester")
	root, _ := leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
	for _, title := range []string{"alpha", "bravo", "charlie"} {
		tcdTicket(t, root, title, "")
	}
	all := atm(t, root, nil, "ticket", "search", "--all")
	if all.res.Outcome != wire.OutcomeOK || len(all.res.Items) < 3 {
		t.Fatalf("search --all: %s", all.stdout)
	}
	page := atm(t, root, nil, "ticket", "search", "--all", "--limit", "2")
	if page.res.Outcome != wire.OutcomeOK || len(page.res.Items) != 2 || page.res.Page == nil {
		t.Fatalf("search --all --limit 2: %s", page.stdout)
	}
	if x := atm(t, root, nil, "ticket", "search", "--all", "--kind", "BUG"); x.res.Outcome == wire.OutcomeOK || !wire.HasCode(x.res, wire.CodeMalformed) {
		t.Fatalf("--all with a filter: %s", x.stdout)
	}
	if x := atm(t, root, nil, "ticket", "search", "--all", "--all"); x.res.Outcome == wire.OutcomeOK {
		t.Fatalf("repeated --all: %s", x.stdout)
	}
	if x := atm(t, root, nil, "ticket", "search", "--text", "--all"); x.res.Outcome == wire.OutcomeOK && len(x.res.Items) != 0 {
		t.Fatalf("--text value --all was taken as the flag: %s", x.stdout)
	}
	none := atm(t, root, nil, "ticket", "search")
	if none.res.Outcome == wire.OutcomeOK || !strings.Contains(strings.Join(none.res.Warnings, " "), "--all") {
		t.Fatalf("unfiltered search refusal does not name --all: %s", none.stdout)
	}
	help := atm(t, root, nil, "help", "ticket", "search")
	if !strings.Contains(string(help.stdout), "--all") {
		t.Fatalf("help does not document --all: %s", help.stdout)
	}
}
