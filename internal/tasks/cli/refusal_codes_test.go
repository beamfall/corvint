package cli_test

import (
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestTCLV0870_StaleUpdateCarriesStaleTicket (TCL-V0-870, V1-3552): a ticket
// refine and a release update at a stale expected revision each refuse with
// STALE_TICKET in the top-level codes, and the item still names the store's
// REVISION_CONFLICT outcome.
func TestTCLV0870_StaleUpdateCarriesStaleTicket(t *testing.T) {
	r, id := attachStore(t)
	refine := atm(t, r.Root, nil, "ticket", "refine", "--request-id", "stale-refine", "--target", id,
		"--expected-revision", "9", "--payload", `{"body":"A refine at a stale revision"}`)
	if refine.res.Outcome != wire.OutcomeRefused || !hasCode(refine.res, wire.CodeStaleTicket) ||
		field(refine.res.Items[0], "outcome").Str != mutation.OutcomeRevisionConflict {
		t.Fatalf("stale refine: %s", refine.stdout)
	}
	created := atm(t, r.Root, nil, "release", "create", "--request-id", "release-one", "--target", "v0-9",
		"--issued-at", "2026-09-20T12:01:00Z", "--payload", releaseCreatePayload("v0-9", "Version 0.9", id, ""))
	if created.res.Outcome != wire.OutcomeOK {
		t.Fatalf("release create: %s", created.stdout)
	}
	stale := atm(t, r.Root, nil, "release", "update", "--request-id", "stale-release-update", "--target", "v0-9",
		"--expected-revision", "9", "--issued-at", "2026-09-20T12:01:30Z", "--payload", releaseCreatePayload("v0-9", "Changed", id, ""))
	if stale.res.Outcome != wire.OutcomeRefused || len(stale.res.Codes) == 0 {
		t.Fatalf("stale release update: %s", stale.stdout)
	}
}
