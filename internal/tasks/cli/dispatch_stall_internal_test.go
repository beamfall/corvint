package cli

import (
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/dispatch"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestCALV0185_DispatchStatusShowsStallCounts: dispatch status lists the
// tickets with a finished session since their native status last changed,
// most sessions first, with the configured threshold and whether each count
// reached it; a ledger without counts keeps the previous status shape.
func TestCALV0185_DispatchStatusShowsStallCounts(t *testing.T) {
	t.Run("CAL-V0-185", func(t *testing.T) {
		now := time.Now()
		l := &dispatch.Ledger{Program: "prog", Stall: map[string]*dispatch.StallState{"ticket:a:q:t0": {Status: "OPEN"}}}
		if _, ok := statusField(t, dispatchStatusValue(&dispatch.Config{}, t.TempDir(), l, nil, now), "sessionsSinceStatusChange"); ok {
			t.Fatal("a ledger with only zero counts shows stall counts")
		}
		l.Stall["ticket:a:q:t1"] = &dispatch.StallState{Status: "OPEN", Sessions: 2}
		l.Stall["ticket:a:q:t2"] = &dispatch.StallState{Status: "HELD", Sessions: 5}
		v, ok := statusField(t, dispatchStatusValue(&dispatch.Config{}, t.TempDir(), l, nil, now), "sessionsSinceStatusChange")
		if want := `{"omitted":"0","threshold":null,"tickets":[{"sessions":"5","stalled":false,"status":"HELD","ticket":"ticket:a:q:t2"},{"sessions":"2","stalled":false,"status":"OPEN","ticket":"ticket:a:q:t1"}]}`; !ok || string(wire.Encode(v)) != want {
			t.Fatalf("sessionsSinceStatusChange = %s", wire.Encode(v))
		}
		n := 3
		v, _ = statusField(t, dispatchStatusValue(&dispatch.Config{StalledAfterSessions: &n}, t.TempDir(), l, nil, now), "sessionsSinceStatusChange")
		if want := `{"omitted":"0","threshold":"3","tickets":[{"sessions":"5","stalled":true,"status":"HELD","ticket":"ticket:a:q:t2"},{"sessions":"2","stalled":false,"status":"OPEN","ticket":"ticket:a:q:t1"}]}`; string(wire.Encode(v)) != want {
			t.Fatalf("with threshold = %s", wire.Encode(v))
		}
	})
}
