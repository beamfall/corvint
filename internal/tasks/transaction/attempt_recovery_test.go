package transaction

import (
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestATRV0280_RecoveryNamesReleaseForEveryHolderWithoutReap pins V1-1964 as
// narrowed: a holder that nothing reaps, a STALE_HOLDER with a live lease, a
// lease-less attempt, or a supervised attempt with a null leaseExpiresAt,
// gets a refusal detail naming the exact release command, and a stale
// heartbeat is reported as advisory rather than as release authority
// (CAL-V0-121, ATR-V0-280, ATR-V0-281).
func TestATRV0280_RecoveryNamesReleaseForEveryHolderWithoutReap(t *testing.T) {
	now := wire.Timestamp("2026-10-21T12:00:00Z")
	old := wire.Timestamp("2026-10-21T11:00:00Z")
	en := snapshot.ReservationEntry{AttemptID: "att-1", Generation: wire.Size("3"), TicketID: wire.TicketID{Raw: "ticket:acme:main:AT-0001"}}
	release := "`corvint-tasks release --attempt att-1 --generation 3 --request-id NEW_ID`"
	for _, c := range []struct {
		name, status string
		a            snapshot.Attempt
		want         []string
	}{
		{"stale holder, live lease", "STALE_HOLDER",
			snapshot.Attempt{AttemptID: "att-1", Phase: "CHECKING", LastHeartbeatAt: &old, Lease: &snapshot.Lease{Holder: "h", ExpiresAt: "2026-10-21T13:00:00Z"}},
			[]string{"heartbeat is stale, which is advisory and frees nothing", "the lease is live until 2026-10-21T13:00:00Z", release}},
		{"stale supervised, null lease", "STALE_HOLDER",
			snapshot.Attempt{AttemptID: "att-1", Phase: "CHECKING", LastHeartbeatAt: &old, Supervision: &snapshot.Supervision{ProgramID: "prog-1"}},
			[]string{"heartbeat is stale", "supervised by program prog-1, whose liveness replaces a work lease, so nothing reaps it", release, "leaseExpiresAt null"}},
		{"no lease", "FRESH_HOLDER",
			snapshot.Attempt{AttemptID: "att-1", Phase: "RUNNING", LastHeartbeatAt: &now},
			[]string{"no work lease to reap", release}},
	} {
		s := SummarizeAttempt(en, &c.a, now, 600)
		if s.HolderStatus != c.status {
			t.Fatalf("%s: holderStatus %s, want %s", c.name, s.HolderStatus, c.status)
		}
		d := s.Detail()
		for _, w := range c.want {
			if !strings.Contains(d, w) {
				t.Errorf("%s: detail lacks %q: %s", c.name, w, d)
			}
		}
		if strings.Contains(d, "reap it") {
			t.Errorf("%s: detail offers a reap: %s", c.name, d)
		}
	}
}
