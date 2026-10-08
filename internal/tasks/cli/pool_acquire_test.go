package cli_test

import (
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// CAL-V0-198, CAL-V0-201, CAL-V0-204: pool acquire and pool release on a live
// attempt claimed without a pool report their allocation keys, replay the
// receipt-bound result, and a refused acquire still names poolAllocation.
func TestCALV0204_CLIAcquireAndReleaseResults(t *testing.T) {
	r := shareRepo(t)
	id := planTicket(t, r.Root, "one", "P1", `["one"]`)
	claim := atm(t, r.Root, nil, "claim", id, "--holder", "builder", "--request-id", "claim-1", "--stage", "implement")
	if claim.res.Outcome != wire.OutcomeOK || field(claim.res.Items[0], "poolAllocation").Kind != wire.KindNull {
		t.Fatalf("claim %s", claim.stdout)
	}
	attempt, generation := field(claim.res.Items[0], "attemptId").Str, field(claim.res.Items[0], "generation").Str
	acquire := []string{"pool", "acquire", "--attempt", attempt, "--generation", generation, "--pool", "db", "--request-id", "acquire-1", "--exclude-member", "a"}
	x := atm(t, r.Root, nil, acquire...)
	allocation := field(x.res.Items[0], "poolAllocation")
	if x.res.Outcome != wire.OutcomeOK || field(allocation, "memberId").Str != "b" || field(x.res.Items[0], "ticketId").Str != id {
		t.Fatalf("acquire %s", x.stdout)
	}
	if again := atm(t, r.Root, nil, acquire...); again.res.Outcome != wire.OutcomeOK || !field(again.res.Items[0], "replayed").Bool || string(wire.Encode(field(again.res.Items[0], "poolAllocation"))) != string(wire.Encode(allocation)) {
		t.Fatalf("acquire replay %s", again.stdout)
	}
	refused := atm(t, r.Root, nil, "pool", "acquire", "--attempt", attempt, "--generation", generation, "--pool", "db", "--request-id", "acquire-2")
	if refused.res.Outcome == wire.OutcomeOK || !hasCode(refused.res, wire.CodeResourceCollision) || field(refused.res.Items[0], "poolAllocation").Kind != wire.KindNull {
		t.Fatalf("second acquire %s", refused.stdout)
	}
	release := []string{"pool", "release", "--attempt", attempt, "--generation", generation, "--allocation", field(allocation, "allocationId").Str, "--request-id", "release-1"}
	x = atm(t, r.Root, nil, release...)
	returned := field(x.res.Items[0], "releasedPoolAllocation")
	if x.res.Outcome != wire.OutcomeOK || string(wire.Encode(field(returned, "allocation"))) != string(wire.Encode(allocation)) {
		t.Fatalf("release %s", x.stdout)
	}
	if again := atm(t, r.Root, nil, release...); !field(again.res.Items[0], "replayed").Bool || string(wire.Encode(field(again.res.Items[0], "releasedPoolAllocation"))) != string(wire.Encode(returned)) {
		t.Fatalf("release replay %s", again.stdout)
	}
	if m := poolStatusMembers(t, r.Root, "--member", "b")["db/b"]; field(m, "state").Str != "QUARANTINED" || field(m, "attemptId").Str != attempt {
		t.Fatalf("pool status %s", wire.Encode(m))
	}
	show := atm(t, r.Root, nil, "attempt", "show", attempt)
	if show.res.Outcome != wire.OutcomeOK || field(show.res.Items[0], "phase").Str != "RUNNING" {
		t.Fatalf("attempt show %s", show.stdout)
	}
	for name, want := range map[string]string{"acquire": "--pool ID --request-id ID", "release": "--allocation SHA256 --request-id ID"} {
		help := atm(t, r.Root, nil, "pool", name, "--help")
		if help.res.Outcome != wire.OutcomeOK || !strings.Contains(field(help.res.Items[0], "usage").Str, want) {
			t.Fatalf("%s help %s", name, help.stdout)
		}
	}
	for _, bad := range [][]string{
		{"pool", "acquire", "--attempt", attempt, "--generation", generation, "--request-id", "no-pool"},
		{"pool", "acquire", "--attempt", attempt, "--generation", generation, "--pool", "db", "--holder", "x", "--request-id", "holder"},
		{"pool", "release", "--attempt", attempt, "--generation", generation, "--request-id", "no-allocation"},
	} {
		if x := atm(t, r.Root, nil, bad...); x.res.Outcome == wire.OutcomeOK {
			t.Fatalf("malformed %v accepted: %s", bad, x.stdout)
		}
	}
}
