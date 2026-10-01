package transaction

import (
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
	"testing"
)

func TestCALV0049_RetryObservationMatchesAdmission(t *testing.T) {
	rec := fixture.Ticket("AT-01")
	a := &snapshot.Attempt{AttemptID: "a", TicketID: rec.TicketID, TicketRevision: rec.AcceptanceRevision, Generation: "1", Phase: "CANCELLED", RetryCount: "3"}
	attempts := map[string]*snapshot.Attempt{"a": a}
	for _, tc := range []struct {
		name               string
		limit              int64
		prepare            func()
		charged, remaining string
		exhausted          bool
	}{
		{"legacy", 3, func() {}, "3", "0", true},
		{"lowered policy", 0, func() {}, "3", "0", true},
		{"handoff at limit", 3, func() {
			a.RuntimeID = snapshot.RuntimeExternalAgent
			a.RetryAccounting = &snapshot.RetryAccounting{Disposition: wire.CodeHandoff}
		}, "3", "0", false},
		{"new acceptance", 3, func() { a.TicketRevision = "99" }, "0", "3", false},
		{"initial zero", 0, func() { delete(attempts, "a") }, "0", "0", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.prepare()
			v := RetryObservation(attempts, rec, tc.limit)
			charged, _ := v.Obj.Get("charged")
			remaining, _ := v.Obj.Get("remaining")
			exhausted, _ := v.Obj.Get("exhausted")
			if charged.Str != tc.charged || remaining.Str != tc.remaining || exhausted.Bool != tc.exhausted || exhausted.Bool != retryExhausted(attempts, rec, tc.limit) {
				t.Fatalf("observation: %s", wire.Encode(v))
			}
		})
	}
}
func TestCALV0049_ChargeReasonPartition(t *testing.T) {
	expired := snapshot.CauseLeaseExpired
	for _, tc := range []struct {
		phase string
		cause *string
		want  string
	}{{"FAILED", &expired, "EXPIRED"}, {"FAILED", nil, "FAILED"}, {"CANCELLED", nil, "RELEASED"}, {"RUNNING", nil, "UNKNOWN"}} {
		if got := chargedReason(&snapshot.Attempt{Phase: tc.phase, Cause: tc.cause}); got != tc.want {
			t.Fatalf("%s: %s", tc.phase, got)
		}
	}
	a := &snapshot.Attempt{RetryCount: "2"}
	if retryReasons(a)["UNKNOWN"] != "2" {
		t.Fatal("legacy debt invented reasons")
	}
}
