package cli

import (
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestTCDV0680_CompletionHealthWindow covers the window and the 31-day cap
// of `receipt audit` completion health (TCD-V0-680, V1-3716).
func TestTCDV0680_CompletionHealthWindow(t *testing.T) {
	day := func(s string, verified, manual int) completionDay {
		d, err := time.Parse("2006-01-02", s)
		if err != nil {
			t.Fatal(err)
		}
		return completionDay{day: d, verified: verified, manual: manual}
	}
	member := func(v wire.Value, key string) wire.Value {
		got, ok := v.Obj.Get(key)
		if !ok {
			t.Fatalf("no %s in %s", key, wire.Encode(v))
		}
		return got
	}

	// A verified close eight days before the newest day is outside the
	// seven-day window, so the window holds only manual closes.
	_, health, warning := completionHealth([]completionDay{day("2026-11-01", 0, 3), day("2026-10-26", 0, 2), day("2026-10-24", 4, 1)})
	if member(health, "state").Str != "NO_VERIFIED" || member(health, "manual").Str != "5" || member(health, "verified").Str != "0" ||
		member(health, "windowEnd").Str != "2026-11-01" || member(health, "windowDays").Str != "7" {
		t.Fatalf("health: %s", wire.Encode(health))
	}
	if !strings.Contains(warning, "5 manual and 0 verified completion(s) in the 7 days ending 2026-11-01") || !strings.Contains(warning, "TCD-V0-680") {
		t.Fatalf("warning: %q", warning)
	}

	// The seventh day back is inside the window.
	if _, health, warning = completionHealth([]completionDay{day("2026-11-01", 0, 3), day("2026-10-26", 1, 0)}); member(health, "state").Str != "OK" || warning != "" {
		t.Fatalf("seventh day: %s %q", wire.Encode(health), warning)
	}

	if byDay, health, warning := completionHealth(nil); member(health, "state").Str != "NO_COMPLETIONS" || member(health, "windowEnd").Kind != wire.KindNull ||
		len(byDay.Arr) != 0 || byDay.Kind != wire.KindArray || warning != "" {
		t.Fatalf("empty: %s %s %q", wire.Encode(byDay), wire.Encode(health), warning)
	}

	var many []completionDay
	start := day("2026-11-01", 1, 0).day
	for i := 0; i < 40; i++ {
		many = append(many, completionDay{day: start.AddDate(0, 0, -i), verified: 1})
	}
	byDay, _, _ := completionHealth(many)
	if len(byDay.Arr) != completionDaysShown || member(byDay.Arr[0], "day").Str != "2026-11-01" || member(byDay.Arr[30], "day").Str != "2026-10-02" {
		t.Fatalf("cap: %s", wire.Encode(byDay))
	}
}
