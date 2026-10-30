package cli_test

import (
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func queueStatusMember(t *testing.T, root, member string) wire.Value {
	t.Helper()
	x := atm(t, root, nil, "queue", "status")
	if x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("queue status %s", x.stdout)
	}
	for _, p := range field(x.res.Items[0], "pools").Arr {
		for _, m := range field(p, "members").Arr {
			if field(m, "memberId").Str == member {
				return m
			}
		}
	}
	t.Fatalf("no member %s in %s", member, x.stdout)
	return wire.Value{}
}

// PSR-V0-268..272 end to end: an operator holds a FREE member, pool status
// and queue status show HELD with the window, a dispatched claim is handed
// another member, a replay reports the same allocation, renew moves the
// expiry, and release quarantines the member for the cleanup path.
func TestPSRV0268_OperatorHoldLifecycle(t *testing.T) {
	r := poolHealthRepo(t)
	hold := []string{"pool", "hold", "--pool", "db", "--member", "a", "--holder", "ops", "--lease-minutes", "60", "--reason", "hand-run schema migration", "--request-id", "hold-a"}
	x := atm(t, r.Root, nil, hold...)
	if x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("hold %s", x.stdout)
	}
	alloc := field(x.res.Items[0], "allocationId").Str
	if alloc == "" {
		t.Fatalf("hold reports no allocation %s", x.stdout)
	}
	if again := atm(t, r.Root, nil, hold...); again.res.Outcome != wire.OutcomeOK || field(again.res.Items[0], "allocationId").Str != alloc {
		t.Fatalf("replay %s", again.stdout)
	}
	a := poolStatusMembers(t, r.Root)["db/a"]
	h := field(a, "hold")
	if field(a, "state").Str != "HELD" || field(a, "holder").Str != "ops" || h.Kind != wire.KindObject || field(h, "expired").Bool || field(h, "reason").Str != "hand-run schema migration" || !recordedTime(field(h, "heldAt").Str) || !recordedTime(field(h, "expiresAt").Str) {
		t.Fatalf("pool status %s", wire.Encode(a))
	}
	if q := queueStatusMember(t, r.Root, "a"); field(q, "state").Str != "HELD" || field(field(q, "hold"), "expiresAt").Str != field(h, "expiresAt").Str {
		t.Fatalf("queue status %s", wire.Encode(q))
	}
	if b := queueStatusMember(t, r.Root, "b"); field(b, "hold").Kind != wire.KindNull && field(b, "hold").Kind != 0 {
		t.Fatalf("FREE member shows a hold %s", wire.Encode(b))
	}
	tk := planTicket(t, r.Root, "work", "P1", `["work"]`)
	c := atm(t, r.Root, nil, "claim", tk, "--holder", "builder", "--request-id", "claim-w", "--pool", "db", "--stage", "implement", "--exclude-member", "review")
	if c.res.Outcome != wire.OutcomeOK || field(field(c.res.Items[0], "poolAllocation"), "memberId").Str != "b" {
		t.Fatalf("claim was not routed around the hold %s", c.stdout)
	}
	if x := atm(t, r.Root, nil, "pool", "hold", "renew", "--member", "a", "--allocation", alloc, "--lease-minutes", "120", "--request-id", "renew-a"); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("renew %s", x.stdout)
	}
	if got := field(poolStatusMembers(t, r.Root)["db/a"], "hold"); field(got, "expiresAt").Str <= field(h, "expiresAt").Str {
		t.Fatalf("renew did not move the expiry %s", wire.Encode(got))
	}
	if x := atm(t, r.Root, nil, "pool", "hold", "release", "--member", "a", "--allocation", alloc, "--request-id", "release-a"); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("release %s", x.stdout)
	}
	a = poolStatusMembers(t, r.Root)["db/a"]
	if field(a, "state").Str != "QUARANTINED" || !strings.Contains(field(field(a, "quarantine"), "reason").Str, "physical safe reuse unproved") {
		t.Fatalf("released %s", wire.Encode(a))
	}
	if x := atm(t, r.Root, nil, "pool", "hold", "--pool", "db", "--member", "b", "--holder", "ops", "--lease-minutes", "60", "--reason", "job", "--request-id", "hold-b"); !hasCode(x.res, wire.CodeResourceCollision) {
		t.Fatalf("hold of an allocated member %s", x.stdout)
	}
	if x := atm(t, r.Root, nil, "pool", "hold", "--pool", "db", "--member", "review", "--holder", "ops", "--lease-minutes", "60", "--request-id", "hold-r"); !hasCode(x.res, wire.CodeMalformed) {
		t.Fatalf("hold without a reason %s", x.stdout)
	}
	if x := atm(t, r.Root, nil, "pool", "hold", "--help", "--verbose"); !strings.Contains(string(x.stdout), "1440") {
		t.Fatalf("help %s", x.stdout)
	}
}
