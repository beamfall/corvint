package cli_test

import (
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestATRV0190_CompleteManualRefusalNamesExpiredAttempt: an attempt whose
// lease expired hours ago still blocks complete-manual (decision: membership
// in the reservation set is the liveness contract, and reap is the one-command
// recovery), but the refusal names the attempt, its holder, holder status and
// the reap command; ticket show carries the same summary (ATR-V0-191); after
// that reap the same complete-manual commits.
func TestATRV0190_CompleteManualRefusalNamesExpiredAttempt(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	root, claims := leaseCLIStore(t, 1, now.Add(-3*time.Hour))
	c := claims[0]
	complete := func(req string) run {
		t.Helper()
		return handoffCLI(t, root, "ticket", "complete-manual", "--request-id", req, "--target", c.Ticket, "--expected-revision", "1", "--payload", `{"evidence":[],"reason":"manual"}`)
	}
	refused := complete("manual-1")
	if !hasCode(refused.res, wire.CodeAttemptLive) {
		t.Fatalf("expired attempt did not block complete-manual: %s", refused.stdout)
	}
	for _, want := range []string{"attempt " + c.AttemptID, "ticket " + c.Ticket, "generation " + string(c.Generation), "holder holder", "holderStatus LEASE_EXPIRED", "corvint-tasks reap --attempt " + c.AttemptID + " --generation " + string(c.Generation) + " --lease-expires-at "} {
		if !strings.Contains(string(refused.stdout), want) {
			t.Fatalf("refusal lacks %q: %s", want, refused.stdout)
		}
	}
	show := handoffCLI(t, root, "ticket", "show", c.Ticket)
	cur := field(show.res.Items[0], "liveAttempt")
	if cur.Kind != wire.KindObject || field(cur, "attemptId").Str != c.AttemptID || field(cur, "holder").Str != "holder" || field(cur, "holderStatus").Str != "LEASE_EXPIRED" || !strings.Contains(field(cur, "recovery").Str, "corvint-tasks reap") {
		t.Fatalf("ticket show liveAttempt: %s", show.stdout)
	}
	expires := field(cur, "leaseExpiresAt").Str
	if reap := handoffCLI(t, root, "reap", "--attempt", c.AttemptID, "--generation", string(c.Generation), "--lease-expires-at", expires, "--request-id", "reap-1"); reap.code != 0 {
		t.Fatalf("reap: %s", reap.stdout)
	}
	if after := handoffCLI(t, root, "ticket", "show", c.Ticket); field(after.res.Items[0], "liveAttempt").Kind != wire.KindNull {
		t.Fatalf("reaped attempt still live: %s", after.stdout)
	}
	if done := complete("manual-2"); done.res.Outcome != wire.OutcomeOK || field(done.res.Items[0], "outcome").Str != "COMPLETED" {
		t.Fatalf("complete-manual after reap: %s", done.stdout)
	}
}

// TestATRV0192_CollisionNamesHoldingAttempt: a RESOURCE_COLLISION claim
// refusal names the holding attempt, its ticket, holder status and the
// colliding keys, so an operator does not read reservations.json.
func TestATRV0192_CollisionNamesHoldingAttempt(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	// Claims land at base+10m and base+11m with five-minute leases: both live.
	root, claims := leaseCLIStore(t, 2, now.Add(-11*time.Minute))
	a, b := claims[0], claims[1]
	if rel := handoffCLI(t, root, "release", "--attempt", b.AttemptID, "--generation", string(b.Generation), "--request-id", "release-b"); rel.code != 0 {
		t.Fatalf("release: %s", rel.stdout)
	}
	x := handoffCLI(t, root, "claim", b.Ticket, "--holder", "other", "--scope", "src/a", "--request-id", "collide")
	if !hasCode(x.res, wire.CodeResourceCollision) {
		t.Fatalf("want RESOURCE_COLLISION: %s", x.stdout)
	}
	for _, want := range []string{"scope collides with live attempt " + a.AttemptID, "ticket " + a.Ticket, "holderStatus FRESH_HOLDER", "colliding keys: PATH:src/a", "recovery: the lease is live until "} {
		if !strings.Contains(string(x.stdout), want) {
			t.Fatalf("collision refusal lacks %q: %s", want, x.stdout)
		}
	}
}
