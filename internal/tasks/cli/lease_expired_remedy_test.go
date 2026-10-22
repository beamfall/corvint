package cli_test

import (
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestATRV0271_GateRunAfterExpiryNamesRemedy: gate run after the lease
// expired during an idle gap is refused FENCED and names the remedy; renew
// cannot revive the lease (CAL-V0-010), and the claim the remedy names reaps
// it and starts the next generation (CAL-V0-011) (ATR-V0-271, V1-0828).
func TestATRV0271_GateRunAfterExpiryNamesRemedy(t *testing.T) {
	root, claims := expiredCLIStore(t, 1)
	c := claims[0]
	remedy := []string{"the lease expired at ", "renew cannot revive it", "claim the ticket again, which reaps the expired lease", "renew or heartbeat before the lease expires"}
	gate := handoffCLI(t, root, "gate", "run", "--attempt", c.AttemptID, "--generation", string(c.Generation), "--gate", "verify", "--request-id", "gate-after-gap", "--worktree", root)
	if !hasCode(gate.res, wire.CodeFenced) {
		t.Fatalf("gate run after expiry: want FENCED: %s", gate.stdout)
	}
	for _, want := range remedy {
		if !strings.Contains(string(gate.stdout), want) {
			t.Fatalf("gate run refusal lacks %q: %s", want, gate.stdout)
		}
	}
	renew := handoffCLI(t, root, "renew", "--attempt", c.AttemptID, "--generation", string(c.Generation), "--lease-minutes", "30", "--request-id", "renew-after-gap")
	if !hasCode(renew.res, wire.CodeFenced) || !strings.Contains(string(renew.stdout), "renew cannot revive it") {
		t.Fatalf("renew after expiry: want FENCED with the remedy: %s", renew.stdout)
	}
	again := handoffCLI(t, root, "claim", c.Ticket, "--holder", "holder", "--scope", "src/a", "--request-id", "claim-after-gap")
	if again.res.Outcome != wire.OutcomeOK {
		t.Fatalf("the named remedy, claim again, failed: %s", again.stdout)
	}
	if gen := field(again.res.Items[0], "generation").Str; gen == "" || (field(again.res.Items[0], "attemptId").Str == c.AttemptID && gen == string(c.Generation)) {
		t.Fatalf("re-claim did not start a new generation: %s", again.stdout)
	}
}
