//go:build darwin || linux

package cli

import (
	"os"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/dispatch"
)

// ATR-V0-065: dispatch status shows the last recorded suspend gap and the
// number kept; an absent record adds nothing and an unreadable one is
// UNKNOWN.
func TestATRV0065_DispatchStatusShowsLastHostSleep(t *testing.T) {
	dir := t.TempDir()
	if v := dispatchHostSleepValue(dir); v != nil {
		t.Fatalf("absent record shown: %+v", v)
	}
	path := dispatch.HostSleepPath(dir)
	rec := `{"profile":"taskman-dispatch-host-sleep/0","program":"prog","gaps":[{"from":"2026-10-17T01:00:00Z","to":"2026-10-17T02:00:00Z","seconds":3600},{"from":"2026-10-17T09:00:00Z","to":"2026-10-17T12:00:00Z","seconds":10800}]}`
	if err := os.WriteFile(path, []byte(rec), 0o600); err != nil {
		t.Fatal(err)
	}
	v := dispatchHostSleepValue(dir)
	if v == nil {
		t.Fatal("record not shown")
	}
	if n, _ := v.Obj.Get("gaps"); n.Str != "2" {
		t.Fatalf("gaps = %q", n.Str)
	}
	last, _ := v.Obj.Get("lastGap")
	for k, want := range map[string]string{"from": "2026-10-17T09:00:00Z", "to": "2026-10-17T12:00:00Z", "seconds": "10800"} {
		if got, _ := last.Obj.Get(k); got.Str != want {
			t.Fatalf("lastGap %s = %q, want %q", k, got.Str, want)
		}
	}
	if err := os.WriteFile(path, []byte(`{"profile":"taskman-dispatch-host-sleep/0"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if v := dispatchHostSleepValue(dir); v == nil {
		t.Fatal("unreadable record hidden")
	} else if s, _ := v.Obj.Get("state"); s.Str != dispatch.StateUnknown {
		t.Fatalf("unreadable record state %q", s.Str)
	}
}
