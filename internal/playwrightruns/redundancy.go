package playwrightruns

import (
	"encoding/json"
	"math"
	"sort"
	"time"
)

// Redundancy profile and vocabulary (PWR-V0-037).
const (
	RedundancyProfile = "corvint-runs-redundancy/0"

	CategoryUndeclared = "UNDECLARED"

	RunCounted         = "COUNTED"
	RunHistory         = "HISTORY"
	RunExcluded        = "EXCLUDED"
	ReasonStartMissing = "START_TIME_MISSING"
	ReasonBeforeSince  = "BEFORE_SINCE"
	ReasonShardRefused = "SHARD_SET_REFUSED"
)

// RedundancyTally is test time by how it relates to earlier qualified passes. A test re-run at a
// fingerprint that already passed strictly earlier is an exact re-run; one re-run at the same
// Playwright version, test-file blob, config digest and project as an earlier pass while an
// application or test revision differs is a same-file-content re-run. Unfingerprinted time is test
// time whose fingerprint is incomplete, so it can be neither. Minutes are rounded to 0.01 and shares
// (of total time) to 0.0001.
type RedundancyTally struct {
	Runs                        int     `json:"runs"`
	Tests                       int     `json:"tests"`
	TotalMS                     int64   `json:"totalMs"`
	TotalMinutes                float64 `json:"totalMinutes"`
	ExactRerunTests             int     `json:"exactRerunTests"`
	ExactRerunMS                int64   `json:"exactRerunMs"`
	ExactRerunMinutes           float64 `json:"exactRerunMinutes"`
	ExactRerunShare             float64 `json:"exactRerunShare"`
	SameFileContentRerunTests   int     `json:"sameFileContentRerunTests"`
	SameFileContentRerunMS      int64   `json:"sameFileContentRerunMs"`
	SameFileContentRerunMinutes float64 `json:"sameFileContentRerunMinutes"`
	SameFileContentRerunShare   float64 `json:"sameFileContentRerunShare"`
	UnfingerprintedTests        int     `json:"unfingerprintedTests"`
	UnfingerprintedMS           int64   `json:"unfingerprintedMs"`
}

// CategoryTally is one run category's tally.
type CategoryTally struct {
	Category string `json:"category"`
	RedundancyTally
}

// RedundancyRun says how one report took part: COUNTED in the window, HISTORY (before --since, its
// passes still count as earlier passes) or EXCLUDED with a reason.
type RedundancyRun struct {
	Run       int    `json:"run"`
	Sha256    string `json:"sha256"`
	Category  string `json:"category"`
	StartTime string `json:"startTime"`
	Role      string `json:"role"`
	Reason    string `json:"reason"`
	// IdentitySource is present only when the caller gave a sidecar option (PWR-V0-067).
	IdentitySource string `json:"identitySource,omitempty"`
}

// RedundancyReport is the `corvint runs redundancy` document.
type RedundancyReport struct {
	Profile    string          `json:"profile"`
	Form       string          `json:"form"`
	Since      string          `json:"since"`
	Runs       []RedundancyRun `json:"runs,omitempty"`
	Categories []CategoryTally `json:"categories,omitempty"`
	Total      RedundancyTally `json:"total"`
}

// Compact is the summary form: the total without per-run and per-category rows.
func (r RedundancyReport) Compact() RedundancyReport {
	r.Form, r.Runs, r.Categories = FormSummary, nil, nil
	return r
}

type timedRun struct {
	logicalRun
	start time.Time
}

// Redundancy tallies test time across runs by category (PWR-V0-037). Logical runs (an unsharded
// report, or an admitted shard set) are ordered by stats.startTime; a run without one, or a refused
// shard set, is excluded. A run before since is history: never counted, but its qualified passes
// are earlier passes for the runs after it. Only a strictly earlier qualified pass makes a re-run.
func Redundancy(runs []*Run, since *time.Time) RedundancyReport {
	report := RedundancyReport{Profile: RedundancyProfile, Form: FormFull, Runs: []RedundancyRun{}, Categories: []CategoryTally{}}
	if since != nil {
		report.Since = since.UTC().Format(time.RFC3339Nano)
	}
	positions := make([]int, len(runs))
	for i := range runs {
		positions[i] = i + 1
	}
	logical, sets := logicalRuns(runs, positions)
	for i, run := range runs {
		row := RedundancyRun{Run: i + 1, Sha256: run.Sha256, Category: screenText(category(run)), IdentitySource: run.IdentitySource}
		if run.HasStartTime {
			row.StartTime = run.StartTime.Format(time.RFC3339Nano)
		}
		report.Runs = append(report.Runs, row)
	}
	role := map[int][2]string{}
	timed := []timedRun{}
	for _, run := range logical {
		switch {
		case run.refusal != "":
			role[run.index] = [2]string{RunExcluded, ReasonShardRefused}
		case !run.HasStartTime:
			role[run.index] = [2]string{RunExcluded, ReasonStartMissing}
		default:
			role[run.index] = [2]string{RunCounted, ""}
			if since != nil && run.StartTime.Before(*since) {
				role[run.index] = [2]string{RunHistory, ReasonBeforeSince}
			}
			timed = append(timed, timedRun{logicalRun: run, start: run.StartTime})
		}
	}
	// Every report of a shard set takes its logical run's role.
	for _, set := range sets {
		for _, pos := range set.Reports {
			role[pos] = role[set.Run]
		}
	}
	for i := range report.Runs {
		r := role[report.Runs[i].Run]
		report.Runs[i].Role, report.Runs[i].Reason = r[0], r[1]
	}
	// Earliest qualified pass per test and exact fingerprint, and per test and file content.
	exact, content := map[string]time.Time{}, map[string]time.Time{}
	note := func(index map[string]time.Time, key string, at time.Time) {
		if prev, ok := index[key]; !ok || at.Before(prev) {
			index[key] = at
		}
	}
	for _, run := range timed {
		for _, t := range run.Tests {
			o := observe(run.logicalRun, t)
			if o.complete && o.passed {
				note(exact, exactKey(t, o.fp), run.start)
				note(content, contentKey(t, o.fp), run.start)
			}
		}
	}
	byCategory := map[string]*RedundancyTally{}
	for _, run := range timed {
		if role[run.index][0] != RunCounted {
			continue
		}
		name := category(run.Run)
		tally := byCategory[name]
		if tally == nil {
			tally = &RedundancyTally{}
			byCategory[name] = tally
		}
		tally.Runs++
		report.Total.Runs++
		for _, t := range run.Tests {
			if !t.Executed {
				continue
			}
			o := observe(run.logicalRun, t)
			for _, tl := range []*RedundancyTally{tally, &report.Total} {
				count(tl, o, t, run.start, exact, content)
			}
		}
	}
	names := make([]string, 0, len(byCategory))
	for name := range byCategory {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		report.Categories = append(report.Categories, CategoryTally{Category: screenText(name), RedundancyTally: finishTally(*byCategory[name])})
	}
	report.Total = finishTally(report.Total)
	return report
}

func category(run *Run) string {
	if run.Category == "" {
		return CategoryUndeclared
	}
	return run.Category
}

func exactKey(t Test, fp Fingerprint) string {
	raw, _ := json.Marshal([]any{t.Key(), fp})
	return string(raw)
}

func contentKey(t Test, fp Fingerprint) string {
	raw, _ := json.Marshal([]any{t.Key(), fp.PlaywrightVersion, fp.TestFileBlob, fp.ConfigDigest, fp.Project})
	return string(raw)
}

func count(tl *RedundancyTally, o priorObservation, t Test, start time.Time, exact, content map[string]time.Time) {
	tl.Tests++
	tl.TotalMS += o.durationM
	if !o.complete {
		tl.UnfingerprintedTests++
		tl.UnfingerprintedMS += o.durationM
		return
	}
	if at, ok := exact[exactKey(t, o.fp)]; ok && at.Before(start) {
		tl.ExactRerunTests++
		tl.ExactRerunMS += o.durationM
		return
	}
	if at, ok := content[contentKey(t, o.fp)]; ok && at.Before(start) {
		tl.SameFileContentRerunTests++
		tl.SameFileContentRerunMS += o.durationM
	}
}

func finishTally(tl RedundancyTally) RedundancyTally {
	minutes := func(ms int64) float64 { return math.Round(float64(ms)/600) / 100 }
	share := func(ms int64) float64 {
		if tl.TotalMS == 0 {
			return 0
		}
		return math.Round(float64(ms)/float64(tl.TotalMS)*10000) / 10000
	}
	tl.TotalMinutes = minutes(tl.TotalMS)
	tl.ExactRerunMinutes, tl.ExactRerunShare = minutes(tl.ExactRerunMS), share(tl.ExactRerunMS)
	tl.SameFileContentRerunMinutes, tl.SameFileContentRerunShare = minutes(tl.SameFileContentRerunMS), share(tl.SameFileContentRerunMS)
	return tl
}
