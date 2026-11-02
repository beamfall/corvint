package cli_test

import (
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestTCDV0680_ReceiptAuditReportsCompletionHealth: `receipt audit` adds
// completionsByDay and completionHealth (TCD-V0-680, V1-3716); a manual
// close with no verified one raises the NO_VERIFIED warning, and the
// existing completions member is unchanged.
func TestTCDV0680_ReceiptAuditReportsCompletionHealth(t *testing.T) {
	r := receiptFixture(t)
	audit := func() run {
		t.Helper()
		x := atm(t, r.Root, nil, "receipt", "audit")
		if x.res.Outcome != wire.OutcomeOK || len(x.res.Items) != 1 {
			t.Fatalf("audit: %s", x.stdout)
		}
		return x
	}
	warned := func(x run) bool {
		for _, w := range x.res.Warnings {
			if strings.Contains(w, "TCD-V0-680") {
				return true
			}
		}
		return false
	}

	x := audit()
	h := field(x.res.Items[0], "completionHealth")
	if field(h, "state").Str != "NO_COMPLETIONS" || len(field(x.res.Items[0], "completionsByDay").Arr) != 0 || warned(x) {
		t.Fatalf("no completions: %s", x.stdout)
	}

	id, _ := effectsTicket(t, r.Root, "closed", "P2", `{"coverage":"QUALIFIED","externalUnbounded":false,"resources":[],"touchPaths":[]}`)
	if c := atm(t, r.Root, nil, "ticket", "complete-manual", "--request-id", "health-close", "--target", id, "--expected-revision", obligationRevision(t, r.Root, id),
		"--issued-at", "2026-10-27T12:00:00Z", "--payload", `{"evidence":[],"reason":"manual"}`); c.res.Outcome != wire.OutcomeOK {
		t.Fatalf("complete-manual: %s", c.stdout)
	}
	x = audit()
	item := x.res.Items[0]
	h, days := field(item, "completionHealth"), field(item, "completionsByDay")
	if field(h, "state").Str != "NO_VERIFIED" || field(h, "manual").Str != "1" || field(h, "verified").Str != "0" || field(h, "windowDays").Str != "7" || !warned(x) {
		t.Fatalf("manual close: %s", x.stdout)
	}
	if len(days.Arr) != 1 || field(days.Arr[0], "day").Str != field(h, "windowEnd").Str || field(days.Arr[0], "manual").Str != "1" {
		t.Fatalf("completionsByDay: %s", x.stdout)
	}
	if c := field(item, "completions"); field(c, "manual").Str != "1" || field(c, "verified").Str != "0" {
		t.Fatalf("completions changed: %s", x.stdout)
	}
}
