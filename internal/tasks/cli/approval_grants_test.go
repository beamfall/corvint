package cli_test

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestATRV0272_ApprovalGrantsNamedUpFront: an APPROVAL_REQUIRED ticket names
// every grant it needs through completion before any claim (ticket blockers
// and ticket show missingGrants, ATR-V0-273), the APPROVAL_MISSING claim
// refusal names RUN and COMPLETE together (ATR-V0-272), and a claim that a
// RUN grant admits warns that COMPLETE is still missing (ATR-V0-274, V1-1874).
func TestATRV0272_ApprovalGrantsNamedUpFront(t *testing.T) {
	t.Setenv("CORVINT_TASKS_ACTOR", "tester")
	root, _ := leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute).Truncate(time.Second))
	create := func(name, class string) string {
		t.Helper()
		payload := strings.NewReplacer(`"title":"Console ticket"`, `"title":"`+name+`"`, `"executionClass":"AUTONOMOUS"`, `"executionClass":"`+class+`"`, `"touchPaths":[]`, `"touchPaths":["src/`+name+`/"]`).Replace(createPayloadJSON)
		x := handoffCLI(t, root, "ticket", "create", "--request-id", "create-"+name, "--payload", payload)
		if x.res.Outcome != wire.OutcomeOK {
			t.Fatalf("create %s: %s", name, x.stdout)
		}
		return field(x.res.Items[0], "ticketId").Str
	}
	gated, plain := create("gated", "APPROVAL_REQUIRED"), create("plain", "AUTONOMOUS")
	missing := func(id string) []string {
		t.Helper()
		var verbs [2][]string
		for i, verb := range []string{"blockers", "show"} {
			x := handoffCLI(t, root, "ticket", verb, id)
			v := field(x.res.Items[0], "missingGrants")
			if v.Kind != wire.KindArray {
				t.Fatalf("ticket %s %s: missingGrants is not an array: %s", verb, id, x.stdout)
			}
			verbs[i] = []string{}
			for _, e := range v.Arr {
				verbs[i] = append(verbs[i], e.Str)
			}
		}
		if !slices.Equal(verbs[0], verbs[1]) {
			t.Fatalf("ticket blockers %v and ticket show %v disagree", verbs[0], verbs[1])
		}
		return verbs[0]
	}
	if got := missing(gated); !slices.Equal(got, []string{"RUN", "COMPLETE"}) {
		t.Fatalf("before any claim, missingGrants = %v, want [RUN COMPLETE]", got)
	}
	if got := missing(plain); len(got) != 0 {
		t.Fatalf("AUTONOMOUS ticket missingGrants = %v, want []", got)
	}
	refused := handoffCLI(t, root, "claim", gated, "--holder", "holder", "--request-id", "claim-ungranted")
	if !hasCode(refused.res, wire.CodeApprovalMissing) || !strings.Contains(string(refused.stdout), "grants missing through completion: RUN,COMPLETE") {
		t.Fatalf("claim refusal does not name every missing grant: %s", refused.stdout)
	}
	grant := `{"actor":"tester","grantId":"g-run","operation":"RUN","scope":[],"targetRevision":"1"}`
	if g := handoffCLI(t, root, "ticket", "grant-approval", "--request-id", "grant-run", "--target", gated, "--expected-revision", "1", "--payload", grant); g.res.Outcome != wire.OutcomeOK {
		t.Fatalf("grant RUN: %s", g.stdout)
	}
	if got := missing(gated); !slices.Equal(got, []string{"COMPLETE"}) {
		t.Fatalf("after the RUN grant, missingGrants = %v, want [COMPLETE]", got)
	}
	claimed := handoffCLI(t, root, "claim", gated, "--holder", "holder", "--request-id", "claim-granted")
	if claimed.res.Outcome != wire.OutcomeOK || !warningsContain(claimed.res.Warnings, "holds no COMPLETE grant at acceptanceRevision 1") {
		t.Fatalf("claim without a COMPLETE grant does not warn: %s", claimed.stdout)
	}
	if other := handoffCLI(t, root, "claim", plain, "--holder", "holder", "--request-id", "claim-plain"); other.res.Outcome != wire.OutcomeOK || warningsContain(other.res.Warnings, "grant at acceptanceRevision") {
		t.Fatalf("AUTONOMOUS claim: %s", other.stdout)
	}
}
