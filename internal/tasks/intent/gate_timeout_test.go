package intent_test

import (
	"strconv"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestCALV0255_GateTimeoutBoundIs14400 decodes a policy whose gate declares
// the bound and one second over it, and keeps the bound inside one lease.
func TestCALV0255_GateTimeoutBoundIs14400(t *testing.T) {
	if wire.MaxGateTimeoutSecs != 14400 {
		t.Fatalf("gate timeout bound %d, want 14400", wire.MaxGateTimeoutSecs)
	}
	if wire.MaxGateTimeoutSecs > transaction.MaxLeaseMinutes*60 {
		t.Fatalf("gate timeout bound %d exceeds the longest lease %d s", wire.MaxGateTimeoutSecs, transaction.MaxLeaseMinutes*60)
	}
	for _, tc := range []struct {
		seconds int
		code    string
	}{{1, ""}, {7200, ""}, {14400, ""}, {14401, wire.CodeLimitExceeded}, {0, wire.CodeLimitExceeded}} {
		v := fixture.PolicyValue()
		gates, _ := v.Obj.Get("gates")
		gates.Arr[0].Obj.Set("timeoutSeconds", wire.String(strconv.Itoa(tc.seconds)))
		p, err := intent.DecodePolicy(wire.EncodeFile(v))
		if code(err) != tc.code {
			t.Fatalf("timeoutSeconds %d: got %q (%v), want %q", tc.seconds, code(err), err, tc.code)
		}
		if err == nil && p.Gates[0].TimeoutSeconds.Int() != int64(tc.seconds) {
			t.Fatalf("timeoutSeconds %d decoded as %s", tc.seconds, p.Gates[0].TimeoutSeconds)
		}
	}
}
