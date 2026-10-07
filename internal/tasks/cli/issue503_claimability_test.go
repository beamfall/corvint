package cli_test

import (
	"fmt"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/wire"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestIssue503_ZeroRemainingHandoffAdmission(t *testing.T) {
	t.Run("CAL-V0-049 zero remaining explicit admission", func(t *testing.T) {
		root, _ := leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
		id := planTicket(t, root, "charged-read", "P1", `["src/"]`)
		runOK := func(args ...string) run {
			t.Helper()
			x := handoffCLI(t, root, args...)
			if x.code != 0 {
				t.Fatalf("%v: %s", args, x.stdout)
			}
			return x
		}
		for i := 0; i <= 3; i++ {
			x := runOK("claim", id, "--holder", "qualification", "--stage", "integrate", "--request-id", fmt.Sprintf("claim-debt-%d", i))
			a, g := field(x.res.Items[0], "attemptId").Str, field(x.res.Items[0], "generation").Str
			show := runOK("ticket", "show", id)
			debt := field(show.res.Items[0], "retries")
			if field(debt, "charged").Str != fmt.Sprint(i) || field(field(debt, "byReason"), "RELEASED").Str != fmt.Sprint(i) || field(debt, "reasonHistory").Str != "COMPLETE" {
				t.Fatalf("prospective reasons: %s", show.stdout)
			}
			args := []string{"release", "--attempt", a, "--generation", g, "--request-id", fmt.Sprintf("release-debt-%d", i)}
			if i == 3 {
				args = append(args, "--reason", "HANDOFF", "--evidence", "local:review-result")
			}
			runOK(args...)
		}
		for _, args := range [][]string{{"ticket", "show", id}, {"plan", "preview"}, {"queue", "status", "--retries"}} {
			x := runOK(args...)
			v := x.res.Items[0]
			if args[0] == "plan" {
				v = field(v, "entries").Arr[0]
			}
			if args[0] == "queue" {
				v = field(v, "retries").Arr[0]
			}
			debt := field(v, "retries")
			if field(debt, "charged").Str != "3" || field(debt, "remaining").Str != "0" || field(debt, "exhausted").Bool {
				t.Fatalf("clean handoff debt: %s", x.stdout)
			}
		}
		shown := runOK("ticket", "show", id)
		item := shown.res.Items[0]
		if field(item, "claimable").Kind != wire.KindBool || !field(item, "claimable").Bool || field(item, "claimabilityScope").Str != "RECORDED_DEFAULT_EXTERNAL_AGENT_PLAN" || field(field(item, "retries"), "retryAdmissionReason").Str != "VERIFIED_HANDOFF" {
			t.Fatalf("ambiguous admission: %s", shown.stdout)
		}
		refusal := handoffCLI(t, root, "ticket", "reopen", "--target", id, "--expected-revision", "1", "--request-id", "unneeded-reopen", "--role", "OWNER", "--payload", `{"reason":"retry limit appears spent"}`)
		if refusal.code == 0 || !strings.Contains(string(refusal.stdout), "RETRY_BUDGET_NOT_EXHAUSTED") {
			t.Fatalf("reopen reason: %s", refusal.stdout)
		}

		x := runOK("claim", id, "--holder", "next-reviewer", "--stage", "review", "--request-id", "claim-clean-handoff")
		show := runOK("attempt", "show", field(x.res.Items[0], "attemptId").Str)
		if field(show.res.Items[0], "retryCount").Str != "3" || field(field(show.res.Items[0], "retryReasons"), "RELEASED").Str != "3" {
			t.Fatalf("handoff charged again: %s", show.stdout)
		}
	})
}

func TestIssue503_JournalAbsentAdmissionUnknown(t *testing.T) {
	t.Run("CAL-V0-049 absent journal preserves unknown", func(t *testing.T) {
		r := fixture.TempRepo(t)
		fixture.WriteIntent(t, r, fixture.Ticket("A"))
		before := fixture.TreeSnapshot(t, r.Root)
		shown := handoffCLI(t, r.Root, "ticket", "show", "A")
		if shown.code != 0 {
			t.Fatalf("show: %s", shown.stdout)
		}
		item := shown.res.Items[0]
		if field(item, "claimable").Kind != wire.KindNull || field(item, "claimabilityReason").Str != "NOT_OBSERVED" || field(field(item, "retries"), "retryAdmissionReason").Str != "NOT_OBSERVED" {
			t.Fatalf("invented admission: %s", shown.stdout)
		}
		after := fixture.TreeSnapshot(t, r.Root)
		if !reflect.DeepEqual(before, after) {
			t.Fatal("read mutated repository")
		}
	})
}
