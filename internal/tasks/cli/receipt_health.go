package cli

import (
	"sort"
	"strconv"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TCD-V0-680 (V1-3716): `receipt audit` reports VERIFIED and MANUAL
// completions per UTC day of completion.recordedAt, and a health state
// over the window ending on the newest completion day, so a run of manual
// closes with no gated one is visible without reading each ticket. The
// window is anchored on the store, never on the clock, so the read stays
// deterministic for one snapshot.
const (
	completionDaysShown  = 31
	completionHealthDays = 7
)

type completionDay struct {
	day              time.Time
	verified, manual int
}

// completionDays folds the COMPLETED tickets' VERIFIED and MANUAL
// completions by day, newest first. A completion whose recordedAt carries no
// date is skipped; other completion kinds are not counted (TCD-V0-190).
func completionDays(inv *ticket.Inventory) []completionDay {
	byDay := map[time.Time]*completionDay{}
	for _, id := range inv.IDs() {
		rec, ok := inv.Get(id)
		if !ok || rec.Status != ticket.StatusCompleted || rec.Completion == nil {
			continue
		}
		kind, at := rec.Completion.Kind, string(rec.Completion.RecordedAt)
		if kind != "VERIFIED" && kind != "MANUAL" || len(at) < 10 {
			continue
		}
		day, err := time.Parse("2006-01-02", at[:10])
		if err != nil {
			continue
		}
		d := byDay[day]
		if d == nil {
			d = &completionDay{day: day}
			byDay[day] = d
		}
		if kind == "VERIFIED" {
			d.verified++
		} else {
			d.manual++
		}
	}
	days := make([]completionDay, 0, len(byDay))
	for _, d := range byDay {
		days = append(days, *d)
	}
	sort.Slice(days, func(i, j int) bool { return days[i].day.After(days[j].day) })
	return days
}

// completionHealth returns the `completionsByDay` and `completionHealth`
// members, and a warning when the window holds manual completions and no
// verified one. state is NO_COMPLETIONS with no counted completion,
// NO_VERIFIED in that warning case, and OK otherwise.
func completionHealth(days []completionDay) (byDay, health wire.Value, warning string) {
	rows := make([]wire.Value, 0, completionDaysShown)
	for i, d := range days {
		if i == completionDaysShown {
			break
		}
		rows = append(rows, wire.ObjectValue(wire.NewObject().Set("day", wire.String(d.day.Format("2006-01-02"))).
			Set("verified", count(d.verified)).Set("manual", count(d.manual))))
	}
	h := wire.NewObject().Set("windowDays", count(completionHealthDays))
	if len(days) == 0 {
		h.Set("windowEnd", wire.Null()).Set("verified", count(0)).Set("manual", count(0)).Set("state", wire.String("NO_COMPLETIONS"))
		return wire.Array(rows...), wire.ObjectValue(h), ""
	}
	end := days[0].day
	from := end.AddDate(0, 0, 1-completionHealthDays)
	verified, manual := 0, 0
	for _, d := range days {
		if d.day.Before(from) {
			break
		}
		verified, manual = verified+d.verified, manual+d.manual
	}
	state := "OK"
	if verified == 0 && manual > 0 {
		state = "NO_VERIFIED"
		warning = prose("completion health: " + strconv.Itoa(manual) + " manual and 0 verified completion(s) in the " +
			strconv.Itoa(completionHealthDays) + " days ending " + end.Format("2006-01-02") +
			"; a gated close (submit, gate run, complete) records a verified one (TCD-V0-680)")
	}
	h.Set("windowEnd", wire.String(end.Format("2006-01-02"))).Set("verified", count(verified)).Set("manual", count(manual)).Set("state", wire.String(state))
	return wire.Array(rows...), wire.ObjectValue(h), warning
}
