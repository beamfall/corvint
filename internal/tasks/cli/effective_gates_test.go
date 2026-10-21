package cli_test

import (
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestV12038_ShowAndBlockersReportPolicyRequiredGates: the fixture policy
// marks gate verify required and ticket A names no gate of its own, so its
// record's requiredGates is empty while complete demands verify. ticket show
// and ticket blockers report that effective set; the record is unchanged.
func TestV12038_ShowAndBlockersReportPolicyRequiredGates(t *testing.T) {
	r := fixture.TempRepo(t)
	fixture.WriteState(t, r)
	a := fixture.Ticket("A")
	fixture.WriteIntent(t, r, a)
	fixture.CommitPosts(t, r, "MUTATION", "", map[string][]byte{"intent/tickets/A.json": a.Encode()})
	for _, verb := range []string{"show", "blockers"} {
		x := atm(t, r.Root, nil, "ticket", verb, "A")
		if x.res.Outcome != wire.OutcomeOK || len(x.res.Items) != 1 {
			t.Fatalf("%s: %+v %v", verb, x.res, x.res.Warnings)
		}
		it := x.res.Items[0]
		if got := field(it, "requiredGates"); len(got.Arr) != 0 {
			t.Errorf("%s: record requiredGates changed: %s", verb, wire.Encode(got))
		}
		got := field(it, "effectiveRequiredGates")
		if len(got.Arr) != 1 || got.Arr[0].Str != "verify" {
			t.Errorf("%s: effectiveRequiredGates %s, want [verify]", verb, wire.Encode(got))
		}
	}
}
