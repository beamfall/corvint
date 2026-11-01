package cli

import (
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/dispatch"
)

// CAL-V0-270, CAL-V0-273, CAL-V0-275 and CAL-V0-276: status reports the
// reader's health with its thresholds, timeout and stale window, NONE for
// an absent hold, an unrecorded error and a command-less reader's timeout,
// the hold once it applies, stale only inside the window, and omits
// workState when no reader is configured or recorded.
func TestCALV0270_DispatchStatusWorkState(t *testing.T) {
	now := time.Date(2026, 10, 16, 0, 0, 0, 0, time.UTC)
	c := &dispatch.Config{TickSeconds: 30, WorkState: &dispatch.WorkState{Kind: "command", Argv: []string{"/usr/bin/true"}, EscalateAfterTicks: 2, HoldLaunchesAfterTicks: 4}}
	l := &dispatch.Ledger{Program: "prog", WorkState: &dispatch.WorkStateHealth{OK: true, Since: now}}
	ws := pressureField(t, dispatchStatusValue(c, "/s", l, nil, now), "workState")
	for key, want := range map[string]string{"health": "OK", "since": "2026-10-16T00:00:00Z", "lastError": "NONE", "failedTicks": "0", "escalateAfterTicks": "2", "holdLaunchesAfterTicks": "4", "timeoutSeconds": "60", "staleAfterSeconds": "30"} {
		if got := pressureField(t, ws, key).Str; got != want {
			t.Errorf("%s = %q want %q", key, got, want)
		}
	}
	if !pressureField(t, ws, "ok").Bool || pressureField(t, ws, "launchesHeld").Bool || pressureField(t, ws, "stale").Bool {
		t.Fatal("healthy reader reported failing or held")
	}
	l.WorkState = &dispatch.WorkStateHealth{Since: now, LastError: "work state command: exit status 127", FailedTicks: 4, Escalated: true}
	ws = pressureField(t, dispatchStatusValue(c, "/s", l, nil, now), "workState")
	if pressureField(t, ws, "health").Str != "FAILING" || pressureField(t, ws, "failedTicks").Str != "4" || !pressureField(t, ws, "escalated").Bool || !pressureField(t, ws, "launchesHeld").Bool || pressureField(t, ws, "lastError").Str != "work state command: exit status 127" {
		t.Fatalf("failing = %+v", ws)
	}
	if !pressureField(t, ws, "stale").Bool {
		t.Fatal("a failure inside the window is not stale")
	}
	if ws = pressureField(t, dispatchStatusValue(c, "/s", l, nil, now.Add(30*time.Second)), "workState"); pressureField(t, ws, "stale").Bool {
		t.Fatal("a failure past the window is still stale")
	}
	c.WorkState.TimeoutSeconds, c.WorkState.StaleAfterSeconds = 5, 90
	if ws = pressureField(t, dispatchStatusValue(c, "/s", l, nil, now.Add(30*time.Second)), "workState"); pressureField(t, ws, "timeoutSeconds").Str != "5" || pressureField(t, ws, "staleAfterSeconds").Str != "90" || !pressureField(t, ws, "stale").Bool {
		t.Fatalf("configured timeout and window = %+v", ws)
	}
	c.WorkState.Kind = "status-line"
	if ws = pressureField(t, dispatchStatusValue(c, "/s", l, nil, now), "workState"); pressureField(t, ws, "timeoutSeconds").Str != "NONE" {
		t.Fatalf("status-line timeout = %+v", ws)
	}
	c.WorkState.Kind = "command"
	c.WorkState.HoldLaunchesAfterTicks = 0
	if ws = pressureField(t, dispatchStatusValue(c, "/s", l, nil, now), "workState"); pressureField(t, ws, "launchesHeld").Bool || pressureField(t, ws, "holdLaunchesAfterTicks").Str != "NONE" {
		t.Fatalf("hold off = %+v", ws)
	}
	if _, ok := dispatchStatusValue(&dispatch.Config{}, "/s", &dispatch.Ledger{Program: "prog"}, nil, now).Obj.Get("workState"); ok {
		t.Fatal("status reported workState without a reader")
	}
}
