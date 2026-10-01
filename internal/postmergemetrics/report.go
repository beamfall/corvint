package postmergemetrics

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"time"
)

func digest(v any) string {
	b, _ := json.Marshal(v)
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

// Build never treats caller-asserted event completeness or a recommendation as authority.
func Build(ctx context.Context, p Policy, h History, from, until string, m Measurer) (Report, error) {
	var report Report
	if err := validatePolicy(p); err != nil {
		return report, err
	}
	if err := validateHistory(p, h); err != nil {
		return report, err
	}
	start, e := timestamp(from)
	end, f := timestamp(until)
	if e != nil || f != nil || !start.Before(end) {
		return report, ErrWindow
	}
	if ctx.Err() != nil {
		return report, ErrCancelled
	}
	// Own the canonical copies: JSON field and array order must not change report bytes.
	pb, _ := json.Marshal(p)
	hb, _ := json.Marshal(h)
	_ = json.Unmarshal(pb, &p)
	_ = json.Unmarshal(hb, &h)
	sort.Slice(p.Classes, func(i, j int) bool { return p.Classes[i].ID < p.Classes[j].ID })
	sort.Slice(h.Classes, func(i, j int) bool { return h.Classes[i].Class < h.Classes[j].Class })
	for i := range h.Classes {
		t, _ := timestamp(h.Classes[i].RunsThrough)
		h.Classes[i].RunsThrough = t.Format(time.RFC3339Nano)
		t, _ = timestamp(h.Classes[i].EventsThrough)
		h.Classes[i].EventsThrough = t.Format(time.RFC3339Nano)
	}
	for i := range h.Runs {
		t, _ := timestamp(h.Runs[i].At)
		h.Runs[i].At = t.Format(time.RFC3339Nano)
		sort.Slice(h.Runs[i].Stages, func(a, b int) bool { return h.Runs[i].Stages[a].Name < h.Runs[i].Stages[b].Name })
	}
	for i := range h.Events {
		t, _ := timestamp(h.Events[i].At)
		h.Events[i].At = t.Format(time.RFC3339Nano)
	}
	sort.Slice(h.Runs, func(i, j int) bool { return timeIDLess(h.Runs[i].At, h.Runs[i].ID, h.Runs[j].At, h.Runs[j].ID) })
	sort.Slice(h.Events, func(i, j int) bool { return timeIDLess(h.Events[i].At, h.Events[i].ID, h.Events[j].At, h.Events[j].ID) })
	report = Report{Profile: "postmerge-report/0", Authority: "none", Provenance: "caller-asserted-run-and-event-inventories; net-edit-proxy-not-authenticated-authorship", From: start.Format(time.RFC3339Nano), Until: end.Format(time.RFC3339Nano), PolicySHA256: digest(p), HistorySHA256: digest(h), Classes: []ClassResult{}, Runs: []RunResult{}}
	byClass := map[string]int{}
	runByID := map[string]Run{}
	for _, r := range h.Runs {
		runByID[r.ID] = r
	}
	for _, c := range p.Classes {
		cr := ClassResult{Class: c.ID, Policy: c, Reasons: []string{}}
		for _, inv := range h.Classes {
			if inv.Class == c.ID {
				copy := inv
				cr.Inventory = &copy
				cr.HistoryComplete = complete(h, inv, runByID, end)
				break
			}
		}
		byClass[c.ID] = len(report.Classes)
		report.Classes = append(report.Classes, cr)
	}
	type point struct {
		at    time.Time
		id    string
		run   *Run
		event *Event
	}
	timeline := make([]point, 0, len(h.Runs)+len(h.Events))
	for i := range h.Runs {
		r := &h.Runs[i]
		at, _ := timestamp(r.At)
		if at.Before(end) {
			timeline = append(timeline, point{at: at, id: r.ID, run: r})
		}
	}
	for i := range h.Events {
		ev := &h.Events[i]
		at, _ := timestamp(ev.At)
		if at.Before(end) {
			timeline = append(timeline, point{at: at, id: ev.ID, event: ev})
		}
	}
	sort.Slice(timeline, func(i, j int) bool {
		a, b := timeline[i], timeline[j]
		if !a.at.Equal(b.at) {
			return a.at.Before(b.at)
		}
		if (a.event != nil) != (b.event != nil) {
			return a.event != nil
		}
		return a.id < b.id
	})
	corrected, reverted, demoted := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, point := range timeline {
		if ctx.Err() != nil {
			return Report{}, ErrCancelled
		}
		if ev := point.event; ev != nil {
			cr := &report.Classes[byClass[runByID[ev.RunID].Class]]
			if ev.Kind == "revert" {
				cr.RemainingDemotion = cr.Policy.DemoteRuns
				reverted[ev.RunID] = true
				demoted[ev.RunID] = true
			}
			if ev.Kind == "revert" || ev.Kind == "correction" {
				corrected[ev.RunID] = true
			}
			if !point.at.Before(start) {
				switch ev.Kind {
				case "revert":
					cr.RevertEvents++
				case "correction":
					cr.CorrectionEvents++
				case "invalid-finding":
					cr.InvalidFindingEvents++
				case "containment":
					cr.ContainmentEvents++
				}
			}
		} else {
			r := point.run
			cr := &report.Classes[byClass[r.Class]]
			if cr.RemainingDemotion > 0 {
				demoted[r.ID] = true
				cr.RemainingDemotion--
			}
		}
	}
	cache := map[GitPair]Measurement{}
	for _, r := range h.Runs {
		at, _ := timestamp(r.At)
		if at.Before(start) || !at.Before(end) {
			continue
		}
		if ctx.Err() != nil {
			return Report{}, ErrCancelled
		}
		cr := &report.Classes[byClass[r.Class]]
		cr.Total++
		rr := RunResult{Run: r, Corrected: corrected[r.ID], Reverted: reverted[r.ID], Demoted: demoted[r.ID]}
		if rr.Corrected {
			cr.Corrected++
		}
		if rr.Reverted {
			cr.Reverted++
		}
		if rr.Demoted {
			cr.Demoted++
		}
		switch r.Outcome {
		case "generated":
			cr.Generated++
		case "no-change":
			cr.NoChange++
		case "failed":
			cr.Failed++
		case "unknown":
			cr.Unknown++
		}
		if len(r.Stages) == 0 {
			reason(cr, "stage-evidence-incomplete")
		}
		for _, s := range r.Stages {
			if s.Outcome == "failed" || s.Outcome == "unknown" {
				reason(cr, "stage-evidence-incomplete")
			}
		}
		if r.Followup.Status == "unknown" {
			reason(cr, "followup-unknown")
		}
		if r.Outcome == "generated" {
			if r.Git == nil {
				cr.UnknownEditRuns++
			} else {
				got, ok := cache[*r.Git]
				if !ok {
					if m == nil {
						return Report{}, ErrGit
					}
					var err error
					got, err = m.Measure(ctx, *r.Git)
					if err != nil {
						return Report{}, err
					}
					if !validMeasurement(*r.Git, got) {
						return Report{}, ErrGit
					}
					cache[*r.Git] = got
				}
				copy := got
				rr.Measurement = &copy
				cr.EditedFiles += got.Files
				cr.TextAdded += got.TextAdded
				cr.TextDeleted += got.TextDeleted
				if got.Added == nil || got.Deleted == nil {
					cr.UnknownEditRuns++
				}
			}
		}
		report.Runs = append(report.Runs, rr)
	}
	for i := range report.Classes {
		cr := &report.Classes[i]
		if !cr.HistoryComplete {
			reason(cr, "history-incomplete")
		}
		if cr.Generated < int64(cr.Policy.MinSamples) {
			reason(cr, "insufficient-samples")
		}
		if cr.Corrected*10000 > cr.Policy.MaxCorrectionBP*cr.Generated {
			reason(cr, "correction-threshold")
		}
		if cr.Failed > 0 || cr.Unknown > 0 {
			reason(cr, "run-outcome-incomplete")
		}
		if cr.UnknownEditRuns > 0 {
			reason(cr, "edit-evidence-incomplete")
		}
		if cr.Reverted > 0 {
			reason(cr, "reverted-cohort")
		}
		if cr.InvalidFindingEvents > 0 || cr.ContainmentEvents > 0 {
			reason(cr, "adverse-event")
		}
		cr.Recommendation = "eligible-for-owner-consideration"
		if len(cr.Reasons) > 0 {
			cr.Recommendation = "review"
		}
		if cr.RemainingDemotion > 0 {
			reason(cr, "revert-cooldown")
			cr.Recommendation = "demoted"
		}
		sort.Strings(cr.Reasons)
	}
	return report, nil
}
func reason(c *ClassResult, s string) {
	for _, old := range c.Reasons {
		if old == s {
			return
		}
	}
	c.Reasons = append(c.Reasons, s)
}
func complete(h History, inv Inventory, runs map[string]Run, end time.Time) bool {
	rt, _ := timestamp(inv.RunsThrough)
	et, _ := timestamp(inv.EventsThrough)
	if !inv.RunsComplete || !inv.EventsComplete || rt.Before(end) || et.Before(end) {
		return false
	}
	seq := make(map[int]bool)
	count := 0
	// Watermarks must cover every supplied row, including future rows, as well as the cutoff.
	for _, r := range h.Runs {
		if r.Class != inv.Class {
			continue
		}
		at, _ := timestamp(r.At)
		if at.After(rt) {
			return false
		}
		seq[r.Sequence] = true
		count++
	}
	if count != inv.LastSequence {
		return false
	}
	for i := 1; i <= inv.LastSequence; i++ {
		if !seq[i] {
			return false
		}
	}
	for _, ev := range h.Events {
		if runs[ev.RunID].Class == inv.Class {
			at, _ := timestamp(ev.At)
			if at.After(et) {
				return false
			}
		}
	}
	return true
}
func validMeasurement(g GitPair, m Measurement) bool {
	if m.Bot != g.Bot || m.Approved != g.Approved || !oid.MatchString(m.BotTree) || !oid.MatchString(m.ApprovedTree) || len(m.BotTree) != len(g.Bot) || len(m.ApprovedTree) != len(g.Bot) || m.GitVersion == "" || m.Convention != "no-renames-delete-plus-add" || m.Files < 0 || m.Files > 10000 || m.BinaryFiles < 0 || m.BinaryFiles > m.Files || m.GitlinkFiles < 0 || m.GitlinkFiles > m.Files-m.BinaryFiles || m.TextAdded < 0 || m.TextDeleted < 0 || m.TextAdded > 1e12 || m.TextDeleted > 1e12 {
		return false
	}
	if m.BinaryFiles > 0 || m.GitlinkFiles > 0 {
		return m.Added == nil && m.Deleted == nil
	}
	return m.Added != nil && m.Deleted != nil && *m.Added == m.TextAdded && *m.Deleted == m.TextDeleted
}
