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

func TestIssue503_RetryExplanation(t *testing.T) {
	t.Run("CAL-V0-049 retry reason exemptions", func(t *testing.T) {
		rec := fixture.Ticket("AT-01")
		for _, tc := range []struct {
			name, phase, revision string
			handoff               bool
			want                  string
		}{
			{"initial", "", "", false, "INITIAL_ADMISSION"},
			{"completed", "COMPLETED", "1", false, "COMPLETED_ATTEMPT"},
			{"new acceptance", "CANCELLED", "2", false, "NEW_ACCEPTANCE"},
			{"handoff", "CANCELLED", "1", true, "VERIFIED_HANDOFF"},
			{"exhausted", "CANCELLED", "1", false, "RETRY_EXHAUSTED"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				attempts := map[string]*snapshot.Attempt{}
				if tc.phase != "" {
					a := &snapshot.Attempt{TicketID: rec.TicketID, TicketRevision: wire.Count(tc.revision), Generation: "1", Phase: tc.phase, RetryCount: "3", RuntimeID: snapshot.RuntimeExternalAgent}
					if tc.handoff {
						a.RetryAccounting = &snapshot.RetryAccounting{Disposition: wire.CodeHandoff}
					}
					attempts["a"] = a
				}
				v := RetryObservation(attempts, rec, 0)
				reason, _ := v.Obj.Get("retryAdmissionReason")
				meaning, _ := v.Obj.Get("remainingMeaning")
				if reason.Str != tc.want || meaning.Str != "RETRY_CAPACITY" {
					t.Fatalf("%s", wire.Encode(v))
				}
			})
		}
	})
}
