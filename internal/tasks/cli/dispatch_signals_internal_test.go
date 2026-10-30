//go:build darwin || linux

package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/dispatch"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// DSG-V0-006: dispatch status shows the latest queue observation's
// unmatched work states and idle streak, and neither without one.
func TestDSGV0006_DispatchStatusSignals(t *testing.T) {
	now := time.Date(2026, 10, 25, 2, 0, 0, 0, time.UTC)
	dir := t.TempDir()
	l := &dispatch.Ledger{Program: "prog"}
	v := dispatchStatusValue(nil, dir, l, nil, now)
	for _, key := range []string{"unmatchedWorkStates", "idleWithCapacity"} {
		if _, ok := v.Obj.Get(key); ok {
			t.Fatalf("status has %q without an observation", key)
		}
	}
	since := now.Add(-2 * time.Hour)
	rec := dispatch.AdmissionRecord{Profile: dispatch.AdmissionProfile, Program: "prog", At: now, TickSeconds: 30, Pressure: dispatch.AdmissionPressureNotConfigured, GlobalCap: 4, Roles: []dispatch.RoleQueue{}, Held: []dispatch.HeldRunner{},
		UnmatchedStates: []dispatch.UnmatchedState{{State: "X~2", Tickets: 3, Example: "ticket:a:q:x"}}, IdleTicks: 240, IdleSince: &since, IdleDeferred: []dispatch.DeferralCount{{Reason: "NO_ROLE_MATCH", Tickets: 3}}}
	raw, _ := json.Marshal(rec)
	if err := os.WriteFile(filepath.Join(dir, dispatch.AdmissionFile), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	v = dispatchStatusValue(nil, dir, l, nil, now)
	if got := string(wire.Encode(pressureField(t, v, "unmatchedWorkStates"))); got != `[{"example":"ticket:a:q:x","state":"X~2","tickets":"3"}]` {
		t.Fatalf("unmatched: %s", got)
	}
	if got := string(wire.Encode(pressureField(t, v, "idleWithCapacity"))); got != `{"deferred":[{"reason":"NO_ROLE_MATCH","tickets":"3"}],"observedAt":"2026-10-25T02:00:00Z","since":"2026-10-25T00:00:00Z","ticks":"240"}` {
		t.Fatalf("idle: %s", got)
	}
}
