//go:build darwin || linux

package cli

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/dispatch"
)

// ATR-V0-061: dispatch status shows each ticket's short-session streak and
// whether it holds launches; an absent record adds nothing and an
// unreadable one is UNKNOWN.
func TestATRV0061_DispatchStatusShowsShortSessionHold(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)
	if v := dispatchSessionChurnValue(dir, now); v != nil {
		t.Fatalf("absent record shown: %+v", v)
	}
	path := dispatch.SessionChurnPath(dir)
	rec := `{"profile":"taskman-dispatch-session-churn/0","program":"prog","tickets":{"ticket:a:q:t1":{"durations":[10,40,20],"streak":3,"firstSeen":"2026-10-17T11:00:00Z","lastEnd":"2026-10-17T11:58:00Z","holdUntil":"2026-10-17T12:08:00Z"}}}`
	if err := os.WriteFile(path, []byte(rec), 0o600); err != nil {
		t.Fatal(err)
	}
	v := dispatchSessionChurnValue(dir, now)
	if v == nil || len(v.Arr) != 1 {
		t.Fatalf("streak rows %+v", v)
	}
	row := v.Arr[0].Obj
	for k, want := range map[string]string{"key": "ticket:a:q:t1", "sessions": "3", "medianSeconds": "20", "firstSeen": "2026-10-17T11:00:00Z", "holdUntil": "2026-10-17T12:08:00Z", "reason": "SHORT_SESSIONS"} {
		if got, _ := row.Get(k); got.Str != want {
			t.Fatalf("%s = %q, want %q", k, got.Str, want)
		}
	}
	if held, _ := row.Get("held"); !held.Bool {
		t.Fatal("an active hold is not shown as held")
	}
	if v := dispatchSessionChurnValue(dir, now.Add(time.Hour)); v == nil {
		t.Fatal("an ended hold dropped the streak")
	} else if held, _ := v.Arr[0].Obj.Get("held"); held.Bool {
		t.Fatal("an ended hold is shown as held")
	}
	if err := os.WriteFile(filepath.Join(dir, "session-churn.json"), []byte(`{`), 0o600); err != nil {
		t.Fatal(err)
	}
	if v := dispatchSessionChurnValue(dir, now); v == nil || v.Obj == nil {
		t.Fatal("an unreadable record is not UNKNOWN")
	} else if s, _ := v.Obj.Get("state"); s.Str != dispatch.StateUnknown {
		t.Fatalf("state %q", s.Str)
	}
}
