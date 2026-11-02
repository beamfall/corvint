package cli_test

import (
	"strings"
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

// TestTCLV0859_UnknownVerbAndMissingTicketCarryCodes (TCL-V0-859, V1-3736):
// an unknown first word carries MALFORMED and a typo hint names the near
// verb; each read of a ticket absent from the queue refuses with MALFORMED.
func TestTCLV0859_UnknownVerbAndMissingTicketCarryCodes(t *testing.T) {
	r, _ := attachStore(t)
	typo := atm(t, r.Root, nil, "tikcet", "show")
	if typo.res.Outcome != wire.OutcomeError || !hasCode(typo.res, wire.CodeMalformed) ||
		!strings.Contains(strings.Join(typo.res.Warnings, ";"), "did you mean `corvint-tasks ticket --help`") {
		t.Fatalf("typo verb: %s", typo.stdout)
	}
	if far := atm(t, r.Root, nil, "no-such-verb"); !hasCode(far.res, wire.CodeMalformed) || strings.Contains(strings.Join(far.res.Warnings, ";"), "did you mean") {
		t.Fatalf("unrelated verb: %s", far.stdout)
	}
	for _, args := range [][]string{
		{"ticket", "show", "NOPE-404"},
		{"ticket", "note", "show", "NOPE-404"},
		{"ticket", "note", "history", "NOPE-404"},
		{"critical-path", "NOPE-404"},
		{"ticket", "escalation", "show", "NOPE-404", "q-1"},
		{"ticket", "escalation", "history", "NOPE-404", "q-1"},
	} {
		x := atm(t, r.Root, nil, args...)
		if x.res.Outcome != wire.OutcomeRefused || !hasCode(x.res, wire.CodeMalformed) {
			t.Errorf("%v: %s", args, x.stdout)
		}
	}
}
