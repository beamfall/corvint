package cli

import (
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/dispatch"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// CAL-V0-102: dispatch status lists, sorted by ticket, each LOOP_DETECTED
// hold of the dispatcher's last native observation with its signal and
// counted generations; a ledger with no hold keeps the previous status shape.
func TestCALV0102_DispatchStatusShowsLoopDetected(t *testing.T) {
	t.Run("CAL-V0-102 DispatchStatusShowsLoopDetected", func(t *testing.T) {
		now := time.Now()
		l := &dispatch.Ledger{Program: "prog", Backoff: map[string]*dispatch.BackoffState{}}
		if _, ok := statusField(t, dispatchStatusValue(&dispatch.Config{}, t.TempDir(), l, nil, now), "loopDetected"); ok {
			t.Fatal("a ledger with no observation shows loop holds")
		}
		l.Seen = &dispatch.Seen{Tickets: map[string]string{}, Loops: map[string]dispatch.LoopHold{
			"ticket:a:q:t2": {Signal: "ALTERNATING_RETURNS", AcceptanceRevision: "2", Generations: []string{"4", "5", "6", "7", "8", "9"}},
			"ticket:a:q:t1": {Signal: "NO_PROGRESS", AcceptanceRevision: "0", Generations: []string{"1", "2", "3"}},
		}}
		held, ok := statusField(t, dispatchStatusValue(&dispatch.Config{}, t.TempDir(), l, nil, now), "loopDetected")
		if want := `[{"acceptanceRevision":"0","generations":["1","2","3"],"signal":"NO_PROGRESS","ticket":"ticket:a:q:t1"},{"acceptanceRevision":"2","generations":["4","5","6","7","8","9"],"signal":"ALTERNATING_RETURNS","ticket":"ticket:a:q:t2"}]`; !ok || string(wire.Encode(held)) != want {
			t.Fatalf("loopDetected = %s", wire.Encode(held))
		}
		l.Seen.Loops = nil
		if _, ok := statusField(t, dispatchStatusValue(&dispatch.Config{}, t.TempDir(), l, nil, now), "loopDetected"); ok {
			t.Fatal("a cleared hold still shows")
		}
	})
}
