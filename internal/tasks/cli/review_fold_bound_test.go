package cli_test

import (
	"fmt"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/dispatch"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestERGV0870_ReviewFoldIsBoundedByTheRetireAnchor (ERG-V0-870, V1-0907):
// two stores whose histories differ only before the retire anchor reach
// different head sequences, yet the review binding fold walks the same
// receipts after the anchor and reports the same gate state.
func TestERGV0870_ReviewFoldIsBoundedByTheRetireAnchor(t *testing.T) {
	t.Setenv("CORVINT_TASKS_ACTOR", "tester")
	t.Setenv("ATM_ACTOR", "tester")
	type measure struct{ head, folded uint64 }
	measureAt := func(filler int) measure {
		root, tree := ergStore(t)
		for i := range filler {
			planTicket(t, root, fmt.Sprintf("filler %d", i), "P3", `["docs/"]`)
		}
		id := planTicket(t, root, "reviewed", "P1", `["src/"]`)
		runOK := func(args ...string) run {
			t.Helper()
			x := atm(t, root, nil, args...)
			if x.res.Outcome != wire.OutcomeOK {
				t.Fatalf("%v: %s", args, x.stdout)
			}
			return x
		}
		runOK("receipt", "retire", "--archive", filepath.Join(t.TempDir(), "retired.tar"))
		c := runOK("claim", id, "--holder", "tester", "--stage", "implement", "--request-id", "claim-a")
		runOK("submit", "--attempt", field(c.res.Items[0], "attemptId").Str, "--generation", field(c.res.Items[0], "generation").Str, "--tree", tree, "--request-id", "submit-a")
		subject := field(runOK("receipt", "audit").res.Items[0], "headSeq").Str
		runOK("gate", "record", id, "--gate", ergGate, "--verdict", "PASS", "--subject-receipt", subject,
			"--expected-generation", "0", "--expected-revision", "0", "--request-id", "review-1")
		if _, state := ergGateView(t, root, id); state != dispatch.GatePass {
			t.Fatalf("filler %d: gate %s, want PASS", filler, state)
		}
		if g := runOK("gate", "state", id).res.Items[0]; field(g, "verdict").Str != "PASS" || field(g, "status").Str != "CURRENT" {
			t.Fatalf("filler %d: gate state %v", filler, g)
		}
		a := runOK("receipt", "audit").res.Items[0]
		head, _ := strconv.ParseUint(field(a, "headSeq").Str, 10, 64)
		anchor, _ := strconv.ParseUint(field(a, "anchorSeq").Str, 10, 64)
		if anchor == 0 || head <= anchor {
			t.Fatalf("filler %d: anchor %d head %d", filler, anchor, head)
		}
		return measure{head, head - anchor}
	}
	small, large := measureAt(2), measureAt(12)
	if large.head <= small.head {
		t.Fatalf("histories did not differ: %d vs %d receipts", small.head, large.head)
	}
	if small.folded != large.folded {
		t.Fatalf("fold walks %d receipts at history %d but %d at history %d", small.folded, small.head, large.folded, large.head)
	}
}
