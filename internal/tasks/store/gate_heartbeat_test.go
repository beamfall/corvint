package store_test

import (
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
)

// TestLGLV0810_GateRunHeartbeatsItsHolder: a gate program that outlives the
// heartbeat TTL measured from the claim leaves its attempt holder fresh,
// because gate run heartbeats while the program runs (V1-3941); a gate that
// finishes before the first beat writes none, and its holder reads stale.
func TestLGLV0810_GateRunHeartbeatsItsHolder(t *testing.T) {
	defer store.SetGateBeatIntervalForTest(40 * time.Millisecond)()
	ttl := int64(intent.DefaultHeartbeatTTLSeconds)
	for gate, want := range map[string]string{"long": "FRESH_HOLDER", "quick": "STALE_HOLDER"} {
		s := newLeaseStore(t, commandGate("long", "/bin/sleep 1", "30", false), commandGate("quick", "printf ok", "30", false))
		claim, _ := s.submitted(t, s.ticket(t, gate), "src-"+gate, 0)
		report, err := s.gate(t, "gate-"+gate, gateOf(claim, gate), 11)
		if err != nil || report.Outcome.Outcome != mutation.OutcomeCompleted {
			t.Fatalf("%s: %+v %v", gate, report, err)
		}
		a := s.attempt(t, claim.AttemptID)
		now, err := time.Parse("2006-01-02T15:04:05Z", string(s.at(t, 11)))
		if err != nil {
			t.Fatal(err)
		}
		if got := transaction.HolderStatus(a, now, ttl); got != want {
			t.Fatalf("%s: holder %s, want %s (last heartbeat %v)", gate, got, want, a.LastHeartbeatAt)
		}
	}
}
